// Package envelope valida o envelope do payload do firmware (fw_version, seq, ts) e monta o lote
// de domínio. A conversão das leituras para grandezas físicas é do DevicePayloadTranslator
// (internal/acl), usado a jusante pela persistência; a bridge repassa o corpo sem interpretar.
package envelope

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/domain"
)

var semver = regexp.MustCompile(`^v?\d+\.\d+\.\d+([-+][0-9A-Za-z.-]+)?$`)

// MaxPayloadBytes limita o corpo aceito; um lote real tem cerca de 170 bytes.
const MaxPayloadBytes = 16 * 1024

// Translator implementa application.PayloadTranslator.
type Translator struct{ now func() time.Time }

func New() *Translator { return &Translator{now: time.Now} }

type head struct {
	FwVersion string  `json:"fw_version"`
	Fw        string  `json:"fw"` // alias do contrato MQTT
	Seq       *uint64 `json:"seq"`
	Ts        *int64  `json:"ts"`
}

func (t *Translator) Translate(topic domain.Topic, payload []byte) (domain.Telemetry, error) {
	if len(payload) > MaxPayloadBytes {
		return domain.Telemetry{}, domain.NewInvalid(domain.ReasonDecode, fmt.Errorf("payload com %d bytes", len(payload)))
	}
	var h head
	dec := json.NewDecoder(bytes.NewReader(payload))
	if err := dec.Decode(&h); err != nil {
		return domain.Telemetry{}, domain.NewInvalid(domain.ReasonDecode, err)
	}
	fw := h.FwVersion
	if fw == "" {
		fw = h.Fw
	}
	switch {
	case fw == "":
		return domain.Telemetry{}, domain.NewInvalid(domain.ReasonSchema, errors.New("fw_version ausente"))
	case !semver.MatchString(fw):
		return domain.Telemetry{}, domain.NewInvalid(domain.ReasonFirmware, fmt.Errorf("fw_version %q não é semver", fw))
	case h.Seq == nil:
		return domain.Telemetry{}, domain.NewInvalid(domain.ReasonSchema, errors.New("seq ausente"))
	case h.Ts == nil:
		return domain.Telemetry{}, domain.NewInvalid(domain.ReasonSchema, errors.New("ts ausente"))
	}

	readAt := t.now()
	if *h.Ts > 0 { // ts 0 = nó ainda sem SNTP; usa o instante de chegada
		readAt = time.Unix(*h.Ts, 0).UTC()
	}
	return domain.Telemetry{
		Tenant: topic.Tenant, Horta: topic.Horta, NoID: topic.NoID,
		Sequence: *h.Seq, FwVersion: fw, ReadAt: readAt, Payload: payload,
	}, nil
}
