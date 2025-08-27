package kafka

import (
	"context"
	"errors"
	"log"
	"time"

	kafka "github.com/segmentio/kafka-go"

	"l0-demo/internal/service"
)

type Config struct {
	Brokers []string
	GroupID string
	Topic   string
}

type Consumer struct {
	reader *kafka.Reader
	svc    service.Order
}

func NewConsumer(cfg Config, svc service.Order) *Consumer {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        cfg.Brokers,
		GroupID:        cfg.GroupID,
		Topic:          cfg.Topic,
		MinBytes:       1,   
		MaxBytes:       10e6,  
		MaxWait:        250 * time.Millisecond,
		CommitInterval: 0, 
	})
	return &Consumer{reader: r, svc: svc}
}

func (c *Consumer) Subscribe(ctx context.Context) error {
	for {
		m, err := c.reader.FetchMessage(ctx)
		if err != nil {

			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			log.Printf("kafka fetch error: %v", err)

			select {
			case <-time.After(500 * time.Millisecond):
				continue
			case <-ctx.Done():
				return nil
			}
		}

		if err := c.svc.HandleMessage(ctx, m.Value); err != nil {
			log.Printf("handle message failed (offset %d, partition %d): %v", m.Offset, m.Partition, err)
			select {
			case <-time.After(500 * time.Millisecond):
				continue
			case <-ctx.Done():
				return nil
			}
		}

		if err := c.reader.CommitMessages(ctx, m); err != nil {
			log.Printf("commit failed (offset %d, partition %d): %v", m.Offset, m.Partition, err)
		}
	}
}

func (c *Consumer) Close(ctx context.Context) error {
	return c.reader.Close()
}
