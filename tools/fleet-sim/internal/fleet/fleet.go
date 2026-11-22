// Package fleet orquestra os nós simulados: tick, publicação e injeção de falhas.
package fleet

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/JohnnyMatteus/garden-iot/tools/fleet-sim/internal/generator"
	"github.com/JohnnyMatteus/garden-iot/tools/fleet-sim/internal/profile"
)

// Link é a conexão MQTT de um nó.
type Link interface {
	Publish(ctx context.Context, topic string, payload []byte) error
	// Kill derruba a conexão sem DISCONNECT, para o broker publicar o Last Will.
	Kill()
	Close(ctx context.Context)
}

// Dialer abre a conexão de um nó registrando o Last Will.
type Dialer interface {
	Dial(ctx context.Context, clientID, willTopic string, willPayload []byte) (Link, error)
}

type NodeSpec struct {
	Tenant, Garden, Node string
	Profile              profile.Profile
}

func (n NodeSpec) Topic(suffix string) string {
	return fmt.Sprintf("horta/%s/%s/%s/%s", n.Tenant, n.Garden, n.Node, suffix)
}

type Config struct {
	Seed          uint64
	Interval      time.Duration // intervalo virtual entre leituras (vira o campo ts)
	Speed         float64       // 1 = tempo real; N = N vezes mais rápido
	Ticks         int
	Start         time.Time
	DuplicateRate float64       // probabilidade de republicar um lote já enviado
	ReorderRate   float64       // probabilidade de segurar um lote e enviá-lo depois do seguinte
	RetryFor      time.Duration // por quanto tempo repetir uma publicação que falha (broker fora)
}

type Fleet struct {
	cfg    Config
	dialer Dialer
	log    *slog.Logger
}

func New(cfg Config, d Dialer, log *slog.Logger) *Fleet {
	if cfg.Speed <= 0 {
		cfg.Speed = 1
	}
	return &Fleet{cfg: cfg, dialer: d, log: log}
}

// Run executa todos os nós em paralelo até o fim dos ticks ou do contexto.
func (f *Fleet) Run(ctx context.Context, nodes []NodeSpec) error {
	var wg sync.WaitGroup
	errs := make(chan error, len(nodes))
	for i, n := range nodes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := f.runNode(ctx, i, n); err != nil {
				errs <- fmt.Errorf("nó %s: %w", n.Node, err)
			}
		}()
	}
	wg.Wait()
	close(errs)
	return <-errs
}

func (f *Fleet) runNode(ctx context.Context, index int, n NodeSpec) error {
	clientID := fmt.Sprintf("sim-%s-%s-%s", n.Tenant, n.Garden, n.Node)
	dial := func() (Link, error) {
		return f.dialer.Dial(ctx, clientID, n.Topic("lwt"), []byte(`{"online":false}`))
	}
	link, err := dial()
	if err != nil {
		return err
	}
	defer func() { link.Close(context.WithoutCancel(ctx)) }()

	gen := generator.NewNode(n.Profile, f.cfg.Seed, index, f.cfg.Start, f.cfg.Interval)
	inj := rand.New(rand.NewPCG(f.cfg.Seed^0x9e3779b97f4a7c15, uint64(index)+1))
	wait := time.Duration(float64(f.cfg.Interval) / f.cfg.Speed)

	var held *generator.Batch
	downUntil := -1
	for tick := 0; tick < f.cfg.Ticks; tick++ {
		if tick > 0 {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(wait):
			}
		}

		if d := n.Profile.Drop; d != nil && tick == d.AfterTicks {
			f.log.Info("nó caiu sem aviso", "no", n.Node, "tick", tick)
			link.Kill()
			downUntil = tick + d.DownTicks
		}
		if downUntil >= 0 && tick >= downUntil {
			if link, err = dial(); err != nil {
				return err
			}
			downUntil = -1
		}
		if downUntil >= 0 { // fora do ar: a leitura é gerada (sequência avança) mas não sai
			gen.Batch(tick, 0)
			continue
		}

		b := gen.Batch(tick, 0)
		if held == nil && inj.Float64() < f.cfg.ReorderRate && tick < f.cfg.Ticks-1 {
			held = &b
			continue
		}
		if err := f.send(ctx, link, n, b, inj); err != nil {
			return err
		}
		if held != nil {
			if err := f.send(ctx, link, n, *held, inj); err != nil {
				return err
			}
			held = nil
		}
	}
	if held != nil {
		return f.send(ctx, link, n, *held, inj)
	}
	return nil
}

func (f *Fleet) send(ctx context.Context, link Link, n NodeSpec, b generator.Batch, inj *rand.Rand) error {
	if err := f.publish(ctx, link, n.Topic("telemetria"), b.Payload); err != nil {
		return err
	}
	if inj.Float64() < f.cfg.DuplicateRate {
		return f.publish(ctx, link, n.Topic("telemetria"), b.Payload)
	}
	return nil
}

// publish repete enquanto o broker estiver fora (a conexão se refaz sozinha), como o buffer
// offline do firmware: o lote só é dado como enviado quando o broker confirma.
func (f *Fleet) publish(ctx context.Context, link Link, topic string, payload []byte) error {
	deadline := time.Now().Add(f.cfg.RetryFor)
	for {
		err := link.Publish(ctx, topic, payload)
		if err == nil || ctx.Err() != nil || time.Now().After(deadline) {
			return err
		}
		f.log.Warn("publicação falhou, nova tentativa", "topico", topic, "erro", err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}
