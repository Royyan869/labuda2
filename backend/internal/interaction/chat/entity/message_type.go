package entity

// MessageType represents the type of chat message.
// Media messages (image, video, audio, file) are sent as "text" with mediaUrls attachment.
type MessageType string

const (
	// MessageTypeText is a plain text message (also used for media messages with attachments).
	MessageTypeText MessageType = "text"

	// MessageTypeNegotiationProposal is a message containing a negotiation proposal.
	MessageTypeNegotiationProposal MessageType = "negotiation_proposal"

	// MessageTypeShippingQuote is a message containing a shipping quote.
	MessageTypeShippingQuote MessageType = "shipping_quote"

	// MessageTypeSystem is a system-generated message.
	MessageTypeSystem MessageType = "system"
)

// String returns the string representation of the message type.
func (m MessageType) String() string {
	return string(m)
}

// IsValid checks if the message type is valid.
func (m MessageType) IsValid() bool {
	switch m {
	case MessageTypeText, MessageTypeNegotiationProposal, MessageTypeShippingQuote, MessageTypeSystem:
		return true
	default:
		return false
	}
}


