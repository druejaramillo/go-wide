# Context

## wide-event

A structured aggregate `slog` event representing the summarized state of an operation tree.

**Avoid:** request log, trace dump

## operation

A named lifecycle boundary in `ops` that can own metadata, logs, errors, and child operations.

**Avoid:** scope, group

## root-operation

The topmost operation in a tree; it owns observer registration, wide collection state, and emission policy for all descendants.

**Avoid:** request, transaction

## emission-strategy

A policy that decides whether logging data is emitted record-by-record or as an aggregate at operation completion.

**Avoid:** formatter, exporter

## aggregate-strategy

An emission strategy that collects log entries into an operation tree and emits one wide event when the root operation ends.

**Avoid:** batch logger, buffered logger

## passthrough-strategy

An emission strategy that emits each `slog` record immediately without aggregate collection.

**Avoid:** normal mode

## operation-metadata

Persistent `slog` attributes attached directly to an operation node through `ops` APIs.

**Avoid:** logger attrs, scoped attrs

## divergence-summary

A structural summary of values that differed across merged child operations or merged log buckets.

**Avoid:** timeline, history

## aggregate-limit

A wide handler configuration value that caps how many records a root-owned aggregate collector may buffer before it abandons aggregation for that root lifetime.

**Avoid:** batch size, queue depth

## overflow-diagnostic

An ordinary `slog` record emitted once when aggregate collection exceeds its configured limit; it signals the overflow and the collector then degrades to passthrough for the rest of the root lifetime.

**Avoid:** panic, dropped log
