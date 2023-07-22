package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/ThreeDotsLabs/go-event-driven/common/clients"
	"github.com/ThreeDotsLabs/go-event-driven/common/clients/receipts"
	"github.com/ThreeDotsLabs/go-event-driven/common/clients/spreadsheets"
	commonHTTP "github.com/ThreeDotsLabs/go-event-driven/common/http"
	"github.com/ThreeDotsLabs/go-event-driven/common/log"
	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-redisstream/pkg/redisstream"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/message/router/middleware"
	"github.com/labstack/echo/v4"
	"github.com/lithammer/shortuuid/v3"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

type PaymentCompleted struct {
	OrderID     string
	ConfirmedAt string
}

type TicketsStatusRequest struct {
	Tickets []string `json:"tickets"`
}

type TicketsStatuses struct {
	Tickets []TicketStatus
}

type TicketStatus struct {
	TicketID      string `json:"ticket_id"`
	Status        string `json:"status"`
	CustomerEmail string `json:"customer_email"`
	Price         Money  `json:"price"`
}

type Money struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

type IssueReceiptPayload struct {
	TicketID string `json:"ticket_id"`
	Price    Money  `json:"price"`
}

type AppendToTrackerPayload struct {
	TicketID      string `json:"ticket_id"`
	CustomerEmail string `json:"customer_email"`
	Price         Money  `json:"price"`
}

type TicketToRefundPayload struct {
	TicketID      string `json:"ticket_id"`
	CustomerEmail string `json:"customer_email"`
	Price         Money  `json:"price"`
}

type Header struct {
	ID          string `json:"id"`
	PublishedAt string `json:"published_at"`
}

