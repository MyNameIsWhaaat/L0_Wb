package kafka

import (
	"context"
	"errors"
	"log"
	"strconv"
	"time"

	kafka "github.com/segmentio/kafka-go"

	"l0-demo/internal/service"
)

type Config struct {
	Brokers     []string
	GroupID     string
	Topic       string
	DLQ         string
	MaxRetries  int
	BaseBackoff time.Duration
}

type Consumer struct {
	reader *kafka.Reader
	dlq    *kafka.Writer
	svc    service.Order
}

func NewConsumer(cfg Config, svc service.Order) *Consumer {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        cfg.Brokers,
		GroupID:        cfg.GroupID,
		Topic:          cfg.Topic,
		MinBytes:       1,
		MaxBytes:       10e6,
		MaxWait:        100 * time.Millisecond,
		CommitInterval: 0,
	})
	w := &kafka.Writer{
		Addr:                   kafka.TCP(cfg.Brokers...),
		Topic:                  cfg.DLQ,
		RequiredAcks:           kafka.RequireAll,
		Balancer:               &kafka.LeastBytes{},
		AllowAutoTopicCreation: true,
	}
	return &Consumer{reader: r, dlq: w, svc: svc}
}

func (c *Consumer) Subscribe(ctx context.Context) error {
	for {
		m, err := c.reader.FetchMessage(ctx)
		log.Printf("[cons] fetched topic=%s part=%d off=%d key=%q", m.Topic, m.Partition, m.Offset, string(m.Key))

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

		ok := false
		var last error
		for attempt := 0; attempt <= c.cfg().MaxRetries; attempt++ {
			if e := c.svc.HandleMessage(ctx, m.Value); e == nil {
				ok = true
				break
			} else if isNonRetryable(e) {
				last = e
				break
			} else {
				last = e
				time.Sleep(backoff(attempt, c.cfg().BaseBackoff))
			}
		}

		if ok {
			if err := c.reader.CommitMessages(ctx, m); err != nil {
				log.Printf("commit failed (offset %d, partition %d): %v", m.Offset, m.Partition, err)
			}
			continue
		}

		dlqMsg := kafka.Message{
			Key:   m.Key,
			Value: m.Value,
			Headers: append(m.Headers,
				kafka.Header{Key: "x-dlq-reason", Value: []byte(trimErr(last))},
				kafka.Header{Key: "x-dlq-attempts", Value: []byte(strconv.Itoa(c.cfg().MaxRetries + 1))},
				kafka.Header{Key: "x-dlq-ts", Value: []byte(time.Now().UTC().Format(time.RFC3339))},
				kafka.Header{Key: "x-dlq-source-topic", Value: []byte(c.reader.Config().Topic)},
				kafka.Header{Key: "x-dlq-group", Value: []byte(c.reader.Config().GroupID)},
			),
		}
		if err := c.dlq.WriteMessages(ctx, dlqMsg); err != nil {

			log.Printf("write to DLQ failed: %v", err)
			time.Sleep(500 * time.Millisecond)
		}

		if err := c.reader.CommitMessages(ctx, m); err != nil {
			log.Printf("commit after DLQ failed (offset %d, partition %d): %v", m.Offset, m.Partition, err)
		}
	}
}

func (c *Consumer) Close(ctx context.Context) error {
	_ = c.dlq.Close()
	return c.reader.Close()
}

func (c *Consumer) cfg() Config {
	return Config{MaxRetries: 5, BaseBackoff: 200 * time.Millisecond}
}

func backoff(n int, base time.Duration) time.Duration {
	if n == 0 {
		return 0
	}
	d := base * (1 << (n - 1))
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}

func trimErr(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	if len(s) > 1000 {
		return s[:1000]
	}
	return s
}

func isNonRetryable(err error) bool {
	return errors.Is(err, service.ErrDecode) || errors.Is(err, service.ErrValidation)
}
