/*
Package wide provides a handler for the slog.Handler interface that translates
slog groups into log messages.
*/
package wide

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/druejaramillo/go-wide/ops"
)

type handlerStrategy string

const (
	strategyPassthrough handlerStrategy = "passthrough"
	strategyAggregate   handlerStrategy = "aggregate"
	strategyCustom      handlerStrategy = "custom"
)

type handlerConfig struct {
	strategy       handlerStrategy
	customStrategy Strategy
	aggregateLimit int
}

type Option func(*handlerConfig)

func WithAggregate() Option {
	return func(cfg *handlerConfig) {
		cfg.strategy = strategyAggregate
	}
}

func WithAggregateLimit(limit int) Option {
	return func(cfg *handlerConfig) {
		cfg.aggregateLimit = limit
	}
}

type Handler struct {
	handler     slog.Handler // ordinary downstream handler
	rootHandler slog.Handler // stable handler for root emission only
	config      *handlerConfig
	prefix      []slog.Attr
	groups      []string
}

func NewHandler(h slog.Handler, opts ...Option) *Handler {
	cfg := &handlerConfig{
		strategy: strategyPassthrough,
	}
	for _, opt := range opts {
		opt(cfg)
	}
	return &Handler{
		handler:     h,
		rootHandler: h,
		config:      cfg,
	}
}

func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	if h.config.strategy == strategyPassthrough {
		return h.handler.Enabled(ctx, level)
	}

	state := aggregateStateFromContext(ctx)
	node := aggregateNodeFromContext(ctx)
	op := ops.GetOperationFromContext(ctx)
	if state != nil && node != nil && op != nil && !op.IsEnded() && !state.isOverflowed() {
		return true
	}

	return h.handler.Enabled(ctx, level)
}

func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	if h.config.strategy == strategyPassthrough {
		return h.handler.Handle(ctx, r)
	}

	state := aggregateStateFromContext(ctx)
	node := aggregateNodeFromContext(ctx)
	op := ops.GetOperationFromContext(ctx)
	if state == nil || node == nil || op == nil || op.IsEnded() || state.isOverflowed() {
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

	if h.config.strategy == strategyCustom {
		entry := LogEntry{
			Level:   effectiveRecord.Level,
			Message: effectiveRecord.Message,
			Attrs:   attrsFromRecord(effectiveRecord),
		}
		h.config.customStrategy.Collect(entry)
		state.collectSnapshotLog(node, entry)
		return nil
	}

	if overflowed := state.collect(node, effectiveRecord); overflowed {
		diagnostic := slog.NewRecord(time.Now(), slog.LevelWarn, "wide aggregate overflow", 0)
		diagnostic.AddAttrs(
			slog.String("reason", "limit_exceeded"),
			slog.Int("limit", state.limit),
		)
		return h.handler.Handle(ctx, diagnostic)
	}

	return nil
}

func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &Handler{
		handler:     h.handler.WithAttrs(attrs),
		rootHandler: h.rootHandler,
		config:      h.config,
		prefix:      mergeAttrsIntoGroupPath(cloneAttrs(h.prefix), h.groups, cloneAttrs(attrs)),
		groups:      append([]string(nil), h.groups...),
	}
}

