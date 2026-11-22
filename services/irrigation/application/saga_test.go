package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JohnnyMatteus/garden-iot/services/irrigation/application"
)

var ref = application.NodeRef{Tenant: "demo", Garden: "h1", Node: "n1"}

type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time { return c.now }
func (c *fakeClock) Sleep(ctx context.Context, d time.Duration) error {
	c.now = c.now.Add(d)
	return ctx.Err()
}

// fakeNode simula o nó: reage a Open/Close e reporta vazão, com falhas injetáveis.
type fakeNode struct {
	clock     *fakeClock
	valve     application.Valve
	flow      *float64
	silent    bool // não atualiza o estado reportado (nó mudo)
	openErr   error
	closeErrs int // quantos Close() falham antes de funcionar
	stuckOpen bool
	flowAfter func(open time.Duration) float64
	openedAt  time.Time
	at        time.Time
	desired   []application.Valve
	calls     []string
}

func (n *fakeNode) report() {
	if n.silent {
		return
	}
	n.at = n.clock.now
}

func (n *fakeNode) Open(context.Context, application.NodeRef, time.Duration) error {
	n.calls = append(n.calls, "open")
	if n.openErr != nil {
		return n.openErr
	}
	n.valve, n.openedAt = application.ValveOpen, n.clock.now
	n.report()
	return nil
}

func (n *fakeNode) Close(context.Context, application.NodeRef) error {
	n.calls = append(n.calls, "close")
	if n.closeErrs > 0 {
		n.closeErrs--
		return errors.New("timeout sem resposta")
	}
	if !n.stuckOpen {
		n.valve = application.ValveClosed
	}
	n.report()
	return nil
}

func (n *fakeNode) SetDesired(_ context.Context, _ application.NodeRef, v application.Valve) error {
	n.desired = append(n.desired, v)
	return nil
}

func (n *fakeNode) Reported(context.Context, application.NodeRef) (application.ActuatorState, bool) {
	if n.at.IsZero() {
		return application.ActuatorState{}, false
	}
	n.report() // o nó publica estado periodicamente
	st := application.ActuatorState{Valve: n.valve, At: n.at}
	if n.valve == application.ValveOpen {
		f := 2.0
		if n.flow != nil {
			f = *n.flow
		}
		if n.flowAfter != nil {
			f = n.flowAfter(n.clock.now.Sub(n.openedAt))
		}
		st.FlowLpm = &f
	}
	return st, true
}

type sinks struct {
	alerts  []application.Alert
	records []application.Record
}

func (s *sinks) Raise(_ context.Context, a application.Alert) error {
	s.alerts = append(s.alerts, a)
	return nil
}
func (s *sinks) Record(_ context.Context, r application.Record) error {
	s.records = append(s.records, r)
	return nil
}

func setup() (*application.Saga, *fakeNode, *sinks) {
	clk := &fakeClock{now: time.Date(2026, 11, 22, 6, 0, 0, 0, time.UTC)}
	node := &fakeNode{clock: clk}
	sk := &sinks{}
	return &application.Saga{Actuator: node, Twin: node, Alerts: sk, Journal: sk, Clock: clk, Cfg: application.DefaultSagaConfig()}, node, sk
}

var req = application.Request{Ref: ref, BedID: "b1", Duration: 60 * time.Second, ExpectedLpm: 2}

func TestHappyPathClosesByTime(t *testing.T) {
	s, node, sk := setup()
	out, err := s.Run(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, application.StatusCompleted, out.Status)
	assert.Equal(t, application.ValveClosed, node.valve)
	assert.Equal(t, []string{"open", "close"}, node.calls)
	assert.Equal(t, []application.Valve{application.ValveOpen, application.ValveClosed}, node.desired)
	assert.Empty(t, sk.alerts)
	require.Len(t, sk.records, 1)
	// 2 L/min integrados entre o fim da estabilização (5 s) e os 60 s: 55 s ≈ 1,83 L.
	assert.InDelta(t, 2.0*55.0/60.0, sk.records[0].Liters, 0.05)
	assert.GreaterOrEqual(t, out.Elapsed, 60*time.Second)
}

