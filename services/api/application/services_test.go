package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JohnnyMatteus/garden-iot/services/api/application"
	"github.com/JohnnyMatteus/garden-iot/services/api/domain"
)

var t0 = time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)

type fakeClock struct{}

func (fakeClock) Now() time.Time { return t0 }

type fakeIDs struct{}

func (fakeIDs) NewID() string { return "cmd-1" }

type fakeCatalog struct {
	application.Catalog
	node domain.Node
	bed  domain.Bed
}

func (c fakeCatalog) GetNode(_ context.Context, _, g, n string) (domain.Node, error) {
	if n != c.node.ID {
		return domain.Node{}, domain.ErrNotFound
	}
	return c.node, nil
}
func (c fakeCatalog) GetBed(context.Context, string, string, string) (domain.Bed, error) { return c.bed, nil }

type fakeReadings struct {
	application.ReadingStore
	latest map[string]float64
}

func (r fakeReadings) Latest(_ context.Context, _, _, kind string, _ time.Time) (*domain.Reading, error) {
	if v, ok := r.latest[kind]; ok {
		return &domain.Reading{Kind: kind, Value: v}, nil
	}
	return nil, nil
}

type fakeTwin struct {
	application.TwinStore
	desired *domain.ActuatorState
}

func (f *fakeTwin) SetDesired(_ context.Context, _, _, _ string, s domain.ActuatorState) error {
	f.desired = &s
	return nil
}

type fakeCommands struct {
	application.CommandStore
	open     bool
	created  []domain.Command
	statuses map[string]domain.CommandStatus
}

func (f *fakeCommands) HasOpen(context.Context, string, string, string) (bool, error) { return f.open, nil }
func (f *fakeCommands) Create(_ context.Context, _ string, c domain.Command) error {
	f.created = append(f.created, c)
	return nil
}
func (f *fakeCommands) SetStatus(_ context.Context, _, id string, s domain.CommandStatus) error {
	f.statuses[id] = s
	return nil
}

type fakePublisher struct {
	err       error
	published []domain.Command
}

func (f *fakePublisher) Publish(_ context.Context, _ string, c domain.Command) error {
	if f.err != nil {
		return f.err
	}
	f.published = append(f.published, c)
	return nil
}

type env struct {
	svc  *application.Service
	cmds *fakeCommands
	twin *fakeTwin
	pub  *fakePublisher
}

func newEnv(latest map[string]float64) env {
	bed := "canteiro-1"
	e := env{cmds: &fakeCommands{statuses: map[string]domain.CommandStatus{}}, twin: &fakeTwin{}, pub: &fakePublisher{}}
	e.svc = &application.Service{
		Catalog:  fakeCatalog{node: domain.Node{ID: "n3", GardenID: "h1", Kind: domain.ActuatorNode, BedID: &bed}, bed: domain.Bed{ID: bed, Irrigation: domain.Sprinkler}},
		Readings: fakeReadings{latest: latest}, Twin: e.twin, Commands: e.cmds, Publisher: e.pub,
		Policy: domain.DefaultIrrigationPolicy(), Clock: fakeClock{}, IDs: fakeIDs{},
		CommandTTL: 30 * time.Second, ConditionsMaxAge: 30 * time.Minute,
	}
	return e
}

var operator = domain.Principal{Subject: "ana", TenantID: "demo", Role: domain.RoleOperator}

func TestRequestIrrigationPublishesCommandWithExpiry(t *testing.T) {
	e := newEnv(nil)
	cmd, err := e.svc.RequestIrrigation(context.Background(), operator, "h1", "n3", 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Status != domain.CommandSent || cmd.DurationSeconds != 300 || !cmd.ExpiresAt.Equal(t0.Add(30*time.Second)) {
		t.Fatalf("comando inesperado: %+v", cmd)
	}
	if len(e.pub.published) != 1 || e.twin.desired == nil || e.twin.desired.Valve != domain.ValveOpen {
		t.Fatal("deveria publicar e gravar o estado desejado")
	}
	if e.cmds.statuses["cmd-1"] != domain.CommandSent {
		t.Fatalf("status = %s", e.cmds.statuses["cmd-1"])
	}
}

func TestRequestIrrigationBlockedByPolicyDoesNotPublish(t *testing.T) {
	e := newEnv(map[string]float64{"air_temp_c": 1, "wind_ms": 8})
	_, err := e.svc.RequestIrrigation(context.Background(), operator, "h1", "n3", time.Minute)
	var pe *domain.PolicyError
	if !errors.As(err, &pe) || len(pe.Violations) != 2 {
		t.Fatalf("esperava 2 violações (geada e vento), veio %v", err)
	}
	if len(e.pub.published) != 0 || len(e.cmds.created) != 0 || e.twin.desired != nil {
		t.Fatal("nada deve ser gravado nem publicado quando bloqueado")
	}
}

func TestRequestIrrigationViewerForbidden(t *testing.T) {
	e := newEnv(nil)
	v := operator
	v.Role = domain.RoleViewer
	if _, err := e.svc.RequestIrrigation(context.Background(), v, "h1", "n3", time.Minute); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("erro = %v", err)
	}
}

func TestRequestIrrigationPublishFailureMarksCommandFailed(t *testing.T) {
	e := newEnv(nil)
	e.pub.err = errors.New("sem conexão")
	_, err := e.svc.RequestIrrigation(context.Background(), operator, "h1", "n3", time.Minute)
	if !errors.Is(err, application.ErrPublish) {
		t.Fatalf("erro = %v", err)
	}
	if e.cmds.statuses["cmd-1"] != domain.CommandFailed {
		t.Fatalf("status = %s", e.cmds.statuses["cmd-1"])
	}
}

func TestRequestIrrigationOpenCommandBlocks(t *testing.T) {
	e := newEnv(nil)
	e.cmds.open = true
	if _, err := e.svc.RequestIrrigation(context.Background(), operator, "h1", "n3", time.Minute); err == nil {
		t.Fatal("esperava bloqueio por comando em aberto")
	}
}

func TestOnlyOwnerCreatesCatalog(t *testing.T) {
	e := newEnv(nil)
	if _, err := e.svc.CreateGarden(context.Background(), operator, domain.Garden{ID: "x", Name: "x"}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("erro = %v", err)
	}
	owner := operator
	owner.Role = domain.RoleOwner
	if _, err := e.svc.CreateGarden(context.Background(), owner, domain.Garden{ID: "BAD ID", Name: "x"}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("erro = %v", err)
	}
}

func TestHistoryRejectsInvalidRange(t *testing.T) {
	e := newEnv(nil)
	_, err := e.svc.History(context.Background(), operator, application.ReadingQuery{From: t0, To: t0.Add(-time.Hour)})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("erro = %v", err)
	}
	_, err = e.svc.History(context.Background(), operator, application.ReadingQuery{Limit: 6000})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("erro = %v", err)
	}
}

func TestApplyCommandResponse(t *testing.T) {
	e := newEnv(nil)
	_ = e.svc.ApplyCommandResponse(context.Background(), "demo", "a", true)
	_ = e.svc.ApplyCommandResponse(context.Background(), "demo", "b", false)
	if e.cmds.statuses["a"] != domain.CommandConfirmed || e.cmds.statuses["b"] != domain.CommandRejected {
		t.Fatalf("statuses = %v", e.cmds.statuses)
	}
}
