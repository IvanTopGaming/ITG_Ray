package logstream

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

type tapHandler struct {
	inner slog.Handler
	buf   *Buffer
}

func NewTapHandler(inner slog.Handler, buf *Buffer) slog.Handler {
	return &tapHandler{inner: inner, buf: buf}
}

func (h *tapHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return h.inner.Enabled(ctx, l)
}

func (h *tapHandler) Handle(ctx context.Context, r slog.Record) error {
	h.buf.Add("bridge", levelName(r.Level), formatRecord(r), r.Time)
	return h.inner.Handle(ctx, r)
}

func (h *tapHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &tapHandler{inner: h.inner.WithAttrs(attrs), buf: h.buf}
}

func (h *tapHandler) WithGroup(name string) slog.Handler {
	return &tapHandler{inner: h.inner.WithGroup(name), buf: h.buf}
}

func levelName(l slog.Level) string {
	switch {
	case l >= slog.LevelError:
		return "ERROR"
	case l >= slog.LevelWarn:
		return "WARN"
	case l >= slog.LevelInfo:
		return "INFO"
	default:
		return "DEBUG"
	}
}

func formatRecord(r slog.Record) string {
	var b strings.Builder
	b.WriteString(r.Message)
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "scope" {
			return true
		}
		b.WriteByte(' ')
		b.WriteString(a.Key)
		b.WriteByte('=')
		b.WriteString(fmt.Sprint(a.Value.Any()))
		return true
	})
	return b.String()
}
