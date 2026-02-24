package ctxlog

import (
	"context"
	"log/slog"
)

type ctxKey string

const (
	slogFields ctxKey = "slog_fields"
)

var _ slog.Handler = &ContextHandler{}

type ContextHandler struct {
	Handler slog.Handler
}

// Handle adds contextual attributes to the Record before calling the underlying
// handler
func (h *ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	if attrs, ok := ctx.Value(slogFields).([]slog.Attr); ok {
		for _, v := range attrs {
			r.AddAttrs(v)
		}
	}
	return h.Handler.Handle(ctx, r)
}

func (h *ContextHandler) Enabled(ctx context.Context, lvl slog.Level) bool {
	return h.Handler.Enabled(ctx, lvl)
}

func (h *ContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h.Handler.WithAttrs(attrs)
}

func (h *ContextHandler) WithGroup(name string) slog.Handler {
	return h.Handler.WithGroup(name)
}

// AppendCtx adds an slog attribute to the provided context so that it will be
// included in any Record created with such context
func AppendCtx(parent context.Context, attr ...slog.Attr) context.Context {
	if parent == nil {
		parent = context.Background()
	}

	if v, ok := parent.Value(slogFields).([]slog.Attr); ok {
		v = append(v, attr...)
		return context.WithValue(parent, slogFields, v)
	}

	v := []slog.Attr{}
	v = append(v, attr...)
	return context.WithValue(parent, slogFields, v)
}
