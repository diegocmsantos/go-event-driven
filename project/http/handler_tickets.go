package http

import (
	"fmt"
	"net/http"
	"tickets/entities"

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
		switch ticket.Status {
		case "confirmed":
			if err = h.eventBus.Publish(c.Request().Context(), ticketEvent); err != nil {
				return fmt.Errorf("failed to publish TicketBookingConfirmed event: %w", err)
			}
		case "canceled":
			if err = h.eventBus.Publish(c.Request().Context(), ticketEvent); err != nil {
				return fmt.Errorf("failed to publish TicketBookingCanceled event: %w", err)
			}
		default:
			return fmt.Errorf("unknown ticket status: %s", ticket.Status)
		}
		if err != nil {
			return err
		}
	}

	return c.NoContent(http.StatusOK)
}
