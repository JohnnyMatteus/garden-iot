// Comando api: REST e SSE do garden-iot (ADR 0012).
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"

	"github.com/JohnnyMatteus/garden-iot/services/api/application"
	"github.com/JohnnyMatteus/garden-iot/services/api/domain"
	"github.com/JohnnyMatteus/garden-iot/services/api/infrastructure/auth"
	"github.com/JohnnyMatteus/garden-iot/services/api/infrastructure/mqtt"
	"github.com/JohnnyMatteus/garden-iot/services/api/infrastructure/postgres"
	"github.com/JohnnyMatteus/garden-iot/services/api/interfaces/httpapi"
)

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

type uuidGenerator struct{}

func (uuidGenerator) NewID() string { return uuid.NewString() }

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func mustEnv(key string) (string, error) {
	if v := os.Getenv(key); v != "" {
		return v, nil
	}
	return "", fmt.Errorf("variável %s é obrigatória", key)
}

func mtlsConfig(dir string) (*tls.Config, error) {
	ca, err := os.ReadFile(dir + "/ca.pem")
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(ca)
	pair, err := tls.LoadX509KeyPair(dir+"/api.pem", dir+"/api.key")
	if err != nil {
		return nil, err
	}
	return &tls.Config{RootCAs: pool, Certificates: []tls.Certificate{pair}, ServerName: env("API_MQTT_SERVER_NAME", "localhost"), MinVersion: tls.VersionTLS12}, nil
}

func run(ctx context.Context, log *slog.Logger) error {
	dsn, err := mustEnv("API_DATABASE_URL")
	if err != nil {
		return err
	}
	secret, err := mustEnv("API_JWT_SECRET")
	if err != nil {
		return err
	}
	verifier, err := auth.NewJWT(secret, env("API_JWT_ISSUER", "garden-api"))
	if err != nil {
		return err
	}
	db, err := postgres.Connect(ctx, dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	svc := &application.Service{
		Catalog: db, Readings: db, Twin: db, Alerts: db, Commands: db,
		Policy: domain.DefaultIrrigationPolicy(), Clock: systemClock{}, IDs: uuidGenerator{},
		CommandTTL: 30 * time.Second, ConditionsMaxAge: 30 * time.Minute,
	}

	tlsCfg, err := mtlsConfig(env("API_PKI_DIR", "infra/pki/out"))
	if err != nil {
		return fmt.Errorf("certificado MQTT da API (rode make up): %w", err)
	}
	bus, err := mqtt.Connect(ctx, mqtt.Config{BrokerURL: env("API_MQTT_URL", "tls://localhost:8883"), TLS: tlsCfg, ClientID: "svc-api"}, svc, log)
	if err != nil {
		return err
	}
	defer func() {
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = bus.Close(shutdown)
	}()
	svc.Publisher = bus

	go func() {
		t := time.NewTicker(5 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if n, err := svc.ExpireCommands(ctx); err != nil {
					log.Warn("expiração de comandos falhou", "err", err)
				} else if n > 0 {
					log.Info("comandos expirados", "count", n)
				}
			}
		}
	}()

	srv := &http.Server{
		Addr: env("API_ADDR", ":8090"), ReadHeaderTimeout: 5 * time.Second,
		Handler: (&httpapi.Server{Svc: svc, Verifier: verifier, Log: log, SSEPeriod: 2 * time.Second}).Handler(),
	}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	log.Info("api no ar", "addr", srv.Addr)

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdown)
	}
}

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, log); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("encerrado com erro", "err", err)
		os.Exit(1)
	}
}