func (h *Handler) WithGroup(name string) slog.Handler {
	return &Handler{
		handler:     h.handler.WithGroup(name),
		rootHandler: h.rootHandler,
		config:      h.config,
		prefix:      cloneAttrs(h.prefix),
		groups:      append(append([]string(nil), h.groups...), name),
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
	if h.config.strategy == strategyPassthrough {
		return func(rc *ops.RootConfig) {
			rc.LifecycleObservers = append(rc.LifecycleObservers, noopLifecycleObserver{})
			rc.ErrorObservers = append(rc.ErrorObservers, noopErrorObserver{})
		}
	}

	return func(rc *ops.RootConfig) {
		if h.config.aggregateLimit < 0 {
			ops.SetOptionError(rc, fmt.Errorf("%w: aggregate limit must be >= 0", ops.ErrInvalidOptionUsage))
			return
		}

		observer := &aggregateObserver{
			handler:  h.rootHandler,
			limit:    h.config.aggregateLimit,
			strategy: h.config.customStrategy,
		}
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

const (
	aggregateStateKey contextKey = "go-wide.wide.aggregate_state"
	aggregateNodeKey  contextKey = "go-wide.wide.aggregate_node"
)

type aggregateState struct {
	mu         sync.Mutex
	start      time.Time
	status     string
	root       *aggregateNode
	limit      int
	collected  int
	overflowed bool
}

type aggregateNode struct {
	name         string
	count        int
	shared       map[string]slog.Value
	variants     map[string][]slog.Value
	logs         []collectedRecord
	snapshotLogs []LogEntry
	children     map[string]*aggregateNode
	status       string
	errMsg       string
}

type collectedRecord struct {
	time    time.Time
	level   slog.Level
	message string
	attrs   []slog.Attr
}

type logIdentity struct {
	level   slog.Level
	message string
	source  string
}

type logBucket struct {
	identity logIdentity
	count    int
	shared   map[string]slog.Value
	variants map[string][]slog.Value
}

func aggregateStateFromContext(ctx context.Context) *aggregateState {
	state, ok := ctx.Value(aggregateStateKey).(*aggregateState)
	if state == nil || !ok {
		return nil
	}
	return state
}

func aggregateNodeFromContext(ctx context.Context) *aggregateNode {
	state, ok := ctx.Value(aggregateNodeKey).(*aggregateNode)
	if state == nil || !ok {
		return nil
	}
	return state
}

func (s *aggregateState) isOverflowed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.overflowed
}

func (s *aggregateState) collect(node *aggregateNode, r slog.Record) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.overflowed {
		return false
	}

	if s.limit > 0 && s.collected >= s.limit {
		s.overflowed = true
		s.collected = 0
		s.root.shared = nil
		s.root.variants = nil
		s.root.logs = nil
		s.root.children = map[string]*aggregateNode{}
		return true
	}

	node.logs = append(node.logs, collectedRecord{
		time:    r.Time,
		level:   r.Level,
		message: r.Message,
		attrs:   attrsFromRecord(r),
	})
	s.collected++
	return false
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

	if len(s.root.shared) > 0 {
		record.AddAttrs(renderNestedValueMap(s.root.shared)...)
	}
	if len(s.root.variants) > 0 {
		record.AddAttrs(slog.GroupAttrs("variants", renderNestedVariants(s.root.variants)...))
	}

	record.AddAttrs(
		slog.Int("version", 1),
		slog.String("status", s.status),
		slog.Time("start", s.start),
		slog.Time("end", end),
	)

	for _, attr := range renderChildGroups(s.root) {
		record.AddAttrs(attr)
	}

	if len(s.root.logs) > 0 {
		record.AddAttrs(slog.GroupAttrs("logs", logAttrs(s.root.logs)...))
	}

	return record
}

func renderChildGroups(node *aggregateNode) []slog.Attr {
	var out []slog.Attr
	for name, child := range node.children {
		out = append(out, slog.GroupAttrs(name, renderNodeBody(child)...))
	}
	return out
}

func renderNodeBody(node *aggregateNode) []slog.Attr {
	var out []slog.Attr

	if node.count > 1 {
		out = append(out, slog.Int("count", node.count))
	}

	if len(node.shared) > 0 {
		out = append(out, renderNestedValueMap(node.shared)...)
	}

	if len(node.variants) > 0 {
		out = append(out, slog.GroupAttrs("variants", renderNestedVariants(node.variants)...))
	}

	out = append(out, renderChildGroups(node)...)

	if len(node.logs) > 0 {
		out = append(out, slog.GroupAttrs("logs", logAttrs(node.logs)...))
	}

	return out
}

func logAttrs(entries []collectedRecord) []slog.Attr {
	buckets := normalizeLogs(entries)
	attrs := make([]slog.Attr, len(buckets))
	for i, bucket := range buckets {
		attrs[i] = slog.GroupAttrs(bucket.renderKey(), renderLogBucket(bucket)...)
	}
	return attrs
}

func normalizeLogs(entries []collectedRecord) []*logBucket {
	order := make([]logIdentity, 0)
	buckets := make(map[logIdentity]*logBucket)

	for _, entry := range entries {
		identity := identityFor(entry)
		bucket := buckets[identity]
		if bucket == nil {
			bucket = &logBucket{
				identity: identity,
				count:    0,
				shared:   flattenAttrs(entry.attrs),
				variants: map[string][]slog.Value{},
			}
			buckets[identity] = bucket
			order = append(order, identity)
		}
		bucket.merge(entry)
	}

	out := make([]*logBucket, len(order))
	for i, identity := range order {
		out[i] = buckets[identity]
	}
	return out
}

func identityFor(entry collectedRecord) logIdentity {
	return logIdentity{
		level:   entry.level,
		message: entry.message,
		source:  sourceFromAttrs(entry.attrs),
	}
}

func sourceFromAttrs(attrs []slog.Attr) string {
	return ""
}

func flattenAttrs(attrs []slog.Attr) map[string]slog.Value {
	out := map[string]slog.Value{}
	for _, attr := range attrs {
		flattenAttrInto(out, "", attr)
	}
	return out
}

func flattenAttrInto(out map[string]slog.Value, prefix string, attr slog.Attr) {
	value := attr.Value.Resolve()

	key := attr.Key
	if prefix != "" {
		key = prefix + "." + key
	}

	if value.Kind() != slog.KindGroup {
		out[key] = value
		return
	}

	for _, child := range value.Group() {
		flattenAttrInto(out, key, child)
	}
}

func (b *logBucket) merge(entry collectedRecord) {
	b.count++

	next := flattenAttrs(entry.attrs)

	if b.count == 1 {
		return
	}

	mergeFlatValues(b.shared, b.variants, next)
}

func cloneValueMap(in map[string]slog.Value) map[string]slog.Value {
	out := make(map[string]slog.Value, len(in))
	maps.Copy(out, in)
	return out
}

func appendUniqueValue(values []slog.Value, next slog.Value) []slog.Value {
	for _, existing := range values {
		if slogValuesEqual(existing, next) {
			return values
		}
	}
	return append(values, next)
}

func slogValuesEqual(a, b slog.Value) bool {
	return reflect.DeepEqual(normalizeValue(a.Resolve()), normalizeValue(b.Resolve()))
}

func normalizeValue(value slog.Value) any {
	switch value.Kind() {
	case slog.KindGroup:
		group := map[string]any{}
		for _, attr := range value.Group() {
			group[attr.Key] = normalizeValue(attr.Value)
		}
		return group
	default:
		return value.Any()
	}
}

func renderLogBucket(bucket *logBucket) []slog.Attr {
	attrs := []slog.Attr{
		slog.String("message", bucket.identity.message),
		slog.String("level", bucket.identity.level.String()),
		slog.Int("count", bucket.count),
	}

	attrs = append(attrs, renderNestedValueMap(bucket.shared)...)

	if len(bucket.variants) > 0 {
		attrs = append(attrs, slog.GroupAttrs("variants", renderNestedVariants(bucket.variants)...))
	}

	return attrs
}

func renderNestedValueMap(flat map[string]slog.Value) []slog.Attr {
	tree := map[string]any{}
	for path, value := range flat {
		insertPath(tree, path, normalizeValue(value))
	}
	return renderTree(tree)
}

func renderNestedVariants(flat map[string][]slog.Value) []slog.Attr {
	tree := map[string]any{}
	for path, values := range flat {
		list := make([]any, len(values))
		for i, value := range values {
			list[i] = normalizeValue(value)
		}
		insertPath(tree, path, list)
	}
	return renderTree(tree)
}

func insertPath(tree map[string]any, path string, value any) {
	parts := strings.Split(path, ".")
	node := tree

	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]

		child, ok := node[part].(map[string]any)
		if !ok {
			child = map[string]any{}
			node[part] = child
		}
		node = child
	}

	node[parts[len(parts)-1]] = value
}

