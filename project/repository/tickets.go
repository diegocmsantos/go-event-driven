package repository

import (
	"context"
	"fmt"

	"tickets/entities"

	"github.com/jmoiron/sqlx"
)

type TicketRepositoryInt interface {
	CreateTicket(ctx context.Context, createTicketInput CreateTicketInput) error
	DeleteTicket(ctx context.Context, createTicketInput DeleteTicketInput) error
	GetAll(ctx context.Context) ([]entities.Ticket, error)
}

type CreateTicketInput struct {
	TicketID      string
	PriceAmount   string
	PriceCurrency string
	CustomerEmail string
}

type DeleteTicketInput struct {
	TicketID string
}

type TicketRepository struct {
	db *sqlx.DB
}

func NewTicketRepository(db *sqlx.DB) *TicketRepository {
	if db == nil {
		panic("error creating ticket repository, db cannot be nil")
	}
	return &TicketRepository{db: db}
}

func (tr TicketRepository) CreateTicket(ctx context.Context, createTicketInput CreateTicketInput) error {
	_, err := tr.db.ExecContext(ctx, `
			INSERT INTO tickets (ticket_id, price_amount, price_currency, customer_email) 
			VALUES ($1, $2, $3, $4) on conflict (ticket_id) do nothing
		`,
		createTicketInput.TicketID,
		createTicketInput.PriceAmount,
		createTicketInput.PriceCurrency,
		createTicketInput.CustomerEmail,
	)
	if err != nil {
		return fmt.Errorf("error saving ticket: %w", err)
	}
	return nil
}

func (tr TicketRepository) DeleteTicket(ctx context.Context, deleteTicketInput DeleteTicketInput) error {
	_, err := tr.db.ExecContext(ctx, "DELETE FROM tickets WHERE ticket_id = $1",
		deleteTicketInput.TicketID,
	)
	if err != nil {
		return fmt.Errorf("error deleting ticket: %w", err)
	}
	return nil
}

func (tr TicketRepository) GetAll(ctx context.Context) ([]entities.Ticket, error) {
	var returnTickets []entities.Ticket

	err := tr.db.SelectContext(
		ctx,
		&returnTickets, `
			SELECT 
				ticket_id,
				price_amount as "price.amount", 
				price_currency as "price.currency",
				customer_email
			FROM 
			    tickets
		`,
	)
	if err != nil {
		return nil, err
	}

	return returnTickets, nil
}
