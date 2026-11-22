// Comando irrigation: decide e executa regas (balanço FAO-56 + regras de segurança + saga).
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

	"github.com/JohnnyMatteus/garden-iot/services/irrigation/application"
	"github.com/JohnnyMatteus/garden-iot/services/irrigation/domain/safety"
	"github.com/JohnnyMatteus/garden-iot/services/irrigation/infrastructure/config"
	irrmqtt "github.com/JohnnyMatteus/garden-iot/services/irrigation/infrastructure/mqtt"
	"github.com/JohnnyMatteus/garden-iot/services/irrigation/infrastructure/openmeteo"
	"github.com/JohnnyMatteus/garden-iot/services/irrigation/infrastructure/sinks"
	"github.com/JohnnyMatteus/garden-iot/services/irrigation/infrastructure/timescale"
)

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }
func (realClock) Sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func main() {
	if err := run(); err != nil {
		slog.Error("irrigation encerrou com erro", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	var lvl slog.LevelVar
	_ = lvl.UnmarshalText([]byte(cfg.LogLevel))
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: &lvl}))

	beds, err := config.LoadBeds(cfg.BedsFile)
	if err != nil {
		return err
	}
	tlsCfg, err := cfg.TLSConfig()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	bus, err := irrmqtt.Connect(ctx, irrmqtt.Config{BrokerURL: cfg.MQTTURL, TLS: tlsCfg, ClientID: cfg.MQTTClientID}, log)
	if err != nil {
		return err
	}
	defer func() {
		dctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = bus.Disconnect(dctx)
	}()

	reader, err := timescale.NewReader(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer reader.Close()

	var weather application.WeatherProvider
	if !cfg.WeatherOff {
		weather = openmeteo.NewClient()
	}
	sink := sinks.Log{L: log}
	limits := safety.DefaultLimits()
	sagaCfg := application.DefaultSagaConfig()
	sagaCfg.Limits = limits
	clock := realClock{}

	sched := &application.Scheduler{
		Planner: &application.Planner{Weather: weather, Alerts: sink, Limits: limits, Horizon: 12 * time.Hour,
			MaxDuration: cfg.MaxDuration, Log: log},
		Saga:    &application.Saga{Actuator: bus, Twin: bus, Alerts: sink, Journal: sink, Clock: clock, Cfg: sagaCfg},
		Sensors: reader, Climate: reader, Beds: beds, Clock: clock, Log: log, DefaultWindMS: cfg.DefaultWind,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("servidor http", "err", err)
		}
	}()
	defer func() {
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(sctx)
	}()

	log.Info("irrigação iniciada", "beds", len(beds), "interval", cfg.Interval)
	sched.Run(ctx, cfg.Interval)
	return nil
}
