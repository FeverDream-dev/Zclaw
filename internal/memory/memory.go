package memory

import (
	"context"
	"fmt"
	"time"

	"github.com/zclaw/zclaw/internal/providers"
)

type ConversationID string

func (id ConversationID) String() string { return string(id) }

type MessageID string

type Conversation struct {
	ID         ConversationID    `json:"id"`
	AgentID    string            `json:"agent_id"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
	TokenCount int               `json:"token_count"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type StoredMessage struct {
	ID             ConversationID        `json:"id"`
	ConversationID ConversationID        `json:"conversation_id"`
	Role           providers.MessageRole `json:"role"`
	Content        string                `json:"content"`
	Seq            int                   `json:"seq"`
	CreatedAt      time.Time             `json:"created_at"`
}

type Summary struct {
	ID             ConversationID `json:"id"`
	AgentID        string         `json:"agent_id"`
	ConversationID ConversationID `json:"conversation_id"`
	Content        string         `json:"content"`
	TokenCount     int            `json:"token_count"`
	CreatedAt      time.Time      `json:"created_at"`
}

type ArtifactID string

type Artifact struct {
	ID        ArtifactID        `json:"id"`
	AgentID   string            `json:"agent_id"`
	TaskID    string            `json:"task_id,omitempty"`
	Filename  string            `json:"filename"`
	FilePath  string            `json:"file_path"`
	MimeType  string            `json:"mime_type"`
	SizeBytes int64             `json:"size_bytes"`
	CreatedAt time.Time         `json:"created_at"`
	ExpiresAt *time.Time        `json:"expires_at,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

type RetentionPolicy struct {
	MaxConversationAge time.Duration `json:"max_conversation_age"`
	MaxSummaryAge      time.Duration `json:"max_summary_age"`
	MaxArtifactAge     time.Duration `json:"max_artifact_age"`
	SummarizeAfter     int           `json:"summarize_after"`
	CompressAfter      time.Duration `json:"compress_after"`
}

func DefaultRetentionPolicy() RetentionPolicy {
	return RetentionPolicy{
		MaxConversationAge: 30 * 24 * time.Hour,
		MaxSummaryAge:      90 * 24 * time.Hour,
		MaxArtifactAge:     90 * 24 * time.Hour,
		SummarizeAfter:     50,
		CompressAfter:      7 * 24 * time.Hour,
	}
}

type ConversationStore interface {
	Create(ctx context.Context, agentID string) (*Conversation, error)
	Get(ctx context.Context, id ConversationID) (*Conversation, error)
	ListByAgent(ctx context.Context, agentID string, limit, offset int) ([]Conversation, error)
	AddMessage(ctx context.Context, convID ConversationID, role providers.MessageRole, content string) (*StoredMessage, error)
	GetMessages(ctx context.Context, convID ConversationID, limit int) ([]StoredMessage, error)
	GetRecent(ctx context.Context, agentID string, limit int) ([]StoredMessage, error)
	GetTokenCount(ctx context.Context, convID ConversationID) (int, error)
	Delete(ctx context.Context, id ConversationID) error
}

type SummaryStore interface {
	Save(ctx context.Context, agentID string, convID ConversationID, content string, tokenCount int) (*Summary, error)
	GetLatest(ctx context.Context, agentID string) (*Summary, error)
	ListByAgent(ctx context.Context, agentID string, limit int) ([]Summary, error)
}

type ArtifactStore interface {
	Save(ctx context.Context, artifact Artifact) error
	Get(ctx context.Context, id ArtifactID) (*Artifact, error)
	ListByAgent(ctx context.Context, agentID string, limit, offset int) ([]Artifact, error)
	Delete(ctx context.Context, id ArtifactID) error
	Prune(ctx context.Context, olderThan time.Duration) (int, error)
}

type SQLiteConversationStore struct {
	db interface {
		ExecContext(ctx context.Context, query string, args ...any) (any, error)
		QueryRowContext(ctx context.Context, query string, args ...any) any
		QueryContext(ctx context.Context, query string, args ...any) (any, error)
	}
}

func NewConversationStore() *SQLiteConversationStore {
	return &SQLiteConversationStore{}
}

func (s *SQLiteConversationStore) Create(ctx context.Context, agentID string) (*Conversation, error) {
	id := ConversationID(fmt.Sprintf("conv-%x", time.Now().UnixMilli()))
	conv := &Conversation{
		ID:        id,
		AgentID:   agentID,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	return conv, nil
}

func (s *SQLiteConversationStore) Get(ctx context.Context, id ConversationID) (*Conversation, error) {
	return nil, fmt.Errorf("not implemented: requires db")
}

func (s *SQLiteConversationStore) ListByAgent(ctx context.Context, agentID string, limit, offset int) ([]Conversation, error) {
	return nil, fmt.Errorf("not implemented: requires db")
}

func (s *SQLiteConversationStore) AddMessage(ctx context.Context, convID ConversationID, role providers.MessageRole, content string) (*StoredMessage, error) {
	return nil, fmt.Errorf("not implemented: requires db")
}

func (s *SQLiteConversationStore) GetMessages(ctx context.Context, convID ConversationID, limit int) ([]StoredMessage, error) {
	return nil, fmt.Errorf("not implemented: requires db")
}

func (s *SQLiteConversationStore) GetRecent(ctx context.Context, agentID string, limit int) ([]StoredMessage, error) {
	return nil, fmt.Errorf("not implemented: requires db")
}

func (s *SQLiteConversationStore) GetTokenCount(ctx context.Context, convID ConversationID) (int, error) {
	return 0, fmt.Errorf("not implemented: requires db")
}

func (s *SQLiteConversationStore) Delete(ctx context.Context, id ConversationID) error {
	return fmt.Errorf("not implemented: requires db")
}
