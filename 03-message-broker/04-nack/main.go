package main

import (
	"context"
	"errors"

	"github.com/ThreeDotsLabs/watermill/message"
)

type AlarmClient interface {
	StartAlarm() error
	StopAlarm() error
}

func ConsumeMessages(sub message.Subscriber, alarmClient AlarmClient) {
	messages, err := sub.Subscribe(context.Background(), "smoke_sensor")
	if err != nil {
		panic(err)
	}

	for msg := range messages {
		if string(msg.Payload) == "0" {
			err = alarmClient.StopAlarm()
		} else if string(msg.Payload) == "1" {
			err = alarmClient.StartAlarm()
		} else {
			err = errors.New("Payload different from 0 or 1")
		}
		if err != nil {
			msg.Nack()
			continue
		}
		msg.Ack()
	}
}