func renderTree(tree map[string]any) []slog.Attr {
	keys := make([]string, 0, len(tree))
	for key := range tree {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	attrs := make([]slog.Attr, len(keys))
	for i, key := range keys {
		attrs[i] = renderTreeValue(key, tree[key])
	}
	return attrs
}

func renderTreeValue(key string, value any) slog.Attr {
	if group, ok := value.(map[string]any); ok {
		return slog.GroupAttrs(key, renderTree(group)...)
	}
	return slog.Any(key, value)
}

func (b *logBucket) renderKey() string {
	return b.identity.message
}

func (s *aggregateState) markError() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = "error"
}

type aggregateObserver struct {
	handler  slog.Handler
	limit    int
	strategy Strategy
}

func (o *aggregateObserver) OnStart(ctx context.Context, op *ops.Operation) context.Context {
	state := aggregateStateFromContext(ctx)
	if state == nil {
		root := &aggregateNode{
			name:     op.Op,
			children: map[string]*aggregateNode{},
		}
		state := &aggregateState{
			start:  time.Now(),
			status: "ok",
			root:   root,
			limit:  o.limit,
		}
		ctx = context.WithValue(ctx, aggregateStateKey, state)
		return context.WithValue(ctx, aggregateNodeKey, root)
	}
	parent := aggregateNodeFromContext(ctx)
	child := state.childNode(parent, op.Op)
	return context.WithValue(ctx, aggregateNodeKey, child)
}

