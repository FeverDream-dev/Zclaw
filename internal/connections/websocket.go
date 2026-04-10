package connections

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"sync"
	"time"
)

// WebSocketBridge is a lightweight, polling-based bridge that mimics a WebSocket
// interface without pulling in external dependencies. It provides a send/receive
// model suitable for local agent communication where true WebSockets are not
// strictly required.
type WebSocketBridge struct {
	endpoint   string
	client     *http.Client
	in         chan []byte
	out        chan []byte
	done       chan struct{}
	onMessage  func([]byte)
	mu         sync.RWMutex
	connected  bool
	backoff    time.Duration
	maxBackoff time.Duration
}

// NewWebSocketBridge creates a new polling-based WebSocketBridge.
func NewWebSocketBridge() *WebSocketBridge {
	return &WebSocketBridge{
		client:     &http.Client{Timeout: 5 * time.Second},
		in:         make(chan []byte, 16),
		out:        make(chan []byte, 16),
		done:       make(chan struct{}),
		backoff:    500 * time.Millisecond,
		maxBackoff: 10 * time.Second,
	}
}

// Connect establishes a base endpoint to communicate with. This implementation uses
// polling against the provided URL as a fallback to true WebSocket connections.
func (b *WebSocketBridge) Connect(url string) error {
	b.mu.Lock()
	b.endpoint = url
	b.connected = true
	b.mu.Unlock()
	go b.pollLoop()
	return nil
}

// Send queues a message to be delivered to the remote endpoint via HTTP POST.
func (b *WebSocketBridge) Send(message []byte) error {
	b.mu.RLock()
	if !b.connected {
		b.mu.RUnlock()
		return nil
	}
	b.mu.RUnlock()
	req, err := http.NewRequest("POST", b.endpoint, bytes.NewReader(message))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err := b.client.Do(req)
	if err != nil {
		return err
	}
	if resp != nil {
		ioutil.ReadAll(resp.Body)
		resp.Body.Close()
	}
	return nil
}

// Receive returns a read-only channel of incoming messages.
func (b *WebSocketBridge) Receive() (<-chan []byte, error) {
	return b.in, nil
}

// Close shuts down the bridge and stops all background activity.
func (b *WebSocketBridge) Close() error {
	close(b.done)
	b.mu.Lock()
	b.connected = false
	b.mu.Unlock()
	return nil
}

// OnMessage registers a callback to be invoked when a message is received.
func (b *WebSocketBridge) OnMessage(handler func([]byte)) {
	b.mu.Lock()
	b.onMessage = handler
	b.mu.Unlock()
}

// pollLoop simulates a WebSocket receive loop by polling the endpoint periodically.
func (b *WebSocketBridge) pollLoop() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-b.done:
			return
		case <-ticker.C:
			b.mu.RLock()
			url := b.endpoint
			conn := b.connected
			b.mu.RUnlock()
			if !conn || url == "" {
				continue
			}
			// Simple long-poll style: GET the endpoint to fetch any pending messages.
			resp, err := http.Get(url)
			if err != nil {
				// back off a bit before retrying
				time.Sleep(b.backoff)
				if b.backoff < b.maxBackoff {
					b.backoff *= 2
				}
				continue
			}
			b.backoff = 500 * time.Millisecond
			if resp != nil {
				data, _ := ioutil.ReadAll(resp.Body)
				resp.Body.Close()
				if len(data) > 0 {
					// deliver to receiver
					b.mu.RLock()
					cb := b.onMessage
					b.mu.RUnlock()
					if cb != nil {
						cb(data)
					}
					// also push to in channel for any listeners
					select {
					case b.in <- data:
					default:
					}
				}
			}
		}
	}
}
