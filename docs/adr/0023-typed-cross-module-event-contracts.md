# ADR-0023: Typed Cross-Module Event Contracts

**Status:** Accepted  
**Date:** 2026-05-22  
**Deciders:** Stefan Moseler  

## Context

Vakt modules emit compliance-relevant events to Vakt Comply (secvitals) via an Asynq background queue. Before this ADR, all three emitting modules (secvault, secprivacy, secreflex) built `crossevidence.EvidencePayload` structs by hand, using unvalidated `string` fields for `Source` and `ResourceType`. This led to:

- **Typo risk**: `"vakt-aware/training-completion"` vs `"vakt-aware/training-completed"` — silent divergence
- **No discovery**: no central list of which events exist across the platform
- **No type checking**: renaming a source requires grep across packages

## Decision

Introduce `internal/shared/platform/events` as the canonical event vocabulary for Vakt. The package provides:

1. **Constants** for `Source` (which module) and `ResourceType` (which event)
2. **Typed constructors** (`FindingCreated`, `BreachNotified`, `DSRCompleted`, `SecretRotated`, `TrainingCompleted`, `IncidentCreated`) returning a `CrossModuleEvent` value
3. **`EvidencePayload` type alias**: `crossevidence.EvidencePayload = events.CrossModuleEvent` ensures backward compatibility with the Asynq worker

Module callers replace raw struct literals with typed constructor calls:

```go
// Before
crossevidence.EvidencePayload{OrgID: orgID, Source: "secvault", ResourceType: "vakt-vault/secret-rotated", ...}

// After
events.SecretRotated(orgID, secretKey)
```

## Consequences

- Adding a new event type requires one typed constructor in `platform/events` — discoverable and reviewable in one place
- The Asynq worker handler (`cmd/worker/handlers.go`) is unchanged — `EvidencePayload` is wire-format compatible
- `ProgressEvent` (scan progress SSE) remains in `secpulse` — it is not a compliance event and uses a different transport (Redis Pub/Sub, not Asynq)
- Future: IncidentCreated and FindingCreated constructors are ready; callers in secvitals and secpulse can adopt them when those create-paths are refactored
