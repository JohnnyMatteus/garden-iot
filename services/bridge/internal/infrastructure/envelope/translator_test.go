package envelope_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/domain"
	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/infrastructure/envelope"
)

var topic = domain.Topic{Tenant: "t1", Horta: "h1", NoID: "n1"}

func TestTranslatesFirmwareV1(t *testing.T) {
	raw := []byte(`{"fw_version":"1.0.0","seq":42,"ts":1732190000,"dropped":0,"soil_adc":2100}`)
	got, err := envelope.New().Translate(topic, raw)
	require.NoError(t, err)
	assert.EqualValues(t, 42, got.Sequence)
	assert.Equal(t, "1.0.0", got.FwVersion)
	assert.EqualValues(t, 1732190000, got.ReadAt.Unix())
	assert.Equal(t, "n1", got.NoID)
	assert.Equal(t, raw, got.Payload)
}

func TestAcceptsContractAlias(t *testing.T) {
	got, err := envelope.New().Translate(topic, []byte(`{"fw":"1.2.0","seq":1,"ts":0}`))
	require.NoError(t, err)
	assert.Equal(t, "1.2.0", got.FwVersion)
	assert.False(t, got.ReadAt.IsZero(), "ts 0 cai para o instante de chegada")
}

func TestRejects(t *testing.T) {
	cases := map[string]struct {
		raw    string
		reason domain.Reason
	}{
		"não é json":   {`xx`, domain.ReasonDecode},
		"sem versão":   {`{"seq":1,"ts":1}`, domain.ReasonSchema},
		"versão ruim":  {`{"fw_version":"abc","seq":1,"ts":1}`, domain.ReasonFirmware},
		"sem seq":      {`{"fw_version":"1.0.0","ts":1}`, domain.ReasonSchema},
		"seq negativa": {`{"fw_version":"1.0.0","seq":-1,"ts":1}`, domain.ReasonDecode},
		"sem ts":       {`{"fw_version":"1.0.0","seq":1}`, domain.ReasonSchema},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := envelope.New().Translate(topic, []byte(c.raw))
			ie, ok := domain.AsInvalid(err)
			require.True(t, ok, "esperava InvalidError, veio %v", err)
			assert.Equal(t, c.reason, ie.Reason)
		})
	}
}
