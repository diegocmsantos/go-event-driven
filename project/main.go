package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"

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
	"golang.org/x/sync/errgroup"
)

type PaymentCompleted struct {
	OrderID     string
	ConfirmedAt string
}

type TicketsConfirmationRequest struct {
	Tickets []string `json:"tickets"`
}

func main() {
	log.Init(logrus.InfoLevel)
	watermillLogger := log.NewWatermill(logrus.NewEntry(logrus.StandardLogger()))

	ctx := context.Background()
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)

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

	router, err := message.NewRouter(message.RouterConfig{}, watermillLogger)
	if err != nil {
		panic(err)
	}

	go func() {
		sub, err := redisstream.NewSubscriber(redisstream.SubscriberConfig{
			Client: rdb,
		}, watermillLogger)
		if err != nil {
			panic(err)
		}
		receiptsClient := NewReceiptsClient(clients)
		router.AddNoPublisherHandler(
			"receipts-hdl",
			"issue-receipt",
			sub,
			func(msg *message.Message) error {
				err = receiptsClient.IssueReceipt(context.Background(), string(msg.Payload))
				if err != nil {
					logrus.WithError(err).Error("failed to issue the receipt")
					return err
				}
				return nil
			},
		)
	}()

	go func() {
		sub, err := redisstream.NewSubscriber(redisstream.SubscriberConfig{
			Client: rdb,
		}, watermillLogger)
		if err != nil {
			panic(err)
		}
		spreadsheetsClient := NewSpreadsheetsClient(clients)

		router.AddNoPublisherHandler(
			"spreadsheets-hdl",
			"append-to-tracker",
			sub,
			func(msg *message.Message) error {

				err = spreadsheetsClient.AppendRow(context.Background(), "tickets-to-print", []string{string(msg.Payload)})
				if err != nil {
					logrus.WithError(err).Error("failed to append to tracker")
					return err
				}
				return nil
			},
		)
	}()

	go func() {
		sub, err := redisstream.NewSubscriber(redisstream.SubscriberConfig{
			Client: rdb,
		}, watermillLogger)
		if err != nil {
			panic(err)
		}
		pub, err := redisstream.NewPublisher(redisstream.PublisherConfig{
			Client: rdb,
		}, watermillLogger)
		if err != nil {
			panic(err)
		}

		router.AddHandler(
			"payment-complete-hdl",
			"payment-complete",
			sub,
			"order-confirmed",
			pub,
			func(msg *message.Message) ([]*message.Message, error) {
				var resultMsgs []*message.Message
				var paymentCompleted PaymentCompleted
				err := json.Unmarshal(msg.Payload, &paymentCompleted)
				if err != nil {
					return nil, nil
				}
				byts, err := json.Marshal(paymentCompleted)
				if err != nil {
					return nil, err
				}
				resultMsgs = append(resultMsgs, message.NewMessage(watermill.NewUUID(), byts))
				return resultMsgs, nil
			},
		)
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

	e.GET("/health", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

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
