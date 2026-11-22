// Comando bridge: MQTT 5 (EMQX) → Kafka, ver ADR 0001.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/twmb/franz-go/pkg/sr"
	"golang.org/x/sync/errgroup"

	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/application"
	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/infrastructure/config"
	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/infrastructure/dedup"
	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/infrastructure/envelope"
	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/infrastructure/kafka"
	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/infrastructure/metrics"
	bridgemqtt "github.com/JohnnyMatteus/garden-iot/services/bridge/internal/infrastructure/mqtt"
)

func main() {
	if err := run(); err != nil {
		slog.Error("bridge encerrou com erro", "erro", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	level := slog.LevelInfo
	_ = level.UnmarshalText([]byte(cfg.LogLevel))
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	tlsCfg, err := cfg.TLSConfig()
	if err != nil {
		return err
	}

	reg, err := sr.NewClient(sr.URLs(cfg.SchemaRegistry))
	if err != nil {
		return err
	}
	encoder, err := kafka.NewProtoEncoder(ctx, reg, cfg.Topic+"-value", kafka.BrutaSample(), kafka.ToBruta)
	if err != nil {
		return err
	}
	kc, err := kafka.NewClient(cfg.KafkaBrokers)
	if err != nil {
		return err
	}
	defer kc.Close()
	producer := kafka.NewProducer(kc, encoder, cfg.Topic, cfg.DLQTopic)

	var dd application.Deduplicator = dedup.NewMemory(cfg.DedupTTL)
	if cfg.RedisAddr != "" {
		dd = dedup.NewRedis(redis.NewClient(&redis.Options{Addr: cfg.RedisAddr}), cfg.DedupTTL)
	}

	prom := metrics.NewPrometheus()
	handler := application.NewHandler(envelope.New(), producer, producer, dd, prom, log)
	sub := bridgemqtt.NewSubscriber(bridgemqtt.Config{
		BrokerURL: cfg.MQTTURL, ClientID: cfg.MQTTClientID, TLS: tlsCfg, ShareGroup: cfg.MQTTShareGroup,
		TopicFilter: cfg.MQTTFilter, SessionExpiry: cfg.MQTTSession, MaxInFlight: cfg.MaxInFlight,
	}, handler, log)

	mux := http.NewServeMux()
	mux.Handle("/metrics", prom.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error { return sub.Run(gctx) })
	g.Go(func() error {
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})
	g.Go(func() error {
		<-gctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdown)
	})
	return g.Wait()
}
