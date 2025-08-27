package kafka

import (
	"context"

	kafka "github.com/segmentio/kafka-go"
)

type Publisher struct {
	writer *kafka.Writer
}

func NewPublisher(brokers string, topic string) (*Publisher, error) {
	w := &kafka.Writer{
		Addr:     kafka.TCP(brokers),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	}
	return &Publisher{writer: w}, nil
}

func (p *Publisher) Publish(ctx context.Context, payload []byte) error {
	return p.writer.WriteMessages(ctx, kafka.Message{
		Value: payload,
	})
}

func (p *Publisher) Close() error {
	return p.writer.Close()
}
