package main

import (
	"context"
	"io"
	"os"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"

	"l0-demo/internal/configs"
	"l0-demo/internal/delivery/kafka"
)

func main() {
	
	if err := godotenv.Load(); err != nil {
    	logrus.Fatalf("failed to load .env: %s", err)
	}

	cfg, err := configs.LoadConfig(".")
	if err != nil {
		logrus.Fatalf("error loading config: %s", err)
	}
	logrus.Print("config loaded")

	pub, err := kafka.NewPublisher(cfg.KafkaBrokers, cfg.KafkaTopic)
	if err != nil {
		logrus.Fatalf("kafka publisher connect error: %s", err)
	}
	defer func() {
		if cerr := pub.Close(); cerr != nil {
			logrus.Errorf("publisher close: %v", cerr)
		}
	}()
	logrus.Print("connected to kafka")

	f, err := os.Open(cfg.JsonStaticModelPath)
	if err != nil {
		logrus.Fatalf("open json file: %s", err)
	}
	defer f.Close()

	body, err := io.ReadAll(f)
	if err != nil {
		logrus.Fatalf("read json file: %s", err)
	}

	if err := pub.Publish(context.Background(), body); err != nil {
		logrus.Fatalf("publish failed: %s", err)
	}
	logrus.Print("successfully published static order JSON to kafka")
}