func NewHeader() Header {
	return Header{
		ID:          watermill.NewUUID(),
		PublishedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

type TicketBookingConfirmed struct {
	Header        Header `json:"header"`
	TicketID      string `json:"ticket_id"`
	CustomerEmail string `json:"customer_email"`
	Price         Money  `json:"price"`
}

type TicketBookingCanceled struct {
	Header        Header `json:"header"`
	TicketID      string `json:"ticket_id"`
	CustomerEmail string `json:"customer_email"`
	Price         Money  `json:"price"`
}

func main() {
	log.Init(logrus.InfoLevel)
	watermillLogger := log.NewWatermill(logrus.NewEntry(logrus.StandardLogger()))

	clients, err := clients.NewClients(
		os.Getenv("GATEWAY_ADDR"),
		func(ctx context.Context, req *http.Request) error {
			req.Header.Set("Correlation-ID", log.CorrelationIDFromContext(ctx))
			return nil
		},
	)
	if err != nil {
		panic(err)
	}

	receiptsClient := NewReceiptsClient(clients)
	spreadsheetsClient := NewSpreadsheetsClient(clients)

	rdb := redis.NewClient(&redis.Options{
		Addr: os.Getenv("REDIS_ADDR"),
	})

	pub, err := redisstream.NewPublisher(redisstream.PublisherConfig{
		Client: rdb,
	}, watermillLogger)

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

	ticketRefundSub, err := redisstream.NewSubscriber(redisstream.SubscriberConfig{
		Client:        rdb,
		ConsumerGroup: "ticket-to-refund",
	}, watermillLogger)
	if err != nil {
		panic(err)
	}

	e := commonHTTP.NewEcho()

	e.POST("/tickets-status", func(c echo.Context) error {
		var request TicketsStatuses
		err := c.Bind(&request)
		if err != nil {
			return err
		}

		for _, ticket := range request.Tickets {
			ticketEvent := TicketBookingConfirmed{
				Header:        NewHeader(),
				TicketID:      ticket.TicketID,
				CustomerEmail: ticket.CustomerEmail,
				Price:         ticket.Price,
			}
			payload, err := json.Marshal(ticketEvent)
			if err != nil {
				return err
			}
			msg := message.NewMessage(watermill.NewUUID(), payload)
			msg.Metadata.Set("correlation_id", c.Request().Header.Get("Correlation-ID"))
			switch ticket.Status {
			case "confirmed":
				msg.Metadata.Set("type", "TicketBookingConfirmed")
				err = pub.Publish("TicketBookingConfirmed", msg)
			case "canceled":
				msg.Metadata.Set("type", "TicketBookingCanceled")
				err = pub.Publish("TicketBookingCanceled", msg)
			default:
				return fmt.Errorf("unknown ticket status: %s", ticket.Status)
			}
			if err != nil {
				return err
			}
		}

		return c.NoContent(http.StatusOK)
	})

	e.GET("/health", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	router, err := message.NewRouter(message.RouterConfig{}, watermillLogger)
	if err != nil {
		panic(err)
	}
	router.AddMiddleware(correlationIDMiddleware)
	router.AddMiddleware(logMessageMiddleware)
	router.AddMiddleware(middleware.Retry{
		MaxRetries:      10,
		InitialInterval: time.Millisecond * 100,
		MaxInterval:     time.Second,
		Multiplier:      2,
		Logger:          watermillLogger,
	}.Middleware)

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
			var issueReceiptPayload IssueReceiptPayload
			err := json.Unmarshal(msg.Payload, &issueReceiptPayload)
			if err != nil {
				logrus.WithError(err).Error("failed to unmarshall")
				return nil
			}
			if issueReceiptPayload.Price.Currency == "" {
				issueReceiptPayload.Price.Currency = "USD"
			}
			err = receiptsClient.IssueReceipt(msg.Context(), IssueReceiptRequest{
				TicketID: issueReceiptPayload.TicketID,
				Price:    issueReceiptPayload.Price,
			})
			if err != nil {
				logrus.WithError(err).Error("failed to issue the receipt")
				return err
			}
			return nil
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
			var payload AppendToTrackerPayload
			err := json.Unmarshal(msg.Payload, &payload)
			if err != nil {
				logrus.WithError(err).Error("failed to unmarshall")
				return nil
			}
			if payload.Price.Currency == "" {
				payload.Price.Currency = "USD"
			}
			err = spreadsheetsClient.AppendRow(msg.Context(), "tickets-to-print",
				[]string{payload.TicketID, payload.CustomerEmail, payload.Price.Amount, payload.Price.Currency})
			if err != nil {
				logrus.WithError(err).Error("failed to append to tracker")
				return err
			}
			return nil
		},
	)

	router.AddNoPublisherHandler(
		"ticketToRefund-hdl",
		"TicketBookingCanceled",
		ticketRefundSub,
		func(msg *message.Message) error {
			if msg.Metadata.Get("type") == "TicketBookingCanceled" {
				return nil
			}
			var payload TicketToRefundPayload
			err := json.Unmarshal(msg.Payload, &payload)
			if err != nil {
				logrus.WithError(err).Error("failed to unmarshall")
				return nil
			}
			err = spreadsheetsClient.AppendRow(msg.Context(), "tickets-to-refund",
				[]string{payload.TicketID, payload.CustomerEmail, payload.Price.Amount, payload.Price.Currency})
			if err != nil {
				logrus.WithError(err).Error("failed to append to tracker")
				return err
			}
			return nil
		},
	)

	ctx := context.Background()
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return router.Run(ctx)
	})

	logrus.Info("Server starting...")

	g.Go(func() error {
		<-router.Running()

		err = e.Start(":8080")
		if err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	})

	g.Go(func() error {
		// Shut down the HTTP server
		<-ctx.Done()
		return e.Shutdown(ctx)
	})

	// Will block until all goroutines finish
	err = g.Wait()
	if err != nil {
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

type IssueReceiptRequest struct {
	TicketID string
	Price    Money
}

func (c ReceiptsClient) IssueReceipt(ctx context.Context, request IssueReceiptRequest) error {
	body := receipts.PutReceiptsJSONRequestBody{
		TicketId: request.TicketID,
		Price: receipts.Money{
			MoneyAmount:   request.Price.Amount,
			MoneyCurrency: request.Price.Currency,
		},
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

func logMessageMiddleware(h message.HandlerFunc) message.HandlerFunc {
	return func(msg *message.Message) ([]*message.Message, error) {
		logger := log.FromContext(msg.Context())
		msgs, err := h(msg)
		if err != nil {
			fmt.Println("Etrou aqui")
			logger.WithField("error", err).
				WithField("message_uuid", middleware.MessageCorrelationID(msg)).
				Error("Message handling error")
			return nil, err
		}
		logger.WithField("message_uuid", middleware.MessageCorrelationID(msg)).
			Info("Handling a message")
		return msgs, nil
	}
}

func correlationIDMiddleware(h message.HandlerFunc) message.HandlerFunc {
	return func(msg *message.Message) ([]*message.Message, error) {
		ctx := msg.Context()
		correlationID := msg.Metadata.Get("correlation_id")
		if correlationID == "" {
			correlationID = shortuuid.New()
		}
		ctx = log.ToContext(ctx, logrus.WithField("correlation_id", correlationID))
		ctx = log.ContextWithCorrelationID(ctx, correlationID)
		msg.SetContext(ctx)
		return h(msg)
	}
}
