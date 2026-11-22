package mqtt

import (
	"testing"
	"time"

	"github.com/JohnnyMatteus/garden-iot/services/api/domain"
)

func TestParseTopic(t *testing.T) {
	ref, suffix, err := parseTopic("horta/demo/horta-1/n3/cmd/resposta")
	if err != nil || ref != (NodeRef{"demo", "horta-1", "n3"}) || suffix != "cmd/resposta" {
		t.Fatalf("%+v %q %v", ref, suffix, err)
	}
	for _, bad := range []string{"horta/demo/h/n", "x/demo/h/n/estado", "horta/DEMO/h/n/estado", "horta/+/h/n/estado", ""} {
		if _, _, err := parseTopic(bad); err == nil {
			t.Errorf("%q deveria ser rejeitado", bad)
		}
	}
}

func TestEncodeCommandCarriesExpiry(t *testing.T) {
	exp := time.Unix(1763726400, 0)
	b, _ := encodeCommand(domain.Command{ID: "abc", Action: domain.ActionOpenValve, DurationSeconds: 60, ExpiresAt: exp})
	want := `{"id":"abc","action":"open_valve","duration_s":60,"expires_at":1763726400}`
	if string(b) != want {
		t.Fatalf("payload = %s", b)
	}
}

func TestDecodeState(t *testing.T) {
	now := time.Unix(100, 0)
	st, err := decodeState([]byte(`{"valve":"open","flow_lpm":1.5}`), now)
	if err != nil || st.Valve != domain.ValveOpen || *st.FlowLpm != 1.5 || !st.At.Equal(now) {
		t.Fatalf("%+v %v", st, err)
	}
	if _, err := decodeState([]byte(`{"valve":"meio"}`), now); err == nil {
		t.Fatal("válvula inválida deveria falhar")
	}
}

func TestDecodeResponsePrefersCorrelationData(t *testing.T) {
	id, ok, err := decodeResponse([]byte(`{"id":"corpo","accepted":true}`), []byte("corr"))
	if err != nil || id != "corr" || !ok {
		t.Fatalf("%s %v %v", id, ok, err)
	}
	if _, _, err := decodeResponse([]byte(`{}`), nil); err == nil {
		t.Fatal("sem id deveria falhar")
	}
}
