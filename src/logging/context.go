package logging

import (
	"context"
	"log/slog"
	"sync"
)

type requestLogStateKey struct{}

type RequestLogState struct {
	mu    sync.Mutex
	attrs []slog.Attr
}

func ContextWithRequestLogState(ctx context.Context) (context.Context, *RequestLogState) {
	state := &RequestLogState{}
	return context.WithValue(ctx, requestLogStateKey{}, state), state
}

func WithAttrs(ctx context.Context, attrs ...slog.Attr) {
	state := RequestLogStateFromContext(ctx)
	if state == nil || len(attrs) == 0 {
		return
	}
	state.mu.Lock()
	for _, attr := range attrs {
		replaced := false
		for i := range state.attrs {
			if state.attrs[i].Key == attr.Key {
				state.attrs[i] = attr
				replaced = true
				break
			}
		}
		if !replaced {
			state.attrs = append(state.attrs, attr)
		}
	}
	state.mu.Unlock()
}

func SetError(ctx context.Context, msg string) {
	if msg == "" {
		return
	}
	WithAttrs(ctx, slog.String("error", msg))
}

func RequestLogStateFromContext(ctx context.Context) *RequestLogState {
	if ctx == nil {
		return nil
	}
	state, _ := ctx.Value(requestLogStateKey{}).(*RequestLogState)
	return state
}

func (s *RequestLogState) Attrs() []slog.Attr {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]slog.Attr, len(s.attrs))
	copy(out, s.attrs)
	return out
}