func TestClosesByVolumeBeforeTime(t *testing.T) {
	s, _, sk := setup()
	r := req
	r.Duration = 10 * time.Minute
	r.TargetL = 1.0
	out, err := s.Run(context.Background(), r)
	require.NoError(t, err)
	assert.Equal(t, application.StatusCompleted, out.Status)
	assert.GreaterOrEqual(t, out.Liters, 1.0)
	assert.Less(t, out.Elapsed, 60*time.Second)
	assert.Empty(t, sk.alerts)
}

func TestOpenRefusedCompensatesAndAlerts(t *testing.T) {
	s, node, sk := setup()
	node.openErr = errors.New("recusado pelo intertravamento")
	out, err := s.Run(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, application.StatusCompensated, out.Status)
	assert.Contains(t, node.calls, "close", "fecha tudo mesmo sem ter confirmado a abertura")
	require.Len(t, sk.alerts, 1)
	assert.Equal(t, application.SeverityWarning, sk.alerts[0].Severity)
	assert.Contains(t, sk.alerts[0].Message, "recusado")
	assert.Equal(t, application.StatusCompensated, sk.records[0].Status)
}

func TestLeakDuringIrrigationCompensates(t *testing.T) {
	s, node, sk := setup()
	f := 3.5 // +75% sobre os 2 L/min esperados
	node.flow = &f
	out, err := s.Run(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, application.StatusCompensated, out.Status)
	assert.Equal(t, application.ValveClosed, node.valve)
	assert.Contains(t, out.Reason, "vazamento")
	require.Len(t, sk.alerts, 1)
}

func TestFlowDropsMidwayCompensates(t *testing.T) {
	s, node, sk := setup()
	node.flowAfter = func(open time.Duration) float64 {
		if open > 20*time.Second {
			return 0.4 // entupimento
		}
		return 2
	}
	out, err := s.Run(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, application.StatusCompensated, out.Status)
	assert.Contains(t, out.Reason, "entupimento")
	assert.Greater(t, out.Liters, 0.0, "registra o que já tinha sido aplicado")
	assert.Len(t, sk.alerts, 1)
}

func TestSilentNodeCompensates(t *testing.T) {
	s, node, sk := setup()
	node.silent = true
	out, err := s.Run(context.Background(), req)
	require.Error(t, err, "sem estado do nó não há como confirmar o fechamento")
	assert.Equal(t, application.StatusCompensated, out.Status)
	assert.Contains(t, out.Reason, "não reportou open")
	require.Len(t, sk.alerts, 1)
	assert.Equal(t, application.SeverityCritical, sk.alerts[0].Severity)
}

func TestCloseRetriedThenSucceeds(t *testing.T) {
	s, node, sk := setup()
	node.closeErrs = 2
	out, err := s.Run(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, application.StatusCompleted, out.Status)
	assert.Equal(t, 3, count(node.calls, "close"))
	assert.Empty(t, sk.alerts)
}

func TestValveStuckOpenRaisesCriticalAlert(t *testing.T) {
	s, node, sk := setup()
	node.stuckOpen = true
	out, err := s.Run(context.Background(), req)
	require.Error(t, err, "não conseguir fechar é erro explícito, além do alerta")
	assert.Equal(t, application.StatusCompensated, out.Status)
	require.Len(t, sk.alerts, 1)
	assert.Equal(t, application.SeverityCritical, sk.alerts[0].Severity)
	assert.Len(t, sk.records, 1, "o registro acontece mesmo na falha")
}

func TestCancelledContextStillClosesValve(t *testing.T) {
	s, node, sk := setup()
	ctx, cancel := context.WithCancel(context.Background())
	node.flowAfter = func(time.Duration) float64 { cancel(); return 2 }
	out, err := s.Run(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, application.StatusCompensated, out.Status)
	assert.Equal(t, application.ValveClosed, node.valve)
	assert.Len(t, sk.alerts, 1)
}

func TestRejectsInvalidRequest(t *testing.T) {
	s, node, _ := setup()
	r := req
	r.ExpectedLpm = 0
	out, err := s.Run(context.Background(), r)
	require.NoError(t, err)
	assert.Equal(t, application.StatusCompensated, out.Status)
	assert.NotContains(t, node.calls, "open", "request inválido nunca abre a válvula")
}

func count(s []string, v string) (n int) {
	for _, x := range s {
		if x == v {
			n++
		}
	}
	return
}
