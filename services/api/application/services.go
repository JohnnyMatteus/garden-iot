package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JohnnyMatteus/garden-iot/services/api/domain"
)

var ErrPublish = errors.New("broker indisponível")

type Service struct {
	Catalog   Catalog
	Readings  ReadingStore
	Twin      TwinStore
	Alerts    AlertStore
	Commands  CommandStore
	Publisher CommandPublisher
	Policy    domain.IrrigationPolicy
	Clock     Clock
	IDs       IDGenerator

	CommandTTL       time.Duration // validade do comando no broker (Message Expiry)
	ConditionsMaxAge time.Duration // idade máxima da leitura usada pela política
}

func requireOwner(p domain.Principal) error {
	if p.Role != domain.RoleOwner {
		return domain.ErrForbidden
	}
	return nil
}

func (s *Service) CreateGarden(ctx context.Context, p domain.Principal, g domain.Garden) (domain.Garden, error) {
	if err := requireOwner(p); err != nil {
		return domain.Garden{}, err
	}
	if !domain.ValidIdentifier(g.ID) || g.Name == "" {
		return domain.Garden{}, fmt.Errorf("%w: id ou nome da horta", domain.ErrInvalidInput)
	}
	return s.Catalog.CreateGarden(ctx, p.TenantID, g)
}

func (s *Service) CreateBed(ctx context.Context, p domain.Principal, b domain.Bed) (domain.Bed, error) {
	if err := requireOwner(p); err != nil {
		return domain.Bed{}, err
	}
	if !domain.ValidIdentifier(b.ID) || b.Name == "" || (b.Irrigation != domain.Drip && b.Irrigation != domain.Sprinkler) {
		return domain.Bed{}, fmt.Errorf("%w: canteiro", domain.ErrInvalidInput)
	}
	return s.Catalog.CreateBed(ctx, p.TenantID, b)
}

func (s *Service) CreateNode(ctx context.Context, p domain.Principal, n domain.Node) (domain.Node, error) {
	if err := requireOwner(p); err != nil {
		return domain.Node{}, err
	}
	if !domain.ValidIdentifier(n.ID) || (n.Kind != domain.SensorNode && n.Kind != domain.ActuatorNode) {
		return domain.Node{}, fmt.Errorf("%w: nó", domain.ErrInvalidInput)
	}
	return s.Catalog.CreateNode(ctx, p.TenantID, n)
}

// History valida o intervalo e limita o tamanho da consulta.
func (s *Service) History(ctx context.Context, p domain.Principal, q ReadingQuery) ([]domain.Reading, error) {
	if q.Limit <= 0 {
		q.Limit = 500
	}
	if q.Limit > 5000 {
		return nil, fmt.Errorf("%w: limit máximo 5000", domain.ErrInvalidInput)
	}
	if !q.From.IsZero() && !q.To.IsZero() && q.To.Before(q.From) {
		return nil, fmt.Errorf("%w: to anterior a from", domain.ErrInvalidInput)
	}
	return s.Readings.History(ctx, p.TenantID, q)
}

func (s *Service) NodeState(ctx context.Context, p domain.Principal, gardenID, nodeID string) (domain.NodeState, error) {
	if _, err := s.Catalog.GetNode(ctx, p.TenantID, gardenID, nodeID); err != nil {
		return domain.NodeState{}, err
	}
	return s.Twin.State(ctx, p.TenantID, gardenID, nodeID)
}

func (s *Service) AcknowledgeAlert(ctx context.Context, p domain.Principal, gardenID, alertID string) error {
	if !p.Role.CanOperate() {
		return domain.ErrForbidden
	}
	return s.Alerts.Acknowledge(ctx, p.TenantID, gardenID, alertID, s.Clock.Now())
}

