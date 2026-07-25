# ADR 0002: Layered, Opt-in Architecture for AI-Assisted Remediation

## Status

Proposed

## Context

OpenFero's value proposition today rests on being a small, predictable,
auditable bridge between Prometheus/Alertmanager and Kubernetes Jobs: an
alert comes in, it is matched deterministically against an `Operarius` CRD's
`AlertSelector`, and a Job is created (`pkg/services/operarius.go`,
`FindMatchingOperarius`, `CreateJobFromOperarius`). There is currently no
AI/LLM code anywhere in the repository; the only trace is a wishlist bullet
in `docs/operarius-development-guide.md` ("Machine learning for remediation
suggestions").

For 2026, the goal is to give AI a real, meaningful role — root-cause
suggestions and AI-generated remediation proposals — without losing what
makes OpenFero valuable: an operator can read an `Operarius` CRD and know
*exactly* what will happen when an alert fires, with no hidden model
inference deciding whether or how to act. A remediation orchestrator that
occasionally takes the wrong automatic action due to a model's mistake is
worse than one that does nothing, because remediation Jobs have real
side effects (restarting pods, scaling deployments, running scripts).

This ADR is about *how* AI capabilities are integrated architecturally,
not about which LLM provider or model is used (that is an implementation
detail behind an interface, not an architectural decision).

## Decision

AI is added as **additive, optional layers wrapped around the existing
deterministic core**, never as a replacement for it, and never with the
ability to directly execute an unreviewed action.

### Layer 0 — Deterministic core (unchanged)

The existing `AlertSelector` → `Operarius` → Job flow remains the default
and must keep working with zero AI configuration. No existing behavior
changes. This is the load-bearing guarantee that lets simple users ignore
AI entirely.

### Layer 1 — Read-only root-cause enrichment (opt-in, fail-open)

An optional advisory step runs alongside job creation: it asks an LLM
(via the provider abstraction described below) for likely root cause /
context, using the alert's labels/annotations and relevant records from the
knowledge base (ADR 0001). The result is attached as metadata (e.g. a
Job annotation and/or a field on the corresponding knowledge base record)
for humans to read in the UI — it does not change which Job runs or how.

Fail-open by design: if the AI provider is slow, unreachable, or returns an
error, remediation proceeds exactly as it would with no AI configured. A
timeout budget keeps this from ever delaying Job creation materially.

### Layer 2 — AI-generated remediation proposals (human-in-the-loop)

For alerts that recur without a matching `Operarius`, or where the
knowledge base shows a pattern, an optional process can propose a *new*
`Operarius` definition (alert selector + job template) generated from
historical context. It is written out **disabled** — `Operarius` already
has an `Enabled *bool` field (`api/v1alpha1/operarius_types.go`) used
exactly for this kind of "exists but not active" state. A human reviews and
explicitly flips it on. There is no code path in which an AI-authored
`Operarius` becomes active without a human action.

### Layer 3 — Knowledge base (shared foundation)

Both layers above are only useful with historical context. This is
addressed in ADR 0001 and is treated as a dependency of this ADR, not a
separate concern.

### Cross-cutting: provider abstraction

A minimal Go interface (e.g. `ai.Advisor`) abstracts the LLM call itself.
The default implementation, when no provider is configured, is a no-op that
Layer 1/2 treat identically to a fail-open timeout. This keeps the "simple"
deployment story exactly as simple as it is today: no API key configured →
no AI dependency compiled into the runtime path, no behavior change.
Configuration follows the existing pattern of plain CLI flags
(`main.go`), not a new configuration framework.

### Cross-cutting: auditability

Anything produced by Layer 1 or 2 is clearly and permanently marked as
AI-originated (e.g. `openfero.io/suggested-by: ai`,
`openfero.io/human-approved: "false"` on generated `Operarius` objects, and
an equivalent marker on enrichment annotations). Nothing AI-produced is
ever presented indistinguishably from human-authored configuration.

### Sequencing

Layer 1 ships first in isolation: lowest risk (read-only), immediate value,
and it validates whether AI-assisted context is actually useful in practice
before Layer 2 (which touches what remediation actions can exist) is
attempted. This avoids building a "full-blown AI orchestrator" speculatively
before there is evidence the simpler enrichment layer earns its keep.

## Consequences

**Positive:**
- Existing users and existing `Operarius` definitions are entirely
  unaffected until they opt in.
- Blast radius of AI mistakes is bounded: Layer 1 can at worst show a
  misleading suggestion in the UI; Layer 2 can at worst create a disabled
  CRD object nobody activates. Neither can trigger an unreviewed Job.
  Layer 0 (actual Job execution) is never touched by AI at all.
- The provider-abstraction + fail-open design means the failure mode of
  "LLM API is down" degrades to "OpenFero behaves like it does today,"
  not an outage.
- Matches the project's existing extensibility patterns (interface +
  pluggable implementation, e.g. `alertstore.Store`), so it is consistent
  with how the codebase already grows.

**Negative / trade-offs:**
- No fully autonomous "AI decides and acts" mode is provided by this
  design. Teams that want a true closed-loop AI remediation orchestrator
  will find this deliberately conservative. That is an explicit trade-off
  in favor of safety and auditability over automation ambition.
- Two new optional subsystems (advisor call path, proposal generation)
  add surface area — more configuration flags, more code paths to test —
  even though they are inert by default. This complexity cost is accepted
  because it is opt-in rather than paid by every user.
- Human review in Layer 2 is a hard dependency for the proposal path to
  produce value; if nobody reviews proposals, that layer's output is
  inert. This is intentional (see "Positive," blast radius) but worth
  naming as a real adoption cost.

## Alternatives Considered

- **Autonomous AI orchestrator** (AI directly authors and enables
  remediation, or dynamically decides which Job to run at alert time
  instead of the deterministic `AlertSelector` match). Rejected for now:
  removes the auditability guarantee that is core to OpenFero's identity,
  and turns an infrastructure tool with real side effects into a system
  where the actual decision logic is opaque and non-deterministic.
- **Do nothing / status quo.** Rejected per the user's explicit goal of
  giving AI a real role in OpenFero's future; kept as the implicit
  "Layer 0 only" state for anyone who doesn't configure a provider.
- **AI as a fully separate, out-of-process sidecar service** consuming
  OpenFero's API rather than integrated into the core binary. Considered
  viable but not chosen as the primary design here because it would
  duplicate matching/context logic that already lives in
  `pkg/services/operarius.go`; may be revisited if operational isolation
  of the AI component becomes a stronger requirement than code reuse.

## References

- `pkg/services/operarius.go` (`FindMatchingOperarius`,
  `CreateJobFromOperarius`)
- `api/v1alpha1/operarius_types.go` (`Enabled *bool`)
- `main.go` (CLI flag configuration pattern)
- `docs/operarius-development-guide.md` (existing "Machine learning for
  remediation suggestions" roadmap item)
- ADR 0001: Persistence Strategy for an AI Remediation Knowledge Base
