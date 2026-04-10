package channels

type ChannelType string

const (
	ChannelCLI     ChannelType = "cli"
	ChannelHTTP    ChannelType = "http"
	ChannelWebhook ChannelType = "webhook"
	ChannelWS      ChannelType = "websocket"
)

type Message struct {
	AgentID  string            `json:"agent_id"`
	Channel  ChannelType       `json:"channel"`
	Content  string            `json:"content"`
	Metadata map[string]string `json:"metadata,omitempty"`
	ReplyTo  string            `json:"reply_to,omitempty"`
}

type Channel interface {
	Send(msg Message) error
	Receive() (<-chan Message, error)
	Type() ChannelType
	Close() error
}
