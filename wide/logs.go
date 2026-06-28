package wide

import "log/slog"

type LogEntry struct {
	Level   slog.Level
	Message string
	Attrs   []slog.Attr
}

func NormalizeLogs(entries []LogEntry) []slog.Attr {
	internal := make([]collectedRecord, len(entries))
	for i, entry := range entries {
		internal[i] = collectedRecord{
			level:   entry.Level,
			message: entry.Message,
			attrs:   cloneAttrs(entry.Attrs),
		}
	}
	return logAttrs(internal)
}
