package readings_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/domain"
	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/readings"
)

type norm struct{}

func (norm) Normalize(b readings.Batch) ([]readings.Reading, error) {
	if string(b.Payload) == "ruim" {
		return nil, domain.NewInvalid(domain.ReasonDecode, errors.New("ruim"))
	}
	return []readings.Reading{{Node: b.Node, Sequence: b.Sequence, Kind: "a"}, {Node: b.Node, Sequence: b.Sequence, Kind: "b"}}, nil
}

type store struct {
	saved [][]readings.Reading
	err   error
}

func (s *store) Save(_ context.Context, r []readings.Reading) error {
	if s.err != nil {
		return s.err
	}
	s.saved = append(s.saved, r)
	return nil
}

type dlq struct{ got []domain.Rejected }

func (d *dlq) PublishRejected(_ context.Context, r domain.Rejected) error {
	d.got = append(d.got, r)
	return nil
}

type m struct{ stored int }

func (x *m) Stored(n int)           { x.stored += n }
func (x *m) Rejected(domain.Reason) {}

func TestIngestWritesOnceAndRejectsInvalid(t *testing.T) {
	s, d, mm := &store{}, &dlq{}, &m{}
	i := readings.NewIngestor(norm{}, s, d, mm, slog.New(slog.NewTextHandler(io.Discard, nil)))

	err := i.Ingest(context.Background(), []readings.Batch{
		{Node: "n1", Sequence: 1, Payload: []byte("ok")},
		{Node: "n1", Sequence: 2, Payload: []byte("ruim")},
		{Node: "n2", Sequence: 1, Payload: []byte("ok")},
	})
	require.NoError(t, err)
	require.Len(t, s.saved, 1, "uma escrita para o lote todo")
	assert.Len(t, s.saved[0], 4)
	require.Len(t, d.got, 1)
	assert.Equal(t, domain.ReasonDecode, d.got[0].Reason)
	assert.Equal(t, 4, mm.stored)
}

func TestIngestStoreFailureIsReturned(t *testing.T) {
	s := &store{err: errors.New("db fora")}
	i := readings.NewIngestor(norm{}, s, &dlq{}, &m{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.Error(t, i.Ingest(context.Background(), []readings.Batch{{Node: "n", Payload: []byte("ok")}}))
}
