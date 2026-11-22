package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JohnnyMatteus/garden-iot/services/irrigation/infrastructure/config"
)

func write(t *testing.T, body string) string {
	p := filepath.Join(t.TempDir(), "beds.json")
	require.NoError(t, os.WriteFile(p, []byte(body), 0o600))
	return p
}

func TestLoadBeds(t *testing.T) {
	beds, err := config.LoadBeds(write(t, `[{"id":"b1","tenant":"demo","garden":"h1","node":"n1","crop":"tomato","stage":"mid",
		"area_m2":2,"method":"drip","flow_lpm":4,"lat":-23.5,"lon":-46.6,"elevation_m":760,"moisture_threshold_pct":35}]`))
	require.NoError(t, err)
	require.Len(t, beds, 1)
	assert.Equal(t, "tomato", beds[0].Crop.Name)
	assert.Equal(t, "n1", beds[0].Ref.Node)
}

func TestLoadBedsRejectsInvalid(t *testing.T) {
	for name, body := range map[string]string{
		"cultura": `[{"id":"b","tenant":"t","garden":"g","node":"n","crop":"uva","method":"drip","area_m2":1,"flow_lpm":1}]`,
		"método":  `[{"id":"b","tenant":"t","garden":"g","node":"n","crop":"tomato","method":"inundação","area_m2":1,"flow_lpm":1}]`,
		"vazão":   `[{"id":"b","tenant":"t","garden":"g","node":"n","crop":"tomato","method":"drip","area_m2":1,"flow_lpm":0}]`,
		"json":    `{`,
	} {
		_, err := config.LoadBeds(write(t, body))
		assert.Error(t, err, name)
	}
}
