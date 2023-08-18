package event

import (
	"context"

	"tickets/entities"
	"tickets/repository"
)

type EventHandler struct {
	spreadsheetsService SpreadsheetsAPI
	receiptsService     ReceiptsService
	ticketRepository    repository.TicketRepositoryInt
}

func NewHandler(
	spreadsheetsService SpreadsheetsAPI,
	receiptsService ReceiptsService,
	ticketRepository repository.TicketRepositoryInt,
) EventHandler {
	if spreadsheetsService == nil {
		panic("missing spreadsheetsService")
	}
	if receiptsService == nil {
		panic("missing receiptsService")
	}
	if ticketRepository == nil {
		panic("missing ticketRepository")
	}

	return EventHandler{
		spreadsheetsService: spreadsheetsService,
		receiptsService:     receiptsService,
		ticketRepository:    ticketRepository,
	}
}

type SpreadsheetsAPI interface {
	AppendRow(ctx context.Context, sheetName string, row []string) error
}

type ReceiptsService interface {
	IssueReceipt(ctx context.Context, request entities.IssueReceiptRequest) (entities.IssueReceiptResponse, error)
}
