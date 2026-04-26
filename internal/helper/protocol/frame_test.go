package protocol

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFrame_RoundTrip(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, WriteFrame(&buf, []byte(`{"hello":"world"}`)))
	got, err := ReadFrame(&buf, 1<<20)
	require.NoError(t, err)
	require.Equal(t, `{"hello":"world"}`, string(got))
}

func TestFrame_RejectsOversize(t *testing.T) {
	var buf bytes.Buffer
	huge := strings.Repeat("a", 1<<20+1)
	require.NoError(t, WriteFrame(&buf, []byte(huge)))
	_, err := ReadFrame(&buf, 1<<20)
	require.Error(t, err)
	require.Contains(t, err.Error(), "frame too large")
}

func TestFrame_TruncatedHeader(t *testing.T) {
	r := bytes.NewReader([]byte{0x00, 0x00})
	_, err := ReadFrame(r, 1<<20)
	require.Error(t, err)
	require.True(t, errors.Is(err, io.ErrUnexpectedEOF))
}

func TestFrame_TruncatedBody(t *testing.T) {
	// length=10, body only 4 bytes
	r := bytes.NewReader([]byte{0, 0, 0, 10, 'a', 'b', 'c', 'd'})
	_, err := ReadFrame(r, 1<<20)
	require.Error(t, err)
	require.True(t, errors.Is(err, io.ErrUnexpectedEOF))
}
