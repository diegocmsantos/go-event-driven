package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/ThreeDotsLabs/go-event-driven/common/clients"
	"github.com/ThreeDotsLabs/go-event-driven/common/clients/receipts"
	"github.com/ThreeDotsLabs/go-event-driven/common/clients/spreadsheets"
	commonHTTP "github.com/ThreeDotsLabs/go-event-driven/common/http"
	"github.com/ThreeDotsLabs/go-event-driven/common/log"
	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-redisstream/pkg/redisstream"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

type TicketsConfirmationRequest struct {
	Tickets []string `json:"tickets"`
}

func main() {
	log.Init(logrus.InfoLevel)
	watermillLogger := log.NewWatermill(logrus.NewEntry(logrus.StandardLogger()))

	rdb := redis.NewClient(&redis.Options{
		Addr: os.Getenv("REDIS_ADDR"),
	})

	clients, err := clients.NewClients(os.Getenv("GATEWAY_ADDR"), nil)
	if err != nil {
		panic(err)
	}

	publisher, err := redisstream.NewPublisher(redisstream.PublisherConfig{
		Client: rdb,
	}, watermillLogger)

	subscriber, err := redisstream.NewSubscriber(redisstream.SubscriberConfig{
		Client: rdb,
	}, watermillLogger)

	go func() {
		messages, err := subscriber.Subscribe(context.Background(), "issue-receipt")
		if err != nil {
			watermillLogger.Error("error subscribing the topic", err, watermill.LogFields{
				"topic": "issue-receipt",
			})
		}
		receiptsClient := NewReceiptsClient(clients)

		for msg := range messages {
			err = receiptsClient.IssueReceipt(context.Background(), string(msg.Payload))
			if err != nil {
				logrus.WithError(err).Error("failed to issue the receipt")
				msg.Nack()
				continue
			}
			msg.Ack()
		}
	}()

	go func() {
		messages, err := subscriber.Subscribe(context.Background(), "append-to-tracker")
		if err != nil {
			watermillLogger.Error("error subscribing the topic", err, watermill.LogFields{
				"topic": "append-to-tracker",
			})
		}
		spreadsheetsClient := NewSpreadsheetsClient(clients)

		for msg := range messages {
			err = spreadsheetsClient.AppendRow(context.Background(), "tickets-to-print", []string{string(msg.Payload)})
			if err != nil {
				logrus.WithError(err).Error("failed to append to tracker")
				msg.Nack()
				continue
			}
			msg.Ack()
		}
	}()

	e := commonHTTP.NewEcho()

	e.POST("/tickets-confirmation", func(c echo.Context) error {
		var request TicketsConfirmationRequest
		err := c.Bind(&request)
		if err != nil {
			return err
		}

		for _, ticket := range request.Tickets {
			publisher.Publish("issue-receipt", message.NewMessage(watermill.NewUUID(), []byte(ticket)))
			publisher.Publish("append-to-tracker", message.NewMessage(watermill.NewUUID(), []byte(ticket)))
		}

		return c.NoContent(http.StatusOK)
	})

	logrus.Info("Server starting...")

	err = e.Start(":8080")
	if err != nil && err != http.ErrServerClosed {
		panic(err)
	}
}

type ReceiptsClient struct {
	clients *clients.Clients
}

func NewReceiptsClient(clients *clients.Clients) ReceiptsClient {
	return ReceiptsClient{
		clients: clients,
	}
}

func (c ReceiptsClient) IssueReceipt(ctx context.Context, ticketID string) error {
	body := receipts.PutReceiptsJSONRequestBody{
		TicketId: ticketID,
	}

	receiptsResp, err := c.clients.Receipts.PutReceiptsWithResponse(ctx, body)
	if err != nil {
		return err
	}
	if receiptsResp.StatusCode() != http.StatusOK {
		return fmt.Errorf("unexpected status code: %v", receiptsResp.StatusCode())
	}

	return nil
}

type SpreadsheetsClient struct {
	clients *clients.Clients
}

func NewSpreadsheetsClient(clients *clients.Clients) SpreadsheetsClient {
	return SpreadsheetsClient{
		clients: clients,
	}
}

func (c SpreadsheetsClient) AppendRow(ctx context.Context, spreadsheetName string, row []string) error {
	request := spreadsheets.PostSheetsSheetRowsJSONRequestBody{
		Columns: row,
	}

	sheetsResp, err := c.clients.Spreadsheets.PostSheetsSheetRowsWithResponse(ctx, spreadsheetName, request)
	if err != nil {
		return err
	}
	if sheetsResp.StatusCode() != http.StatusOK {
		return fmt.Errorf("unexpected status code: %v", sheetsResp.StatusCode())
	}

	return nil
}

type Task int

const (
	TaskIssueReceipt = iota
	TaskAppendToTracker
)

type Message struct {
	Task     Task
	TicketID string
}
