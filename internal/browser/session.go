package browser

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type SessionID string

func (id SessionID) String() string { return string(id) }

type SessionState string

const (
	SessionActive  SessionState = "active"
	SessionIdle    SessionState = "idle"
	SessionExpired SessionState = "expired"
	SessionClosed  SessionState = "closed"
)

type Session struct {
	ID         SessionID    `json:"id"`
	AgentID    string       `json:"agent_id"`
	TaskID     string       `json:"task_id"`
	State      SessionState `json:"state"`
	WSURL      string       `json:"ws_url"`
	CreatedAt  time.Time    `json:"created_at"`
	LastUsedAt time.Time    `json:"last_used_at"`
	ExpiresAt  *time.Time   `json:"expires_at,omitempty"`
}

type CaptureResult struct {
	Data     []byte `json:"data"`
	MimeType string `json:"mime_type"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
}

type NavigateResult struct {
	URL        string `json:"url"`
	Title      string `json:"title"`
	StatusCode int    `json:"status_code"`
}

type DOMSnapshot struct {
	HTML  string `json:"html"`
	Text  string `json:"text"`
	URL   string `json:"url"`
	Title string `json:"title"`
}

type BrowserConfig struct {
	WSURL           string        `json:"ws_url"`
	MaxSessions     int           `json:"max_sessions"`
	SessionTimeout  time.Duration `json:"session_timeout"`
	IdleTimeout     time.Duration `json:"idle_timeout"`
	Headless        bool          `json:"headless"`
	DefaultViewport Viewport      `json:"default_viewport"`
}

type Viewport struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

func DefaultBrowserConfig() BrowserConfig {
	return BrowserConfig{
		MaxSessions:     5,
		SessionTimeout:  30 * time.Minute,
		IdleTimeout:     10 * time.Minute,
		Headless:        true,
		DefaultViewport: Viewport{Width: 1280, Height: 720},
	}
}

type SessionManager interface {
	Acquire(ctx context.Context, agentID, taskID string) (*Session, error)
	Release(ctx context.Context, sessionID SessionID) error
	Screenshot(ctx context.Context, sessionID SessionID) (*CaptureResult, error)
	Navigate(ctx context.Context, sessionID SessionID, url string) (*NavigateResult, error)
	GetDOM(ctx context.Context, sessionID SessionID) (*DOMSnapshot, error)
	Click(ctx context.Context, sessionID SessionID, selector string) error
	Type(ctx context.Context, sessionID SessionID, selector, text string) error
	GetSession(ctx context.Context, sessionID SessionID) (*Session, error)
	ListSessions(ctx context.Context) ([]Session, error)
	ReapExpired(ctx context.Context) (int, error)
	ActiveCount() int
}

type LocalSessionManager struct {
	config   BrowserConfig
	mu       sync.Mutex
	sessions map[SessionID]*Session
	counter  int
}

func NewSessionManager(config BrowserConfig) *LocalSessionManager {
	return &LocalSessionManager{
		config:   config,
		sessions: make(map[SessionID]*Session),
	}
}

func (m *LocalSessionManager) Acquire(ctx context.Context, agentID, taskID string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	activeCount := 0
	for _, s := range m.sessions {
		if s.State == SessionActive || s.State == SessionIdle {
			activeCount++
		}
	}
	if activeCount >= m.config.MaxSessions {
		return nil, fmt.Errorf("browser pool full (%d/%d sessions)", activeCount, m.config.MaxSessions)
	}

	m.counter++
	session := &Session{
		ID:         SessionID(fmt.Sprintf("bs-%x-%d", time.Now().UnixMilli(), m.counter)),
		AgentID:    agentID,
		TaskID:     taskID,
		State:      SessionActive,
		WSURL:      m.config.WSURL,
		CreatedAt:  time.Now().UTC(),
		LastUsedAt: time.Now().UTC(),
	}

	expires := time.Now().UTC().Add(m.config.SessionTimeout)
	session.ExpiresAt = &expires

	m.sessions[session.ID] = session
	return session, nil
}

func (m *LocalSessionManager) Release(ctx context.Context, sessionID SessionID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}
	session.State = SessionClosed
	delete(m.sessions, sessionID)
	return nil
}

func (m *LocalSessionManager) Screenshot(ctx context.Context, sessionID SessionID) (*CaptureResult, error) {
	return nil, fmt.Errorf("screenshot requires connected browser worker")
}

func (m *LocalSessionManager) Navigate(ctx context.Context, sessionID SessionID, url string) (*NavigateResult, error) {
	return nil, fmt.Errorf("navigate requires connected browser worker")
}

func (m *LocalSessionManager) GetDOM(ctx context.Context, sessionID SessionID) (*DOMSnapshot, error) {
	return nil, fmt.Errorf("getdom requires connected browser worker")
}

func (m *LocalSessionManager) Click(ctx context.Context, sessionID SessionID, selector string) error {
	return fmt.Errorf("click requires connected browser worker")
}

func (m *LocalSessionManager) Type(ctx context.Context, sessionID SessionID, selector, text string) error {
	return fmt.Errorf("type requires connected browser worker")
}

func (m *LocalSessionManager) GetSession(ctx context.Context, sessionID SessionID) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}
	return s, nil
}

func (m *LocalSessionManager) ListSessions(ctx context.Context) ([]Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []Session
	for _, s := range m.sessions {
		result = append(result, *s)
	}
	return result, nil
}

func (m *LocalSessionManager) ReapExpired(ctx context.Context) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().UTC()
	reaped := 0
	for id, s := range m.sessions {
		if s.ExpiresAt != nil && now.After(*s.ExpiresAt) {
			s.State = SessionExpired
			delete(m.sessions, id)
			reaped++
		}
		if now.Sub(s.LastUsedAt) > m.config.IdleTimeout {
			s.State = SessionExpired
			delete(m.sessions, id)
			reaped++
		}
	}
	return reaped, nil
}

func (m *LocalSessionManager) ActiveCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	count := 0
	for _, s := range m.sessions {
		if s.State == SessionActive || s.State == SessionIdle {
			count++
		}
	}
	return count
}
