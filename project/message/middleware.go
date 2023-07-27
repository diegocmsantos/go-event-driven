package message

import (
	"time"

	"github.com/ThreeDotsLabs/go-event-driven/common/log"
	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/message/router/middleware"
	"github.com/lithammer/shortuuid/v3"
	"github.com/sirupsen/logrus"
)

func useMiddlewares(router *message.Router, watermillLogger watermill.LoggerAdapter) {
	router.AddMiddleware(middleware.Recoverer)

	router.AddMiddleware(middleware.Retry{
		MaxRetries:      10,
		InitialInterval: time.Millisecond * 100,
		MaxInterval:     time.Second,
		Multiplier:      2,
		Logger:          watermillLogger,
	}.Middleware)

	router.AddMiddleware(func(h message.HandlerFunc) message.HandlerFunc {
		return func(msg *message.Message) ([]*message.Message, error) {
			logger := log.FromContext(msg.Context()).
				WithFields(logrus.Fields{
					"message_id": msg.UUID,
					"payload":    string(msg.Payload),
					"metadata":   msg.Metadata,
				})

			logger.Info("Handling message")

			msgs, err := h(msg)
			if err != nil {
				logger.WithError(err).Error("Error while handling a message")
			}
			return msgs, nil
		}
	})

	router.AddMiddleware(func(h message.HandlerFunc) message.HandlerFunc {
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
	})

}
