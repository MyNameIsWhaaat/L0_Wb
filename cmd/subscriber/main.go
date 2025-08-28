package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"

	"l0-demo/internal/configs"
	httpdelivery "l0-demo/internal/delivery/http"
	"l0-demo/internal/delivery/kafka"
	"l0-demo/internal/repository"
	"l0-demo/internal/repository/postgres"
	"l0-demo/internal/service"
)

// @title kafka learning service
// @version 1.0
// @description This service uses a nats streaming server as message broker to get model Order from it and stores into the postgres db & app's cache. Provides a way to get information about orders from cache via the HTTP requests.

// @host localhost:8081
// @basePath /

// @contact.name Ekaterina Perminova
// @contact.email katanatroll@yandex.ru

func main() {
	_ = godotenv.Load()
	cfg, err := configs.LoadConfig(".")
	if err != nil {
		logrus.Fatalf("config load: %s", err)
	}
	logrus.Print("config parsed")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := postgres.ConnectDB(postgres.Config{
		Host:     cfg.PostgresHost,
		Port:     cfg.PostgresPort,
		Username: cfg.PostgresUser,
		Password: cfg.PostgresPass,
		DbName:   cfg.PostgresDB,
		SslMode:  cfg.PostgresSSLMode,
	})
	if err != nil {
		logrus.Fatalf("postgres connect: %s", err)
	}
	defer func() {
		if derr := db.Close(); derr != nil {
			logrus.Errorf("db close: %v", derr)
		}
	}()
	logrus.Print("connected to postgres")

	repo := repository.NewRepository(db)
	svc := service.NewService(repo)

	if err := svc.PutOrdersFromDbToCache(); err != nil {
		logrus.Fatalf("warm cache: %s", err)
	}
	logrus.Print("cache warmed from db")

	consumer := kafka.NewConsumer(kafka.Config{
		Brokers: []string{cfg.KafkaBrokers},
		GroupID: cfg.KafkaGroupID,
		Topic:   cfg.KafkaTopic,
	}, svc)
	
	defer func() {
		if cerr := consumer.Close(context.Background()); cerr != nil {
			logrus.Errorf("kafka close: %v", cerr)
		}
	}()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := consumer.Subscribe(ctx); err != nil {
			logrus.Errorf("consumer stopped: %v", err)
			cancel()
		}
	}()
	logrus.Print("kafka subscription started")

	h := httpdelivery.NewHandler(svc)
	srv := new(httpdelivery.Server)

	go func() {
		if err := srv.Run(cfg.HTTPAddr, h.InitRoutes()); err != nil {
			logrus.Errorf("http run: %v", err)
			cancel()
		}
	}()
	logrus.Printf("http server started on %s", cfg.HTTPAddr)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	
	select {
	case <-quit:
		logrus.Print("shutdown signal received")
	case <-ctx.Done():
		logrus.Print("context canceled, shutting down")
	}

	if err := srv.Shutdown(context.Background()); err != nil {
		logrus.Errorf("http shutdown: %s", err)
	}

	if err := consumer.Close(context.Background()); err != nil {
		logrus.Errorf("consumer close: %s", err)
	}

	wg.Wait()
	logrus.Print("service stopped")
}
