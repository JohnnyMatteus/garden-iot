package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/JohnnyMatteus/garden-iot/services/api/application"
	"github.com/JohnnyMatteus/garden-iot/services/api/domain"
)

type gardenCatalog struct{ application.Catalog }

func (gardenCatalog) GetGarden(context.Context, string, string) (domain.Garden, error) {
	return domain.Garden{ID: "h1"}, nil
}
func (gardenCatalog) GetBed(context.Context, string, string, string) (domain.Bed, error) {
	return domain.Bed{ID: "c1"}, nil
}
func (gardenCatalog) ListBeds(context.Context, string, string) ([]domain.Bed, error) {
	return []domain.Bed{{ID: "c1", Name: "Alface", Crop: "alface"}, {ID: "c2", Name: "Tomate", Crop: "tomate"}}, nil
}
func (gardenCatalog) ListNodes(context.Context, string, string) ([]domain.Node, error) {
	c1, c2 := "c1", "c2"
	return []domain.Node{
		{ID: "n1", Kind: domain.SensorNode, BedID: &c1}, {ID: "n3", Kind: domain.ActuatorNode, BedID: &c1},
		{ID: "n2", Kind: domain.SensorNode, BedID: &c2},
	}, nil
}

type gardenReadings struct{ application.ReadingStore }

func (gardenReadings) LatestByNode(context.Context, string, string, time.Time) ([]domain.Reading, error) {
	return []domain.Reading{
		{NodeID: "n1", Kind: domain.KindSoilMoisture, Value: 22},
		{NodeID: "n1", Kind: domain.KindAirTemp, Value: 24.5},
	}, nil
}

type gardenTwin struct{ application.TwinStore }

func (gardenTwin) States(context.Context, string, string) ([]domain.NodeState, error) {
	return []domain.NodeState{{
		NodeID:   "n3",
		Desired:  &domain.ActuatorState{Valve: domain.ValveOpen},
		Reported: &domain.ActuatorState{Valve: domain.ValveOpen},
	}}, nil
}

type gardenCommands struct{ application.CommandStore }

func (gardenCommands) Recent(context.Context, string, string, string, int) ([]domain.Command, error) {
	return []domain.Command{
		{ID: "a", NodeID: "n3", Status: domain.CommandSent, RequestedBy: "ana", CreatedAt: t0.Add(-time.Minute), ExpiresAt: t0.Add(time.Minute)},
		{ID: "b", NodeID: "n3", Status: domain.CommandConfirmed, RequestedBy: "bia", CreatedAt: t0.Add(-time.Hour), ExpiresAt: t0.Add(-time.Hour + 30*time.Second)},
		{ID: "c", NodeID: "n9", Status: domain.CommandConfirmed, RequestedBy: "cris", CreatedAt: t0.Add(-time.Minute)},
	}, nil
}

type gardenAlerts struct{ application.AlertStore }

func (gardenAlerts) List(_ context.Context, _, _ string, onlyOpen bool) ([]domain.Alert, error) {
	n := "n3"
	return []domain.Alert{{ID: "x", NodeID: &n, Severity: "warning", Kind: "wind", Message: "Vento acima do limite", RaisedAt: t0.Add(-30 * time.Minute)}}, nil
}

func gardenService() *application.Service {
	return &application.Service{
		Catalog: gardenCatalog{}, Readings: gardenReadings{}, Twin: gardenTwin{}, Commands: gardenCommands{},
		Alerts: gardenAlerts{}, Clock: fakeClock{},
	}
}

func TestOverviewJoinsReadingValveAndOpenCommand(t *testing.T) {
	o, err := gardenService().Overview(context.Background(), operator, "h1")
	if err != nil {
		t.Fatal(err)
	}
	if len(o.Beds) != 2 || o.OpenAlerts != 1 {
		t.Fatalf("beds %d alertas %d", len(o.Beds), o.OpenAlerts)
	}
	alface, tomate := o.Beds[0], o.Beds[1]
	if alface.Status != domain.MoistureBelow || alface.Moisture.Value != 22 || alface.AirTemp.Value != 24.5 {
		t.Fatalf("alface: %+v", alface)
	}
	if !alface.Irrigating() || alface.OpenCommand == nil || alface.OpenCommand.ID != "a" {
		t.Fatalf("rega em curso esperada, comando em aberto %+v", alface.OpenCommand)
	}
	if tomate.Status != domain.MoistureUnknown || tomate.Irrigating() {
		t.Fatalf("sem leitura não pode virar 'dentro da faixa': %+v", tomate)
	}
}

func TestBedDecisionsMergesCommandsAndAlertsOfBedOnly(t *testing.T) {
	ds, err := gardenService().BedDecisions(context.Background(), operator, "h1", "c1", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(ds) != 3 {
		t.Fatalf("esperava 2 comandos do canteiro e 1 alerta, veio %d", len(ds))
	}
	if ds[0].Reason != "Rega manual pedida por ana" || ds[1].Kind != domain.DecisionAlert || ds[2].Reason != "Rega manual pedida por bia" {
		t.Fatalf("ordem ou motivo inesperado: %+v", ds)
	}
}

func TestAggregateRejectsUnknownBucketAndHugeRange(t *testing.T) {
	svc := gardenService()
	for _, q := range []application.AggregateQuery{
		{Kind: domain.KindSoilMoisture, Bucket: 7 * time.Minute},
		{Kind: domain.KindSoilMoisture, Bucket: 5 * time.Minute, From: t0.Add(-30 * 24 * time.Hour), To: t0},
		{Bucket: time.Hour},
	} {
		if _, err := svc.Aggregate(context.Background(), operator, q); err == nil {
			t.Fatalf("deveria recusar %+v", q)
		}
	}
}