func (s *aggregateState) childNode(parent *aggregateNode, name string) *aggregateNode {
	s.mu.Lock()
	defer s.mu.Unlock()

	if parent.children == nil {
		parent.children = map[string]*aggregateNode{}
	}

	child := parent.children[name]
	if child == nil {
		child = &aggregateNode{
			name:     name,
			count:    1,
			status:   "ok",
			variants: map[string][]slog.Value{},
			children: map[string]*aggregateNode{},
		}
		parent.children[name] = child
		return child
	}

	child.count++
	child.status = "ok"
	child.errMsg = ""
	return child
}

func (s *aggregateState) markNodeError(node *aggregateNode, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	node.status = "error"
	node.errMsg = err.Error()
}

func (o *aggregateObserver) OnEnd(ctx context.Context, op *ops.Operation) context.Context {
	state := aggregateStateFromContext(ctx)
	node := aggregateNodeFromContext(ctx)
	if state == nil || node == nil {
		return ctx
	}

	if state.isOverflowed() {
		return ctx
	}

	state.setNodeAttrs(node, op.Attrs())

	if len(op.Ops()) != 1 {
		return ctx
	}

	var record slog.Record
	if o.strategy != nil {
		record = o.strategy.Flush(state.snapshot(op))
	} else {
		record = state.finalRecord(op)
	}

	if err := o.handler.Handle(ctx, record); err != nil {
		return ops.WithEndError(ctx, err)
	}

	return ctx
}

func (s *aggregateState) setNodeAttrs(node *aggregateNode, attrs []slog.Attr) {
	s.mu.Lock()
	defer s.mu.Unlock()

	allAttrs := cloneAttrs(attrs)

	if node != s.root {
		allAttrs = append(allAttrs, slog.String("status", node.status))
		if node.errMsg != "" {
			allAttrs = append(allAttrs, slog.String("error", node.errMsg))
		}
	}

	next := flattenAttrs(allAttrs)

	if node.shared == nil {
		node.shared = next
		if node.variants == nil {
			node.variants = map[string][]slog.Value{}
		}
		return
	}

	mergeFlatValues(node.shared, node.variants, next)
}

func mergeFlatValues(shared map[string]slog.Value, variants map[string][]slog.Value, next map[string]slog.Value) {
	for path, sharedValue := range cloneValueMap(shared) {
		nextValue, ok := next[path]
		if ok && slogValuesEqual(sharedValue, nextValue) {
			delete(next, path)
			continue
		}

		variants[path] = appendUniqueValue(variants[path], sharedValue)
		if ok {
			variants[path] = appendUniqueValue(variants[path], nextValue)
			delete(next, path)
		}
		delete(shared, path)
	}

	for path, nextValue := range next {
		if _, alreadyVariant := variants[path]; alreadyVariant {
			variants[path] = appendUniqueValue(variants[path], nextValue)
			continue
		}

		if _, stillShared := shared[path]; stillShared {
			continue
		}

		variants[path] = appendUniqueValue(nil, nextValue)
	}
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

	node := aggregateNodeFromContext(ctx)
	if node == nil {
		return
	}

	state.markNodeError(node, err)
}

func (s *aggregateState) collectSnapshotLog(node *aggregateNode, entry LogEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	node.snapshotLogs = append(node.snapshotLogs, LogEntry{
		Level:   entry.Level,
		Message: entry.Message,
		Attrs:   cloneAttrs(entry.Attrs),
	})
}

func (s *aggregateState) snapshot(op *ops.Operation) OperationSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	return snapshotFromNode(s.root, op.Op, true)
}

func snapshotFromNode(node *aggregateNode, name string, isRoot bool) OperationSnapshot {
	snapshot := OperationSnapshot{
		Name: name,
		Logs: cloneLogEntries(node.snapshotLogs),
	}

	if len(node.shared) > 0 {
		snapshot.Attrs = append(snapshot.Attrs, renderNestedValueMap(node.shared)...)
	}
	if len(node.variants) > 0 {
		snapshot.Attrs = append(snapshot.Attrs, slog.GroupAttrs("variants", renderNestedVariants(node.variants)...))
	}

	if !isRoot {
		snapshot.Status = node.status
		snapshot.Error = node.errMsg
		if node.count > 1 {
			snapshot.Attrs = append(snapshot.Attrs, slog.Int("count", node.count))
		}
	}

	for name, child := range node.children {
		snapshot.Children = append(snapshot.Children, snapshotFromNode(child, name, false))
	}

	return snapshot
}

func cloneLogEntries(in []LogEntry) []LogEntry {
	if len(in) == 0 {
		return nil
	}
	out := make([]LogEntry, len(in))
	for i, entry := range in {
		out[i] = LogEntry{
			Level:   entry.Level,
			Message: entry.Message,
			Attrs:   cloneAttrs(entry.Attrs),
		}
	}
	return out
}
