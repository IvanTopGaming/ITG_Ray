package main

import (
	"bytes"
	"io"
	"net"
	"os"
	"testing"

	"github.com/itg-team/itg-ray/internal/subscription"
	"github.com/stretchr/testify/require"
)

// captureStdout redirects os.Stdout for the duration of fn and returns
// everything written to it.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()

	require.NoError(t, w.Close())
	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	require.NoError(t, err)
	return buf.String()
}

// TestSubSync_RedactsTokenFromStdoutAndStore is the RED test for backend
// review Finding 1: a fetch failure on `sub sync` must not leak the
// subscription URL/token to stdout, nor persist it into subscriptions.json
// via UpdateMeta. The failure is a real (fast) connection-refused, produced
// by connecting to a port nothing is listening on, so subscription.Sync
// returns the exact *url.Error-wrapping error path the finding describes.
func TestSubSync_RedactsTokenFromStdoutAndStore(t *testing.T) {
	dir := t.TempDir()
	origDataDir := dataDir
	dataDir = dir
	defer func() { dataDir = origDataDir }()

	// Reserve then release a port so dialing it is refused immediately —
	// fast and deterministic, no real network/DNS dependency.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()
	require.NoError(t, ln.Close())

	const token = "REDACTED"
	subURL := "http://" + addr + "/" + token + "/api/sub/00000000-0000-4000-8000-000000000000"

	st := subsStore()
	require.NoError(t, st.Save([]subscription.Stored{{ID: "s1", Name: "test", URL: subURL}}))

	syncCmd, _, err := newSubCmd().Find([]string{"sync"})
	require.NoError(t, err)

	stdout := captureStdout(t, func() {
		require.NoError(t, syncCmd.RunE(syncCmd, nil))
	})

	require.NotContains(t, stdout, token, "token leaked into sub sync stdout")

	// The persisted "url" field legitimately still carries the full
	// subscription URL/token — that's the user's own stored config
	// (Finding 3, out of scope here), not a leak. What Finding 1 is about is
	// the derived LastMessage field, which must be the redacted message, not
	// a re-derivation from the raw error.
	persisted, err := st.Load()
	require.NoError(t, err)
	require.Len(t, persisted, 1)
	require.NotContains(t, persisted[0].LastMessage, token, "token leaked into persisted LastMessage")
	require.Equal(t, "error", persisted[0].LastStatus)
}
