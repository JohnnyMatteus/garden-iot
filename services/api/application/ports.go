// Package application orquestra os casos de uso da API. Depende só do domínio e destas portas.
package application

import (
	"context"
	"time"

	"github.com/JohnnyMatteus/garden-iot/services/api/domain"
)

// Todas as portas recebem o tenant do Principal; a implementação aplica o isolamento (RLS).

type Catalog interface {
	ListGardens(ctx context.Context, tenant string) ([]domain.Garden, error)
	GetGarden(ctx context.Context, tenant, gardenID string) (domain.Garden, error)
	CreateGarden(ctx context.Context, tenant string, g domain.Garden) (domain.Garden, error)
	ListBeds(ctx context.Context, tenant, gardenID string) ([]domain.Bed, error)
	GetBed(ctx context.Context, tenant, gardenID, bedID string) (domain.Bed, error)
	CreateBed(ctx context.Context, tenant string, b domain.Bed) (domain.Bed, error)
	ListNodes(ctx context.Context, tenant, gardenID string) ([]domain.Node, error)
	GetNode(ctx context.Context, tenant, gardenID, nodeID string) (domain.Node, error)
	CreateNode(ctx context.Context, tenant string, n domain.Node) (domain.Node, error)
}

type ReadingQuery struct {
	GardenID string
	NodeID   string
	Kind     string
	From, To time.Time
	Limit    int
}

type ReadingStore interface {
	History(ctx context.Context, tenant string, q ReadingQuery) ([]domain.Reading, error)
	// Latest devolve a leitura mais recente do tipo na horta desde "since", ou nil.
	Latest(ctx context.Context, tenant, gardenID, kind string, since time.Time) (*domain.Reading, error)
	// IngestedAfter devolve leituras gravadas depois do instante, em ordem, para o SSE.
	IngestedAfter(ctx context.Context, tenant, gardenID string, after time.Time, limit int) ([]IngestedReading, error)
}

type IngestedReading struct {
	domain.Reading
	IngestedAt time.Time
}

type TwinStore interface {
	State(ctx context.Context, tenant, gardenID, nodeID string) (domain.NodeState, error)
	SetDesired(ctx context.Context, tenant, gardenID, nodeID string, s domain.ActuatorState) error
	// SetReported é chamado pelo adaptador MQTT, em nome do nó (identidade vem do tópico).
	SetReported(ctx context.Context, tenant, gardenID, nodeID string, s domain.ActuatorState) error
}

type AlertStore interface {
	List(ctx context.Context, tenant, gardenID string, onlyOpen bool) ([]domain.Alert, error)
	Acknowledge(ctx context.Context, tenant, gardenID, alertID string, at time.Time) error
}

type CommandStore interface {
	Create(ctx context.Context, tenant string, c domain.Command) error
	Get(ctx context.Context, tenant, gardenID, nodeID, commandID string) (domain.Command, error)
	HasOpen(ctx context.Context, tenant, gardenID, nodeID string) (bool, error)
	SetStatus(ctx context.Context, tenant, commandID string, s domain.CommandStatus) error
	// ExpireBefore marca como expirados os comandos em aberto vencidos, entre tenants.
	ExpireBefore(ctx context.Context, at time.Time) (int, error)
}

// CommandPublisher entrega o comando ao broker. Nunca aciona atuador: o nó decide.
type CommandPublisher interface {
	Publish(ctx context.Context, tenant string, c domain.Command) error
}

type Clock interface{ Now() time.Time }

type IDGenerator interface{ NewID() string }
