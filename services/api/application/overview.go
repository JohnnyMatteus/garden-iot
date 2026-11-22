package application

import (
	"context"
	"fmt"
	"time"

	"github.com/JohnnyMatteus/garden-iot/services/api/domain"
)

// overviewMaxAge: leitura mais velha que isso não pinta o canteiro (o mapa mostra "sem leitura").
const overviewMaxAge = 30 * time.Minute

type GardenOverview struct {
	Beds       []domain.BedOverview
	OpenAlerts int
}

// Overview monta o mapa da horta com poucas consultas: catálogo, última leitura por nó,
// estado dos atuadores, comandos recentes e alertas abertos.
func (s *Service) Overview(ctx context.Context, p domain.Principal, gardenID string) (GardenOverview, error) {
	t := p.TenantID
	if _, err := s.Catalog.GetGarden(ctx, t, gardenID); err != nil {
		return GardenOverview{}, err
	}
	beds, err := s.Catalog.ListBeds(ctx, t, gardenID)
	if err != nil {
		return GardenOverview{}, err
	}
	nodes, err := s.Catalog.ListNodes(ctx, t, gardenID)
	if err != nil {
		return GardenOverview{}, err
	}
	latest, err := s.Readings.LatestByNode(ctx, t, gardenID, s.Clock.Now().Add(-overviewMaxAge))
	if err != nil {
		return GardenOverview{}, err
	}
	states, err := s.Twin.States(ctx, t, gardenID)
	if err != nil {
		return GardenOverview{}, err
	}
	cmds, err := s.Commands.Recent(ctx, t, gardenID, "", 50)
	if err != nil {
		return GardenOverview{}, err
	}
	alerts, err := s.Alerts.List(ctx, t, gardenID, true)
	if err != nil {
		return GardenOverview{}, err
	}

	stateByNode := map[string]domain.NodeState{}
	for _, st := range states {
		stateByNode[st.NodeID] = st
	}
	now := s.Clock.Now()
	out := GardenOverview{OpenAlerts: len(alerts)}
	for _, b := range beds {
		o := domain.BedOverview{Bed: b, Band: domain.BedBand(b)}
		for _, n := range nodes {
			if n.BedID == nil || *n.BedID != b.ID {
				continue
			}
			switch n.Kind {
			case domain.SensorNode:
				o.SensorNodeID = n.ID
				for i := range latest {
					r := latest[i]
					if r.NodeID != n.ID {
						continue
					}
					switch r.Kind {
					case domain.KindSoilMoisture:
						o.Moisture = &r
					case domain.KindAirTemp:
						o.AirTemp = &r
					}
				}
			case domain.ActuatorNode:
				o.ActuatorNodeID = n.ID
				if st, ok := stateByNode[n.ID]; ok {
					o.Valve = &st
				}
				for i := range cmds {
					c := cmds[i]
					if c.NodeID == n.ID && (c.Status == domain.CommandPending || c.Status == domain.CommandSent) && c.ExpiresAt.After(now) {
						o.OpenCommand = &c
						break
					}
				}
			}
		}
		var v *float64
		if o.Moisture != nil {
			v = &o.Moisture.Value
		}
		o.Status = o.Band.Classify(v)
		out.Beds = append(out.Beds, o)
	}
	return out, nil
}

// ListCommands lista o histórico de comandos (estado desejado versus confirmação) da horta.
func (s *Service) ListCommands(ctx context.Context, p domain.Principal, gardenID, nodeID string, limit int) ([]domain.Command, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		return nil, fmt.Errorf("%w: limit máximo 500", domain.ErrInvalidInput)
	}
	if _, err := s.Catalog.GetGarden(ctx, p.TenantID, gardenID); err != nil {
		return nil, err
	}
	return s.Commands.Recent(ctx, p.TenantID, gardenID, nodeID, limit)
}

// BedDecisions reúne o que aconteceu no canteiro: regas pedidas e alertas das regras de segurança.
// Quando o planejador passar a persistir as decisões automáticas, elas entram aqui.
func (s *Service) BedDecisions(ctx context.Context, p domain.Principal, gardenID, bedID string, limit int) ([]domain.Decision, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		return nil, fmt.Errorf("%w: limit máximo 200", domain.ErrInvalidInput)
	}
	if _, err := s.Catalog.GetBed(ctx, p.TenantID, gardenID, bedID); err != nil {
		return nil, err
	}
	nodes, err := s.Catalog.ListNodes(ctx, p.TenantID, gardenID)
	if err != nil {
		return nil, err
	}
	inBed := map[string]bool{}
	for _, n := range nodes {
		if n.BedID != nil && *n.BedID == bedID {
			inBed[n.ID] = true
		}
	}
	cmds, err := s.Commands.Recent(ctx, p.TenantID, gardenID, "", 200)
	if err != nil {
		return nil, err
	}
	alerts, err := s.Alerts.List(ctx, p.TenantID, gardenID, false)
	if err != nil {
		return nil, err
	}
	var ds []domain.Decision
	for _, c := range cmds {
		if inBed[c.NodeID] {
			ds = append(ds, domain.DecisionFromCommand(c))
		}
	}
	for _, a := range alerts {
		if a.NodeID != nil && inBed[*a.NodeID] {
			ds = append(ds, domain.DecisionFromAlert(a))
		}
	}
	return domain.SortDecisions(ds, limit), nil
}

var aggregateBuckets = map[time.Duration]bool{5 * time.Minute: true, 15 * time.Minute: true, time.Hour: true, 24 * time.Hour: true}

const maxAggregatePoints = 2000

// Aggregate devolve a série em baldes de tempo (média, mínimo e máximo) para os gráficos.
func (s *Service) Aggregate(ctx context.Context, p domain.Principal, q AggregateQuery) ([]domain.AggregatePoint, error) {
	if q.Kind == "" || !aggregateBuckets[q.Bucket] {
		return nil, fmt.Errorf("%w: kind e bucket (5m, 15m, 1h ou 1d) são obrigatórios", domain.ErrInvalidInput)
	}
	if q.To.IsZero() {
		q.To = s.Clock.Now()
	}
	if q.From.IsZero() {
		q.From = q.To.Add(-24 * time.Hour)
	}
	if !q.To.After(q.From) || int(q.To.Sub(q.From)/q.Bucket) > maxAggregatePoints {
		return nil, fmt.Errorf("%w: intervalo vazio ou com mais de %d baldes", domain.ErrInvalidInput, maxAggregatePoints)
	}
	return s.Readings.Aggregate(ctx, p.TenantID, q)
}
