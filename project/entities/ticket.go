package entities

type Ticket struct {
	TicketID      string `json:"ticket_id"`
	Price         Money  `json:"money"`
	CustomerEmail string `json:"customer_email"`
}