// RequestIrrigation avalia a política e, se aprovada, grava o comando e o estado desejado e o
// publica no broker. O atuador só muda se o nó aceitar (confirmação chega por MQTT).
func (s *Service) RequestIrrigation(ctx context.Context, p domain.Principal, gardenID, nodeID string, duration time.Duration) (domain.Command, error) {
	if !p.Role.CanOperate() {
		return domain.Command{}, domain.ErrForbidden
	}
	node, err := s.Catalog.GetNode(ctx, p.TenantID, gardenID, nodeID)
	if err != nil {
		return domain.Command{}, err
	}
	cond, err := s.conditions(ctx, p.TenantID, node)
	if err != nil {
		return domain.Command{}, err
	}
	if err := s.Policy.Evaluate(p.Role, node, duration, cond); err != nil {
		return domain.Command{}, err
	}

	now := s.Clock.Now()
	cmd := domain.Command{
		ID: s.IDs.NewID(), GardenID: gardenID, NodeID: nodeID, Action: domain.ActionOpenValve,
		DurationSeconds: int(duration / time.Second), Status: domain.CommandPending,
		RequestedBy: p.Subject, CreatedAt: now, ExpiresAt: now.Add(s.CommandTTL),
	}
	if err := s.Commands.Create(ctx, p.TenantID, cmd); err != nil {
		return domain.Command{}, err
	}
	if err := s.Twin.SetDesired(ctx, p.TenantID, gardenID, nodeID, domain.ActuatorState{Valve: domain.ValveOpen, At: now}); err != nil {
		return domain.Command{}, err
	}
	if err := s.Publisher.Publish(ctx, p.TenantID, cmd); err != nil {
		_ = s.Commands.SetStatus(ctx, p.TenantID, cmd.ID, domain.CommandFailed)
		return domain.Command{}, fmt.Errorf("%w: %v", ErrPublish, err)
	}
	// A confirmação do nó pode chegar antes deste update; nesse caso o status já é final.
	if err := s.Commands.SetStatus(ctx, p.TenantID, cmd.ID, domain.CommandSent); err != nil && !errors.Is(err, domain.ErrNotFound) {
		return domain.Command{}, err
	}
	cmd.Status = domain.CommandSent
	return cmd, nil
}

func (s *Service) conditions(ctx context.Context, tenant string, node domain.Node) (domain.Conditions, error) {
	since := s.Clock.Now().Add(-s.ConditionsMaxAge)
	c := domain.Conditions{}
	latest := func(kind string) (*float64, error) {
		r, err := s.Readings.Latest(ctx, tenant, node.GardenID, kind, since)
		if err != nil || r == nil {
			return nil, err
		}
		return &r.Value, nil
	}
	var err error
	if c.AirTempC, err = latest("air_temp_c"); err != nil {
		return c, err
	}
	if c.WindMS, err = latest("wind_ms"); err != nil {
		return c, err
	}
	if c.ReservoirPct, err = latest("reservoir_level_pct"); err != nil {
		return c, err
	}
	if node.BedID != nil {
		bed, err := s.Catalog.GetBed(ctx, tenant, node.GardenID, *node.BedID)
		if err != nil {
			return c, err
		}
		c.Irrigation = bed.Irrigation
	}
	if c.OpenCommand, err = s.Commands.HasOpen(ctx, tenant, node.GardenID, node.ID); err != nil {
		return c, err
	}
	return c, nil
}

// StreamReadings emite leituras novas da horta até o contexto terminar. Consulta por
// ingested_at com o papel do tenant, então o isolamento não depende de filtro na aplicação.
func (s *Service) StreamReadings(ctx context.Context, p domain.Principal, gardenID string, every time.Duration) (<-chan domain.Reading, error) {
	if _, err := s.Catalog.GetGarden(ctx, p.TenantID, gardenID); err != nil {
		return nil, err
	}
	out := make(chan domain.Reading, 64)
	cursor := s.Clock.Now()
	go func() {
		defer close(out)
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				rows, err := s.Readings.IngestedAfter(ctx, p.TenantID, gardenID, cursor, 500)
				if err != nil {
					continue
				}
				for _, r := range rows {
					select {
					case out <- r.Reading:
						cursor = r.IngestedAt
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()
	return out, nil
}

// ApplyNodeReport trata o estado reportado por um nó (tópico .../estado).
func (s *Service) ApplyNodeReport(ctx context.Context, tenant, gardenID, nodeID string, st domain.ActuatorState) error {
	return s.Twin.SetReported(ctx, tenant, gardenID, nodeID, st)
}

// ApplyCommandResponse trata a resposta do nó (.../cmd/resposta).
func (s *Service) ApplyCommandResponse(ctx context.Context, tenant, commandID string, accepted bool) error {
	status := domain.CommandRejected
	if accepted {
		status = domain.CommandConfirmed
	}
	return s.Commands.SetStatus(ctx, tenant, commandID, status)
}

// ExpireCommands fecha comandos sem confirmação dentro do prazo (design doc, seção 6.4).
func (s *Service) ExpireCommands(ctx context.Context) (int, error) {
	return s.Commands.ExpireBefore(ctx, s.Clock.Now())
}
