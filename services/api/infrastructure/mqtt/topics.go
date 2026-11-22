// Package mqtt liga a API ao EMQX: publica comandos (MQTT 5 com Correlation Data e Message
// Expiry) e consome o estado reportado e as respostas dos nós. Nunca fala com o atuador.
package mqtt

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/JohnnyMatteus/garden-iot/services/api/domain"
)

// NodeRef identifica o nó pelo tópico; a ACL do broker garante que o nó só publica nos seus.
type NodeRef struct{ Tenant, Garden, Node string }

func (n NodeRef) topic(suffix string) string {
	return fmt.Sprintf("horta/%s/%s/%s/%s", n.Tenant, n.Garden, n.Node, suffix)
}

const (
	suffixState    = "estado"
	suffixResponse = "cmd/resposta"
	suffixCommand  = "cmd"
)

var errUnknownTopic = errors.New("tópico não reconhecido")

// parseTopic extrai o nó e o sufixo de horta/{tenant}/{horta}/{no}/{sufixo}.
func parseTopic(topic string) (NodeRef, string, error) {
	p := strings.Split(topic, "/")
	if len(p) < 5 || p[0] != "horta" {
		return NodeRef{}, "", errUnknownTopic
	}
	ref := NodeRef{p[1], p[2], p[3]}
	for _, id := range []string{ref.Tenant, ref.Garden, ref.Node} {
		if !domain.ValidIdentifier(id) {
			return NodeRef{}, "", fmt.Errorf("%w: identificador %q", errUnknownTopic, id)
		}
	}
	return ref, strings.Join(p[4:], "/"), nil
}

type commandPayload struct {
	ID        string `json:"id"`
	Action    string `json:"action"`
	DurationS int    `json:"duration_s"`
	ExpiresAt int64  `json:"expires_at"`
}

func encodeCommand(c domain.Command) ([]byte, error) {
	return json.Marshal(commandPayload{ID: c.ID, Action: c.Action, DurationS: c.DurationSeconds, ExpiresAt: c.ExpiresAt.Unix()})
}

type statePayload struct {
	Valve   string   `json:"valve"`
	FlowLpm *float64 `json:"flow_lpm"`
	TS      int64    `json:"ts"`
}

func decodeState(payload []byte, now time.Time) (domain.ActuatorState, error) {
	var p statePayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return domain.ActuatorState{}, err
	}
	v := domain.Valve(p.Valve)
	if v != domain.ValveOpen && v != domain.ValveClosed {
		return domain.ActuatorState{}, fmt.Errorf("válvula inválida %q", p.Valve)
	}
	at := now
	if p.TS > 0 {
		at = time.Unix(p.TS, 0).UTC()
	}
	return domain.ActuatorState{Valve: v, FlowLpm: p.FlowLpm, At: at}, nil
}

type responsePayload struct {
	ID       string `json:"id"`
	Accepted bool   `json:"accepted"`
}

// decodeResponse prefere o Correlation Data (MQTT 5) e cai para o id no corpo.
func decodeResponse(payload, correlation []byte) (id string, accepted bool, err error) {
	var p responsePayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return "", false, err
	}
	id = p.ID
	if len(correlation) > 0 {
		id = string(correlation)
	}
	if id == "" {
		return "", false, errors.New("resposta sem id de comando")
	}
	return id, p.Accepted, nil
}
