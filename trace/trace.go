package trace

import (
	"context"
	"log/slog"
	"time"
)

type ctxKey int

const (
	requestIDKey ctxKey = iota
	spanKey
)

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

func RequestID(ctx context.Context) string {
	v, _ := ctx.Value(requestIDKey).(string)
	return v
}

type Span struct {
	ctx   context.Context
	path  string
	start time.Time
}

func Start(ctx context.Context, name string) (context.Context, *Span) {
	path := name
	if parent, ok := ctx.Value(spanKey).(string); ok && parent != "" {
		path = parent + " > " + name
	}
	return context.WithValue(ctx, spanKey, path), &Span{ctx: ctx, path: path, start: time.Now()}
}

func (s *Span) End(err error) {
	attrs := []any{
		"request_id", RequestID(s.ctx),
		"span", s.path,
		"duration", time.Since(s.start).String(),
	}
	if err != nil {
		attrs = append(attrs, "error", err.Error())
		slog.Warn("span", attrs...)
		return
	}
	slog.Debug("span", attrs...)
}

func Run[T any](ctx context.Context, name string, fn func(context.Context) (T, error)) (T, error) {
	ctx, span := Start(ctx, name)
	v, err := fn(ctx)
	span.End(err)
	return v, err
}
