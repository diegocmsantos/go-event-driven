package event

import (
	"context"

	"tickets/entities"
	"tickets/repository"

	"github.com/ThreeDotsLabs/go-event-driven/common/log"
)

func (h EventHandler) CreateTicket(ctx context.Context, event *entities.TicketBookingConfirmed) error {
	log.FromContext(ctx).Info("Appending ticket to the tracker")

	return h.ticketRepository.CreateTicket(ctx, repository.CreateTicketInput{
		TicketID:      event.TicketID,
		PriceAmount:   event.Price.Amount,
		PriceCurrency: event.Price.Currency,
		CustomerEmail: event.CustomerEmail,
	})
}
