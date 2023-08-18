package main

import "context"

var deduplicationPaymentMap = map[string]struct{}{}

type PaymentTaken struct {
	PaymentID string
	Amount    int
}

type PaymentsHandler struct {
	repo *PaymentsRepository
}

func NewPaymentsHandler(repo *PaymentsRepository) *PaymentsHandler {
	return &PaymentsHandler{repo: repo}
}

func (p *PaymentsHandler) HandlePaymentTaken(ctx context.Context, event *PaymentTaken) error {
	return p.repo.SavePaymentTaken(ctx, event)
}

type PaymentsRepository struct {
	payments []PaymentTaken
}

func (p *PaymentsRepository) Payments() []PaymentTaken {
	return p.payments
}

func NewPaymentsRepository() *PaymentsRepository {
	return &PaymentsRepository{}
}

func (p *PaymentsRepository) SavePaymentTaken(ctx context.Context, event *PaymentTaken) error {
	if _, ok := deduplicationPaymentMap[event.PaymentID]; ok {
		return nil
	}
	deduplicationPaymentMap[event.PaymentID] = struct{}{}
	p.payments = append(p.payments, *event)
	return nil
}
