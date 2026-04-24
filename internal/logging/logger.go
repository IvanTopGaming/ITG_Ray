package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
)

type humanHandler struct {
	mu    *sync.Mutex
	w     io.Writer
	level slog.Level
	scope string
	attrs []slog.Attr
}

// NewHandler returns a human-readable slog.Handler that writes to w at the
// given minimum level. Secret values are redacted before writing. It is safe
// for concurrent use.
func NewHandler(w io.Writer, level slog.Level) slog.Handler {
	return &humanHandler{mu: &sync.Mutex{}, w: w, level: level}
}

// Enabled reports whether records at level l should be handled.
func (h *humanHandler) Enabled(_ context.Context, l slog.Level) bool { return l >= h.level }

// Handle formats and writes r to the handler's writer after redacting secrets.
func (h *humanHandler) Handle(_ context.Context, r slog.Record) error { //nolint:gocritic // slog.Handler interface requires slog.Record by value
	var b strings.Builder
	b.WriteString(r.Time.Format("2006-01-02T15:04:05.000Z07:00"))
	b.WriteByte(' ')
	b.WriteString(levelTag(r.Level))
	b.WriteByte(' ')
	if h.scope != "" {
		b.WriteByte('[')
		b.WriteString(h.scope)
		b.WriteString("] ")
	}
	b.WriteString(r.Message)

	write := func(a slog.Attr) {
		if a.Key == "scope" {
			return
		}
		b.WriteByte(' ')
		b.WriteString(a.Key)
		b.WriteByte('=')
		b.WriteString(fmt.Sprint(a.Value.Any()))
	}
	for _, a := range h.attrs {
		write(a)
	}
	r.Attrs(func(a slog.Attr) bool { write(a); return true })

	line := Redact(b.String()) + "\n"
	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := io.WriteString(h.w, line)
	return err
}

// WithAttrs returns a new handler with the given attributes pre-set. A "scope"
// attribute is treated specially: it renders as [scope] in the log prefix
// instead of appearing as a trailing key=value pair.
func (h *humanHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	dup := *h
	for _, a := range attrs {
		if a.Key == "scope" {
			dup.scope = a.Value.String()
			continue
		}
		dup.attrs = append(dup.attrs, a)
	}
	return &dup
}

// WithGroup returns the handler unchanged; groups are not supported.
func (h *humanHandler) WithGroup(_ string) slog.Handler { return h }

func levelTag(l slog.Level) string {
	switch {
	case l >= slog.LevelError:
		return "ERROR"
	case l >= slog.LevelWarn:
		return "WARN "
	case l >= slog.LevelInfo:
		return "INFO "
	default:
		return "DEBUG"
	}
}
