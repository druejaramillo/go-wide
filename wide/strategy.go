package wide

import "log/slog"

type Strategy interface {
	Collect(entry LogEntry)
	Flush(snapshot OperationSnapshot) slog.Record
}

func WithStrategy(strategy Strategy) Option {
	return func(cfg *handlerConfig) {
		cfg.strategy = strategyCustom
		cfg.customStrategy = strategy
	}
}
