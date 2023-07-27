package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"tickets/entities"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/labstack/echo/v4"
)

type ticketsStatusRequest struct {
	Tickets []ticketStatusRequest `json:"tickets"`
}

type ticketStatusRequest struct {
	TicketID      string         `json:"ticket_id"`
	Status        string         `json:"status"`
	Price         entities.Money `json:"price"`
	CustomerEmail string         `json:"customer_email"`
	BookingID     string         `json:"booking_id"`
}

func (h Handler) PostTicketsStatus(c echo.Context) error {
	var request ticketsStatusRequest
	err := c.Bind(&request)
	if err != nil {
		return err
	}

	for _, ticket := range request.Tickets {
		ticketEvent := entities.TicketBookingConfirmed{
			Header:        entities.NewHeader(),
			TicketID:      ticket.TicketID,
			CustomerEmail: ticket.CustomerEmail,
			Price:         ticket.Price,
		}
		payload, err := json.Marshal(ticketEvent)
		if err != nil {
			return err
		}
		msg := message.NewMessage(watermill.NewUUID(), payload)
		msg.Metadata.Set("correlation_id", c.Request().Header.Get("Correlation-ID"))
		switch ticket.Status {
		case "confirmed":
			msg.Metadata.Set("type", "TicketBookingConfirmed")
			err = h.publisher.Publish("TicketBookingConfirmed", msg)
		case "canceled":
			msg.Metadata.Set("type", "TicketBookingCanceled")
			err = h.publisher.Publish("TicketBookingCanceled", msg)
		default:
			return fmt.Errorf("unknown ticket status: %s", ticket.Status)
		}
		if err != nil {
			return err
		}
	}

	return c.NoContent(http.StatusOK)
}
