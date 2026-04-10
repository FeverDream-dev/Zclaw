package connections

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// WebhookConfig describes how to deliver events to an external HTTP endpoint.
type WebhookConfig struct {
	URL         string
	Method      string
	Headers     map[string]string
	Secret      string
	Events      []string
	RetryPolicy RetryPolicy
}

// RetryPolicy controls how a webhook sender retries failed deliveries.
type RetryPolicy struct {
	MaxRetries  int
	BackoffBase time.Duration
	BackoffMax  time.Duration
}

// WebhookSender is a helper that sends events to a configured webhook endpoint.
type WebhookSender struct {
	client *http.Client
	config WebhookConfig
}

// WithSignature returns the X-Signature for a given payload using the configured secret.
func (s *WebhookSender) WithSignature(payload []byte) string {
	mac := hmac.New(sha256.New, []byte(s.config.Secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// Send delivers an event payload to the webhook endpoint. It signs the body if a secret
// is configured and respects the configured retry policy.
func (s *WebhookSender) Send(ctx context.Context, event string, payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if s.config.Secret != "" {
		// Attach signature to headers for verification on the receiver side
		sig := s.WithSignature(body)
		if s.config.Headers == nil {
			s.config.Headers = make(map[string]string)
		}
		s.config.Headers["X-Signature"] = sig
	}
	req, err := http.NewRequest(stringsToUpper(s.config.Method, http.MethodPost), s.config.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range s.config.Headers {
		req.Header.Set(k, v)
	}
	// Simple retry loop
	var resp *http.Response
	for attempt := 0; attempt <= s.config.RetryPolicy.MaxRetries; attempt++ {
		resp, err = s.client.Do(req.WithContext(ctx))
		if err == nil && resp != nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if resp.Body != nil {
				ioutil.ReadAll(resp.Body)
				resp.Body.Close()
			}
			return nil
		}
		// wait before retry
		wait := s.config.RetryPolicy.BackoffBase * (1 << uint(attempt))
		if wait > s.config.RetryPolicy.BackoffMax {
			wait = s.config.RetryPolicy.BackoffMax
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
	if resp != nil && resp.Body != nil {
		ioutil.ReadAll(resp.Body)
		resp.Body.Close()
	}
	return errFallback(err)
}

// errFallback is a tiny helper to satisfy the return type when retries exhausted.
func errFallback(err error) error {
	if err == nil {
		return fmt.Errorf("webhook: delivery failed after retries")
	}
	return err
}

// WebhookReceiver is a lightweight HTTP receiver for webhook events.
type WebhookReceiver struct {
	mu       sync.RWMutex
	handlers map[string]func(WebhookEvent)
	secret   string
	server   *http.Server
	port     int
}

// WebhookEvent represents the payload delivered from a webhook source.
type WebhookEvent struct {
	Source    string          `json:"source"`
	EventType string          `json:"event_type"`
	Timestamp int64           `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

// NewWebhookReceiver creates a new receiver with an optional shared secret for signing.
func NewWebhookReceiver(secret string) *WebhookReceiver {
	return &WebhookReceiver{handlers: make(map[string]func(WebhookEvent)), secret: secret}
}

// RegisterHandler registers a handler for a given path.
func (r *WebhookReceiver) RegisterHandler(path string, handler func(WebhookEvent)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[path] = handler
}

// Start launches an HTTP server to receive webhook events.
func (r *WebhookReceiver) Start(port int) error {
	r.port = port
	mux := http.NewServeMux()
	for path, _ := range r.handlers {
		p := path
		mux.HandleFunc(p, func(w http.ResponseWriter, req *http.Request) {
			if req.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			body, _ := ioutil.ReadAll(req.Body)
			// Basic signature verification if a secret is configured
			if r.secret != "" {
				sig := req.Header.Get("X-Signature")
				if !verifySignature(body, r.secret, sig) {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
			}
			var ev WebhookEvent
			if err := json.Unmarshal(body, &ev); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			r.mu.RLock()
			handler := r.handlers[p]
			r.mu.RUnlock()
			if handler != nil {
				handler(ev)
			}
			w.WriteHeader(http.StatusOK)
		})
	}
	srv := &http.Server{Addr: ":" + strconv.Itoa(port), Handler: mux}
	r.server = srv
	go func() {
		_ = srv.ListenAndServe()
	}()
	return nil
}

// Stop stops the webhook receiver server if running.
func (r *WebhookReceiver) Stop() error {
	if r.server != nil {
		_ = r.server.Close()
	}
	return nil
}

func verifySignature(payload []byte, secret, signature string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

// stringsToUpper is a tiny helper to default to POST when empty.
func stringsToUpper(input string, defaultVal string) string {
	if input == "" {
		return defaultVal
	}
	return input
}
