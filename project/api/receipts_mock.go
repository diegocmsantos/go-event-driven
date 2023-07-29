package api

import (
	"context"
	"sync"
	"time"

	"tickets/entities"
)

type ReceiptsMock struct {
	mock           sync.Mutex
	IssuedReceipts []entities.IssueReceiptRequest
}

func (c *ReceiptsMock) IssueReceipt(ctx context.Context, request entities.IssueReceiptRequest) (entities.IssueReceiptResponse, error) {
	c.mock.Lock()
	defer c.mock.Unlock()
	c.IssuedReceipts = append(c.IssuedReceipts, request)
	return entities.IssueReceiptResponse{
		ReceiptNumber: "mocked-receipt-id",
		IssuedAt:      time.Now(),
	}, nil
}
