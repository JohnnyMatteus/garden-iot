package devicepayload

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Registry despacha o payload para o tradutor da versão maior do firmware.
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

// Translate lê fw_version do payload e delega ao tradutor correspondente.
func (r *Registry) Translate(raw []byte, cal Calibration) (Measurements, error) {
	var head struct {
		FWVersion string `json:"fw_version"`
	}
	if err := json.Unmarshal(raw, &head); err != nil {
		return Measurements{}, fmt.Errorf("%w: %v", ErrInvalidPayload, err)
	}
	if head.FWVersion == "" {
		return Measurements{}, fmt.Errorf("%w: fw_version ausente", ErrInvalidPayload)
	}
	major, ok := parseMajor(head.FWVersion)
	if !ok {
		return Measurements{}, &UnsupportedFirmwareError{Version: head.FWVersion}
	}
	t, ok := r.byMajor[major]
	if !ok {
		return Measurements{}, &UnsupportedFirmwareError{Version: head.FWVersion}
	}
	return t.Translate(raw, cal)
}

func parseMajor(version string) (int, bool) {
	major, _, _ := strings.Cut(strings.TrimPrefix(version, "v"), ".")
	n, err := strconv.Atoi(major)
	return n, err == nil && n >= 0
}
