package devicepayload

import (
	"fmt"
	"strconv"
	"strings"

	brutav1 "github.com/JohnnyMatteus/garden-iot/contracts/gen/go/telemetria/bruta/v1"
)

// Registry despacha o lote para o tradutor da versão maior de firmware em fw_version.
type Registry struct {
	byMajor map[int]Translator
}

// NewRegistry devolve o registro com os tradutores conhecidos (v1).
func NewRegistry() *Registry {
	r := &Registry{byMajor: map[int]Translator{}}
	r.Register(1, V1Translator{})
	return r
}

// Register associa um tradutor a uma versão maior de firmware.
func (r *Registry) Register(major int, t Translator) {
	r.byMajor[major] = t
}

// Translate escolhe o tradutor por msg.FwVersion e converte msg.Payload.
func (r *Registry) Translate(msg *brutav1.TelemetriaBruta, cals CalibrationSource) (Measurements, error) {
	if msg == nil || msg.GetFwVersion() == "" {
		return Measurements{}, fmt.Errorf("%w: fw_version ausente", ErrInvalidPayload)
	}
	major, ok := parseMajor(msg.GetFwVersion())
	if !ok {
		return Measurements{}, &UnsupportedFirmwareError{Version: msg.GetFwVersion()}
	}
	t, ok := r.byMajor[major]
	if !ok {
		return Measurements{}, &UnsupportedFirmwareError{Version: msg.GetFwVersion()}
	}
	return t.Translate(msg, cals)
}

func parseMajor(version string) (int, bool) {
	major, _, _ := strings.Cut(strings.TrimPrefix(version, "v"), ".")
	n, err := strconv.Atoi(major)
	return n, err == nil && n >= 0
}
