/*
Package wide provides a handler for the slog.Handler interface that translates
slog groups into log messages.
*/
package wide

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/druejaramillo/go-wide/ops"
)

type handlerStrategy string

const (
	strategyPassthrough handlerStrategy = "passthrough"
	strategyAggregate   handlerStrategy = "aggregate"
)

type handlerConfig struct {
	strategy handlerStrategy
}

type Option func(*handlerConfig)

func WithAggregate() Option {
	return func(cfg *handlerConfig) {
		cfg.strategy = strategyAggregate
	}
}

type Handler struct {
	handler slog.Handler
	config  *handlerConfig
	prefix  []slog.Attr
	groups  []string
}

func NewHandler(h slog.Handler, opts ...Option) *Handler {
	cfg := &handlerConfig{
		strategy: strategyPassthrough,
	}
	for _, opt := range opts {
		opt(cfg)
	}
	return &Handler{
		handler: h,
		config:  cfg,
	}
}

func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	if h.config.strategy == strategyPassthrough {
		return h.handler.Enabled(ctx, level)
	}
	if state := aggregateStateFromContext(ctx); state != nil {
		return true
	}
	return h.handler.Enabled(ctx, level)
}

func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	if h.config.strategy == strategyPassthrough {
		return h.handler.Handle(ctx, r)
	}
	state := aggregateStateFromContext(ctx)
	if state == nil {
		return h.handler.Handle(ctx, r)
	}

	effectiveRecord := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	effectiveRecord.AddAttrs(
		mergeAttrsIntoGroupPath(
			cloneAttrs(h.prefix),
			h.groups,
			attrsFromRecord(r),
		)...,
	)

	state.collect(effectiveRecord)
	return nil
}

func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &Handler{
		handler: h.handler.WithAttrs(attrs),
		config:  h.config,
		prefix:  mergeAttrsIntoGroupPath(cloneAttrs(h.prefix), h.groups, cloneAttrs(attrs)),
		groups:  append([]string(nil), h.groups...),
	}
}

func (h *Handler) WithGroup(name string) slog.Handler {
	return &Handler{
		handler: h.handler.WithGroup(name),
		config:  h.config,
		prefix:  cloneAttrs(h.prefix),
		groups:  append(append([]string(nil), h.groups...), name),
	}
}

func mergeAttrsIntoGroupPath(base []slog.Attr, path []string, add []slog.Attr) []slog.Attr {
	if len(path) == 0 {
		return append(base, add...)
	}

	group := path[0]

	for i, attr := range base {
		if attr.Key != group {
			continue
		}

		v := attr.Value.Resolve()
		if v.Kind() != slog.KindGroup {
			continue
		}

		merged := mergeAttrsIntoGroupPath(cloneAttrs(v.Group()), path[1:], add)
		base[i] = slog.GroupAttrs(group, merged...)
		return base
	}

	nested := mergeAttrsIntoGroupPath(nil, path[1:], add)
	return append(base, slog.GroupAttrs(group, nested...))
}

func cloneAttrs(attrs []slog.Attr) []slog.Attr {
	if len(attrs) == 0 {
		return nil
	}
	out := make([]slog.Attr, len(attrs))
	copy(out, attrs)
	return out
}

func (h *Handler) RootOption() ops.Option {
	if h.config.strategy != strategyAggregate {
		return func(rc *ops.RootConfig) {
			rc.LifecycleObservers = append(rc.LifecycleObservers, noopLifecycleObserver{})
			rc.ErrorObservers = append(rc.ErrorObservers, noopErrorObserver{})
		}
	}

	return func(rc *ops.RootConfig) {
		observer := &aggregateObserver{handler: h.handler}
		rc.LifecycleObservers = append(rc.LifecycleObservers, observer)
		rc.ErrorObservers = append(rc.ErrorObservers, observer)
	}
}

type noopLifecycleObserver struct{}

func (o noopLifecycleObserver) OnStart(ctx context.Context, op *ops.Operation) context.Context {
	return ctx
}

func (o noopLifecycleObserver) OnEnd(ctx context.Context, op *ops.Operation) context.Context {
	return ctx
}

type noopErrorObserver struct{}

func (o noopErrorObserver) OnError(ctx context.Context, op *ops.Operation, err error) {
}

type contextKey string

const aggregateStateKey contextKey = "go-wide.wide.aggregate_state"

type aggregateState struct {
	mu     sync.Mutex
	start  time.Time
	status string
	logs   []collectedRecord
}

type collectedRecord struct {
	time    time.Time
	level   slog.Level
	message string
	attrs   []slog.Attr
}

func aggregateStateFromContext(ctx context.Context) *aggregateState {
	state, ok := ctx.Value(aggregateStateKey).(*aggregateState)
	if state == nil || !ok {
		return nil
	}
	return state
}

func (s *aggregateState) collect(r slog.Record) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logs = append(s.logs, collectedRecord{
		time:    r.Time,
		level:   r.Level,
		message: r.Message,
		attrs:   attrsFromRecord(r),
	})
}

func attrsFromRecord(r slog.Record) []slog.Attr {
	var attrs []slog.Attr
	r.Attrs(func(attr slog.Attr) bool {
		attrs = append(attrs, attr)
		return true
	})
	return attrs
}

func (s *aggregateState) finalRecord(op *ops.Operation) slog.Record {
	s.mu.Lock()
	defer s.mu.Unlock()

	end := time.Now()
	record := slog.NewRecord(end, slog.LevelInfo, op.Op, 0)
	record.AddAttrs(op.Attrs()...)
	record.AddAttrs(
		slog.Int("version", 1),
		slog.String("status", s.status),
		slog.Time("start", s.start),
		slog.Time("end", end),
		slog.GroupAttrs("logs", s.logAttrs()...),
	)
	return record
}

func (s *aggregateState) logAttrs() []slog.Attr {
	attrs := make([]slog.Attr, len(s.logs))
	for i, entry := range s.logs {
		numAttrs := len(entry.attrs) + 3
		entryAttrs := make([]slog.Attr, numAttrs)
		entryAttrs[0] = slog.String("message", entry.message)
		entryAttrs[1] = slog.String("level", entry.level.String())
		entryAttrs[2] = slog.Time("time", entry.time)
		copy(entryAttrs[3:], entry.attrs)
		attrs[i] = slog.GroupAttrs(fmt.Sprintf("%d", i), entryAttrs...)
	}
	return attrs
}

func (s *aggregateState) markError() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = "error"
}

type aggregateObserver struct {
	handler slog.Handler
}

func (o *aggregateObserver) OnStart(ctx context.Context, op *ops.Operation) context.Context {
	state := &aggregateState{
		start:  time.Now(),
		status: "ok",
	}
	return context.WithValue(ctx, aggregateStateKey, state)
}

func (o *aggregateObserver) OnEnd(ctx context.Context, op *ops.Operation) context.Context {
	state := aggregateStateFromContext(ctx)
	if state == nil {
		return ctx
	}

	record := state.finalRecord(op)
	_ = o.handler.Handle(ctx, record)
	return ctx
}

func (o *aggregateObserver) OnError(ctx context.Context, op *ops.Operation, err error) {
	if err == nil {
		return
	}

	state := aggregateStateFromContext(ctx)
	if state == nil {
		return
	}

	state.markError()
}
