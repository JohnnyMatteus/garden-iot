// Comando persist: telemetria.bruta.v1 → TimescaleDB (M1-10), via ACL do payload de dispositivo.
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

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"

	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/acl/devicepayload"
	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/infrastructure/config"
	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/infrastructure/consumer"
	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/infrastructure/kafka"
	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/infrastructure/metrics"
	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/infrastructure/normalizer"
	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/infrastructure/timescale"
	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/readings"
)

func main() {
	if err := run(); err != nil {
		slog.Error("persist encerrou com erro", "erro", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.LoadPersist()
	if err != nil {
		return err
	}
	level := slog.LevelInfo
	_ = level.UnmarshalText([]byte(cfg.LogLevel))
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	cons, err := consumer.NewClient(cfg.KafkaBrokers, cfg.Group, cfg.Topic)
	if err != nil {
		return err
	}
	defer cons.Close()
	dlqClient, err := kafka.NewClient(cfg.KafkaBrokers)
	if err != nil {
		return err
	}
	defer dlqClient.Close()
	dlq := kafka.NewProducer(dlqClient, nil, "", cfg.DLQTopic)

	prom := metrics.NewPersist()
	acl := normalizer.New(devicepayload.Calibration{SoilDry: cfg.SoilDry, SoilSaturated: cfg.SoilSaturated})
	ingestor := readings.NewIngestor(acl, timescale.NewStore(pool), dlq, prom, log)
	loop := consumer.New(cons, ingestor, dlq, cfg.BatchSize, log)

	mux := http.NewServeMux()
	mux.Handle("/metrics", prom.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error { return loop.Run(gctx) })
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
