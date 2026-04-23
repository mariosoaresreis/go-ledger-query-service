package kafka

import (
	"context"
	"fmt"

	"github.com/segmentio/kafka-go"
	"github.com/sirupsen/logrus"
)

// Consumer reads events from Kafka and hands them to the Processor.
type Consumer struct {
	reader    *kafka.Reader
	processor *Processor
}

// NewConsumer creates a new Kafka consumer.
func NewConsumer(bootstrapServers, topic, groupID string, processor *Processor) *Consumer {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        []string{bootstrapServers},
		Topic:          topic,
		GroupID:        groupID,
		MinBytes:       1,
		MaxBytes:       10e6, // 10MB
		CommitInterval: 0,    // manual commit
	})
	return &Consumer{reader: r, processor: processor}
}

// Title satisfies the Module interface.
func (c *Consumer) Title() string { return "Kafka Consumer" }

// GracefulStop closes the reader.
func (c *Consumer) GracefulStop(_ context.Context) error {
	return c.reader.Close()
}

// Run starts the consume loop and returns a channel that receives fatal errors.
func (c *Consumer) Run(ctx context.Context) <-chan error {
	errCh := make(chan error, 1)
	go func() {
		logrus.Info("kafka consumer: starting consume loop")
		if err := c.consumeLoop(ctx); err != nil {
			errCh <- fmt.Errorf("kafka consumer: %w", err)
		}
		close(errCh)
	}()
	return errCh
}

func (c *Consumer) consumeLoop(ctx context.Context) error {
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil // clean shutdown
			}
			return fmt.Errorf("fetch message: %w", err)
		}

		if err := c.processor.Handle(ctx, msg); err != nil {
			logrus.WithError(err).Error("kafka consumer: failed to process message – skipping")
			// In production: send to dead-letter topic instead.
		}

		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			logrus.WithError(err).Warn("kafka consumer: failed to commit offset")
		}
	}
}
