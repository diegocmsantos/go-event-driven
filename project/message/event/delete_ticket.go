package event

import (
	"context"

	"tickets/entities"
	"tickets/repository"

	"github.com/ThreeDotsLabs/go-event-driven/common/log"
)

func (h EventHandler) DeleteTicket(ctx context.Context, event *entities.TicketBookingCanceled) error {
	log.FromContext(ctx).Info("Deleting ticket from the database")

	return h.ticketRepository.DeleteTicket(ctx, repository.DeleteTicketInput{
		TicketID: event.TicketID,
	})
}
