package configs

import (
	"fmt"
	"strings"

	"github.com/caarlos0/env/v9"
)

type Config struct {
	KafkaBrokers string `env:"KAFKA_BROKERS" envDefault:"localhost:9092"`
	KafkaTopic   string `env:"KAFKA_TOPIC" envDefault:"orders"`
	KafkaGroupID string `env:"KAFKA_GROUP_ID" envDefault:"order-svc"`

	HTTPAddr string `env:"HTTP_ADDR" envDefault:":8081"`

	CacheWarmLimit int `env:"CACHE_WARM_LIMIT" envDefault:"100"`

	JsonStaticModelPath string `env:"JSON_STATIC_MODEL_PATH" envDefault:"web/model.json"`

	DatabaseURL     string `env:"DATABASE_URL" envDefault:""`
	PostgresHost    string `env:"POSTGRES_HOST" envDefault:"localhost"`
	PostgresPort    string `env:"POSTGRES_PORT" envDefault:"5432"`
	PostgresUser    string `env:"POSTGRES_USER" envDefault:"postgres"`
	PostgresPass    string `env:"POSTGRES_PASSWORD" envDefault:"postgres"`
	PostgresDB      string `env:"POSTGRES_DB" envDefault:"orders"`
	PostgresSSLMode string `env:"POSTGRES_SSLMODE" envDefault:"disable"`
}

func LoadConfig(_ string) (Config, error) {
    var c Config
    if err := env.Parse(&c); err != nil {
        return Config{}, fmt.Errorf("config parse: %w", err)
    }
    return c, nil
}

func (c Config) KafkaBrokersSlice() []string {
	parts := strings.Split(c.KafkaBrokers, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (c Config) PgDSN() string {
	if c.DatabaseURL != "" {
		return c.DatabaseURL
	}
	pass := c.PostgresPass

	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		c.PostgresUser,
		pass,
		c.PostgresHost,
		c.PostgresPort,
		c.PostgresDB,
		c.PostgresSSLMode,
	)
}