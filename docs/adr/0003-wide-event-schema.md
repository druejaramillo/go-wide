# 0003 - Wide Event Schema

**Date:** 2026-06-08
**Status:** Accepted

## Context

The aggregate strategy emits a final wide event as a `slog.Record`. That event must be searchable in ordinary logging systems, stay within `slog` semantics, and avoid encodings that depend on chronological slices or custom exporter envelopes.

The final output also needs to preserve debugging signal when repeated child operations or repeated logs diverge.

## Decision

The aggregate strategy emits a single final `slog.Record` when the root operation ends.

The final record is a unified attribute tree:

- root operation metadata becomes top-level attrs
- child operation names become direct groups
- logger group paths merge into the same structural tree
- aggregated logs live under a reserved `logs` child within each branch

The schema uses a fixed reserved vocabulary. Important reserved keys include:

- `version`
- `status`
- `error`
- `start`
- `end`
- `count`
- `failed_children`
- `failed_descendants`
- `logs`
- `variants`

The initial schema version is `1`.

Aggregated log buckets are keyed by synthetic identifiers under `logs`. Their merge identity is:

- logger-derived group path
- level
- message
- source, when available

Same-named child operations merge by operation name. Diverging values are preserved structurally through explicit divergence summaries under `variants`.

Chronological slices are not part of the aggregate schema. If chronology is required, callers should use a passthrough strategy.

Scalar-versus-branch collisions are preserved by promoting the scalar value into a reserved leaf within the branch.

Reserved-key collisions from user attributes are escaped during aggregate rendering.

Finalized aggregate records bypass normal downstream level filtering and are marked to avoid being re-collected recursively.

## Consequences

Wide events are emitted as ordinary `slog.Record` values and remain compatible with downstream handlers.

The schema is shallow and searchable because root attrs stay at top level and child operations become direct groups.

The aggregate format is intentionally structural rather than chronological.

Debugging remains possible because divergence across merged logs and child operations is preserved explicitly instead of being discarded.
