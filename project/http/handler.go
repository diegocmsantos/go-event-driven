package http

import (
	"context"

	"tickets/repository"

	"github.com/ThreeDotsLabs/watermill/components/cqrs"
)

type Handler struct {
	eventBus              *cqrs.EventBus
	spreadsheetsAPIClient SpreadsheetsAPI
	ticketRepository      repository.TicketRepositoryInt
}

type SpreadsheetsAPI interface {
	AppendRow(ctx context.Context, spreadsheetName string, row []string) error
}
