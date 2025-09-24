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
		BatchTimeout:           10 * time.Millisecond,
	}

	if cfg.MaxRetries < 0 {
		cfg.MaxRetries = 0
	}
	if cfg.BaseBackoff <= 0 {
		cfg.BaseBackoff = 200 * time.Millisecond
	}

	return &Consumer{reader: r, dlq: w, svc: svc}
}

func (c *Consumer) Subscribe(ctx context.Context) error {
	    for {
        select {
        case <-ctx.Done():
            return nil
        default:
        }
	
		m, err := c.reader.FetchMessage(ctx)
        if err != nil {
            if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
                return nil
            }
            log.Printf("kafka fetch error: %v", err)
            select {
            case <-time.After(300 * time.Millisecond):
                continue
            case <-ctx.Done():
                return nil
            }
        }

        log.Printf("[cons] fetched topic=%s part=%d off=%d key=%q", m.Topic, m.Partition, m.Offset, string(m.Key))


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
		if c.dlq != nil {
            if ctx.Err() != nil {
                return nil
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
                if ctx.Err() != nil {
                    return nil
                }

                log.Printf("write to DLQ failed (offset %d, partition %d): %v", m.Offset, m.Partition, err)

                time.Sleep(500 * time.Millisecond)
                continue
            }
        } else {
            log.Printf("DLQ disabled, drop message (offset %d, partition %d): %v", m.Offset, m.Partition, last)
        }

        if err := c.reader.CommitMessages(ctx, m); err != nil {
            if ctx.Err() != nil { return nil }
            log.Printf("commit after DLQ (offset %d, partition %d) failed: %v", m.Offset, m.Partition, err)
        }
    }
	
}

func (c *Consumer) Close() error {
    var first error
    if c.reader != nil {
        if err := c.reader.Close(); err != nil {
            first = err
        }
    }
    if c.dlq != nil {
        if err := c.dlq.Close(); err != nil && first == nil {
            first = err
        }
    }
    return first
}

func (c *Consumer) cfg() Config {
	return Config{MaxRetries: 5, BaseBackoff: 200 * time.Millisecond}
}

func backoff(n int, base time.Duration) time.Duration {
    if n <= 0 { return 0 }
    d := base * (1 << (n - 1))
    if d > 5*time.Second {
        d = 5 * time.Second
    }
    return d
}

func trimErr(err error) string {
    if err == nil { return "" }
    s := err.Error()
    if len(s) > 1000 { return s[:1000] }
    return s
}

func isNonRetryable(err error) bool {
	return errors.Is(err, service.ErrDecode) || errors.Is(err, service.ErrValidation)
}
