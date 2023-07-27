package message

import (
	"encoding/json"
	"tickets/entities"
	"tickets/message/event"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-redisstream/pkg/redisstream"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/redis/go-redis/v9"
)

const brokenMessageID = "2beaf5bc-d5e4-4653-b075-2b36bbf28949"

func NewWatermillRouter(receiptsService event.ReceiptsService, spreadsheetsService event.SpreadsheetsAPI, rdb redis.UniversalClient, watermillLogger watermill.LoggerAdapter) *message.Router {

	router, err := message.NewRouter(message.RouterConfig{}, watermillLogger)
	if err != nil {
		panic(err)
	}

	handler := event.NewHandler(spreadsheetsService, receiptsService)

	useMiddlewares(router, watermillLogger)

	issueReceiptSub, err := redisstream.NewSubscriber(redisstream.SubscriberConfig{
		Client:        rdb,
		ConsumerGroup: "issue-receipt",
	}, watermillLogger)
	if err != nil {
		panic(err)
	}

	appendToTrackerSub, err := redisstream.NewSubscriber(redisstream.SubscriberConfig{
		Client:        rdb,
		ConsumerGroup: "append-to-tracker",
	}, watermillLogger)
	if err != nil {
		panic(err)
	}

	// ticketRefundSub, err := redisstream.NewSubscriber(redisstream.SubscriberConfig{
	// 	Client:        rdb,
	// 	ConsumerGroup: "ticket-to-refund",
	// }, watermillLogger)
	// if err != nil {
	// 	panic(err)
	// }

	cancelTicketSub, err := redisstream.NewSubscriber(redisstream.SubscriberConfig{
		Client:        rdb,
		ConsumerGroup: "cancel-ticket",
	}, watermillLogger)
	if err != nil {
		panic(err)
	}

	router.AddNoPublisherHandler(
		"receipts-hdl",
		"TicketBookingConfirmed",
		issueReceiptSub,
		func(msg *message.Message) error {
			if msg.UUID == "2beaf5bc-d5e4-4653-b075-2b36bbf28949" {
				return nil
			}
			if msg.Metadata.Get("type") != "TicketBookingConfirmed" {
				return nil
			}
			var event entities.TicketBookingConfirmed
			err := json.Unmarshal(msg.Payload, &event)
			if err != nil {
				return err
			}
			if event.Price.Currency == "" {
				event.Price.Currency = "USD"
			}
			return handler.IssueReceipt(msg.Context(), event)
		},
	)

	router.AddNoPublisherHandler(
		"spreadsheets-hdl",
		"TicketBookingConfirmed",
		appendToTrackerSub,
		func(msg *message.Message) error {
			if msg.UUID == "2beaf5bc-d5e4-4653-b075-2b36bbf28949" {
				return nil
			}
			if msg.Metadata.Get("type") != "TicketBookingConfirmed" {
				return nil
			}
			var event entities.TicketBookingConfirmed
			err := json.Unmarshal(msg.Payload, &event)
			if err != nil {
				return err
			}
			if event.Price.Currency == "" {
				event.Price.Currency = "USD"
			}
			return handler.AppendTracker(msg.Context(), event)
		},
	)

	router.AddNoPublisherHandler(
		"ticketToRefund-hdl",
		"TicketBookingCanceled",
		cancelTicketSub,
		func(msg *message.Message) error {
			if msg.Metadata.Get("type") == "TicketBookingCanceled" {
				return nil
			}
			var event entities.TicketBookingCanceled
			err := json.Unmarshal(msg.Payload, &event)
			if err != nil {
				return err
			}
			return handler.CancelTicket(msg.Context(), event)
		},
	)
	return router
}
