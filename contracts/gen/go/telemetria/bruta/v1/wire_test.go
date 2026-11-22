package brutav1_test

import (
	"bytes"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	brutav1 "github.com/JohnnyMatteus/garden-iot/contracts/gen/go/telemetria/bruta/v1"
)

func sample() *brutav1.TelemetriaBruta {
	return &brutav1.TelemetriaBruta{
		TenantId:   "demo",
		GardenId:   "horta-1",
		NodeId:     "n1",
		Sequence:   10,
		FwVersion:  "1.0.0",
		ReadAt:     timestamppb.New(timestamppb.Now().AsTime().Truncate(1e9)),
		ReceivedAt: timestamppb.New(timestamppb.Now().AsTime().Truncate(1e9)),
		Payload:    []byte(`{"fw":"1.0.0"}`),
	}
}

func TestRoundTrip(t *testing.T) {
	in := sample()
	raw, err := proto.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out brutav1.TelemetriaBruta
	if err := proto.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(in, &out) {
		t.Fatalf("round trip diverged: %v != %v", in, &out)
	}
}

// Trava os números de campo: mudar qualquer um quebra consumidores já em produção (ADR 0002).
func TestFieldNumbersAreStable(t *testing.T) {
	msg := &brutav1.TelemetriaBruta{TenantId: "t", GardenId: "h", NodeId: "n", Sequence: 7, FwVersion: "f", Payload: []byte("p")}
	raw, err := proto.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{
		0x0a, 1, 't', // 1 tenant_id
		0x12, 1, 'h', // 2 garden_id
		0x1a, 1, 'n', // 3 node_id
		0x20, 7, //      4 sequence
		0x2a, 1, 'f', // 5 fw_version
		0x42, 1, 'p', // 8 payload
	}
	if !bytes.Equal(raw, want) {
		t.Fatalf("wire format changed: % x", raw)
	}
}
