package wide

import "log/slog"

type rawNode struct {
	name     string
	attrs    []slog.Attr
	logs     []LogEntry
	children []*rawNode
	status   string
	errMsg   string
}

type normalizedSnapshotNode struct {
	name     string
	count    int
	shared   map[string]slog.Value
	variants map[string][]slog.Value
	logs     []collectedRecord
	children map[string]*normalizedSnapshotNode
}

type OperationSnapshot struct {
	Name     string
	Attrs    []slog.Attr
	Logs     []LogEntry
	Children []OperationSnapshot
	Status   string
	Error    string
}

func NormalizeSnapshot(snapshot OperationSnapshot) []slog.Attr {
	node := buildNormalizedSnapshotNode(snapshot, true)
	return renderNormalizedSnapshotNode(node, true)
}

func buildNormalizedSnapshotNode(snapshot OperationSnapshot, isRoot bool) *normalizedSnapshotNode {
	node := &normalizedSnapshotNode{
		name:     snapshot.Name,
		count:    1,
		shared:   map[string]slog.Value{},
		variants: map[string][]slog.Value{},
		children: map[string]*normalizedSnapshotNode{},
	}

	allAttrs := cloneAttrs(snapshot.Attrs)
	if !isRoot {
		allAttrs = append(allAttrs, slog.String("status", snapshot.Status))
		if snapshot.Error != "" {
			allAttrs = append(allAttrs, slog.String("error", snapshot.Error))
		}
	}

	node.shared = flattenAttrs(allAttrs)
	node.logs = collectedRecordsFromLogEntries(snapshot.Logs)

	for _, child := range snapshot.Children {
		next := buildNormalizedSnapshotNode(child, false)

		existing := node.children[child.Name]
		if existing == nil {
			node.children[child.Name] = next
			continue
		}

		mergeNormalizedSnapshotNodes(existing, next)
	}

	return node
}

func mergeNormalizedSnapshotNodes(dst, src *normalizedSnapshotNode) {
	dst.count += src.count

	mergeFlatValues(dst.shared, dst.variants, cloneValueMap(src.shared))

	for path, values := range src.variants {
		for _, value := range values {
			dst.variants[path] = appendUniqueValue(dst.variants[path], value)
		}
		delete(dst.shared, path)
	}

	dst.logs = append(dst.logs, src.logs...)

	for name, child := range src.children {
		existing := dst.children[name]
		if existing == nil {
			dst.children[name] = child
			continue
		}
		mergeNormalizedSnapshotNodes(existing, child)
	}
}

func collectedRecordsFromLogEntries(entries []LogEntry) []collectedRecord {
	if len(entries) == 0 {
		return nil
	}

	out := make([]collectedRecord, len(entries))
	for i, entry := range entries {
		out[i] = collectedRecord{
			level:   entry.Level,
			message: entry.Message,
			attrs:   cloneAttrs(entry.Attrs),
		}
	}
	return out
}

func renderNormalizedSnapshotNode(node *normalizedSnapshotNode, isRoot bool) []slog.Attr {
	var attrs []slog.Attr

	if !isRoot && node.count > 1 {
		attrs = append(attrs, slog.Int("count", node.count))
	}

	if len(node.shared) > 0 {
		attrs = append(attrs, renderNestedValueMap(node.shared)...)
	}

	if len(node.variants) > 0 {
		attrs = append(attrs, slog.GroupAttrs("variants", renderNestedVariants(node.variants)...))
	}

	attrs = append(attrs, renderNormalizedChildren(node.children)...)

	if len(node.logs) > 0 {
		attrs = append(attrs, slog.GroupAttrs("logs", logAttrs(node.logs)...))
	}

	return attrs
}

func renderNormalizedChildren(children map[string]*normalizedSnapshotNode) []slog.Attr {
	var out []slog.Attr
	for name, child := range children {
		out = append(out, slog.GroupAttrs(name, renderNormalizedSnapshotNode(child, false)...))
	}
	return out
}
