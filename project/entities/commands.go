package entities

type TicketRefund struct {
	Header   EventHeader `json:"header"`
	TicketID string      `json:"ticket_id"`
}
