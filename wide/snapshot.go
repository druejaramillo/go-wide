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

type OperationSnapshot struct {
	Name     string
	Attrs    []slog.Attr
	Logs     []LogEntry
	Children []OperationSnapshot
	Status   string
	Error    string
}

func NormalizeSnapshot(snapshot OperationSnapshot) []slog.Attr {
	return normalizeSnapshot(snapshot, true)
}

func normalizeSnapshot(snapshot OperationSnapshot, isRoot bool) []slog.Attr {
	var attrs []slog.Attr

	if len(snapshot.Attrs) > 0 {
		attrs = append(attrs, renderNestedValueMap(flattenAttrs(snapshot.Attrs))...)
	}

	if !isRoot {
		if snapshot.Status != "" {
			attrs = append(attrs, slog.String("status", snapshot.Status))
		}
		if snapshot.Error != "" {
			attrs = append(attrs, slog.String("error", snapshot.Error))
		}
	}

	for _, child := range snapshot.Children {
		attrs = append(attrs, slog.GroupAttrs(child.Name, normalizeSnapshot(child, false)...))
	}

	if len(snapshot.Logs) > 0 {
		attrs = append(attrs, slog.GroupAttrs("logs", NormalizeLogs(snapshot.Logs)...))
	}

	return attrs
}
