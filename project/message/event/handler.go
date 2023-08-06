package event

import (
	"context"
	"tickets/entities"
)

type EventHandler struct {
	spreadsheetsService SpreadsheetsAPI
	receiptsService     ReceiptsService
}

func NewHandler(
	spreadsheetsService SpreadsheetsAPI,
	receiptsService ReceiptsService,
) EventHandler {
	if spreadsheetsService == nil {
		panic("missing spreadsheetsService")
	}
	if receiptsService == nil {
		panic("missing receiptsService")
	}

	return EventHandler{
		spreadsheetsService: spreadsheetsService,
		receiptsService:     receiptsService,
	}
}

type SpreadsheetsAPI interface {
	AppendRow(ctx context.Context, sheetName string, row []string) error
}

type ReceiptsService interface {
	IssueReceipt(ctx context.Context, request entities.IssueReceiptRequest) (entities.IssueReceiptResponse, error)
}
