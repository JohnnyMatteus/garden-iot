// Comando fleet-sim: gera telemetria de N nós simulados e publica no EMQX (mTLS).
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/JohnnyMatteus/garden-iot/tools/fleet-sim/internal/fleet"
	"github.com/JohnnyMatteus/garden-iot/tools/fleet-sim/internal/profile"
	"github.com/JohnnyMatteus/garden-iot/tools/fleet-sim/internal/publisher"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fleet-sim:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		broker   = flag.String("broker", "tls://localhost:8883", "URL do EMQX")
		cert     = flag.String("cert", "", "certificado do simulador (CN svc:fleet-sim)")
		key      = flag.String("key", "", "chave do certificado")
		ca       = flag.String("ca", "", "CA que assina o servidor")
		tenant   = flag.String("tenant", "demo", "tenant dos nós simulados")
		garden   = flag.String("garden", "horta-1", "horta dos nós simulados")
		nodes    = flag.Int("nodes", 10, "número de nós")
		profiles = flag.String("profiles", "sunny", "perfis, separados por vírgula, distribuídos em rodízio entre os nós")
		interval = flag.Duration("interval", time.Minute, "intervalo entre leituras (virtual)")
		speed    = flag.Float64("speed", 1, "fator de aceleração do tempo (60 = 1 min vira 1 s)")
		ticks    = flag.Int("ticks", 5, "leituras por nó")
		seed     = flag.Uint64("seed", 1, "semente; a mesma semente gera as mesmas leituras")
		dup      = flag.Float64("duplicates", 0, "probabilidade de republicar um lote (0 a 1)")
		reorder  = flag.Float64("reorder", 0, "probabilidade de enviar um lote depois do seguinte (0 a 1)")
		list     = flag.Bool("list-profiles", false, "lista os perfis e sai")
	)
	flag.Parse()

	all, err := profile.Load()
	if err != nil {
		return err
	}
	if *list {
		for _, n := range profile.Names(all) {
			fmt.Printf("%-14s %s\n", n, all[n].Description)
		}
		return nil
	}

	var chosen []profile.Profile
	for _, name := range strings.Split(*profiles, ",") {
		p, ok := all[strings.TrimSpace(name)]
		if !ok {
			return fmt.Errorf("perfil %q desconhecido; use -list-profiles", name)
		}
		chosen = append(chosen, p)
	}
	specs := make([]fleet.NodeSpec, *nodes)
	for i := range specs {
		specs[i] = fleet.NodeSpec{Tenant: *tenant, Garden: *garden, Node: fmt.Sprintf("sim-%03d", i+1), Profile: chosen[i%len(chosen)]}
	}

	dialer, err := publisher.New(publisher.Config{BrokerURL: *broker, CertFile: *cert, KeyFile: *key, CAFile: *ca})
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// acelerado, o tempo virtual termina "agora"; assim os instantes das leituras nunca ficam no futuro
	start := time.Now().UTC().Truncate(time.Second)
	if *speed > 1 {
		start = start.Add(-time.Duration(*ticks-1) * *interval)
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	f := fleet.New(fleet.Config{
		Seed: *seed, Interval: *interval, Speed: *speed, Ticks: *ticks, Start: start,
		DuplicateRate: *dup, ReorderRate: *reorder,
	}, dialer, log)
	log.Info("simulação iniciada", "nos", *nodes, "ticks", *ticks, "perfis", *profiles)
	return f.Run(ctx, specs)
}
