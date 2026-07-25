# ADR 0001: Persistence Strategy for an AI Remediation Knowledge Base

## Status

Proposed

## Context

OpenFero is evolving from a purely rule-based remediation tool (Alert →
`Operarius` CRD match → Kubernetes Job) towards optionally including
AI-assisted capabilities: root-cause suggestions and AI-generated remediation
proposals (see ADR 0002). Both capabilities need access to a history of past
incidents and their outcomes to be useful — a "knowledge base" rather than a
one-shot LLM call with no context.

Today, OpenFero already has two persistence-adjacent mechanisms, and it is
important to be precise about what each one actually does before deciding
where a knowledge base should live:

1. **`Operarius` CRD status (`pkg/services/operarius.go`,
   `CheckDeduplication` / `UpdateOperariusDedupStatus`)** — this is the
   source of truth for "has this alert already been remediated". It is
   stored as a Kubernetes status subresource and is therefore durable
   (backed by etcd) and survives pod restarts.

2. **memberlist alert store (`pkg/alertstore/memberlist/memberlist.go`)** —
   this is *not* a durable store. It is a bounded, in-memory slice
   (`alertEntry`, capped at `-alertStoreSize`, default 10, max 100 if unset)
   guarded by a mutex and gossiped between replicas via HashiCorp
   memberlist purely so that every OpenFero replica's `/api/alerts` and
   WebSocket feed shows the same "recent activity" view. On restart of all
   replicas, or in a single-replica deployment, this history is gone
   (`Initialize()` always starts from an empty slice). It was designed
   exclusively to solve a UI-consistency problem across replicas, not to
   retain history.

Using memberlist as the foundation for an AI knowledge base would be a
misuse of a mechanism that was deliberately built to be small and volatile.
A knowledge base needs to survive restarts, needs to hold more than
10–100 entries, and needs to be queryable by more than "linear scan with
substring filter".

At the same time, the project's operating philosophy is to remain
Kubernetes-native and easy to run: a single binary, no mandatory external
stateful dependency (no required Postgres/Redis/vector database). Any
persistence decision for the knowledge base must respect that constraint
unless a future ADR explicitly revisits it.

## Decision

We will persist the AI knowledge base using the same Kubernetes-native
pattern already used for deduplication state — **no new infrastructure
dependency is introduced**.

1. **Source of truth: Kubernetes CRDs/etcd.** Structured incident records
   (alert, matched `Operarius`, job outcome, optional AI-generated
   root-cause annotation, human feedback/approval state) are stored as
   Kubernetes status data, either as an extension of `Operarius.Status` or
   as a dedicated new CRD (e.g. `RemediationRecord`) referencing the
   `Operarius` that handled it. This mirrors the existing dedup mechanism
   in `pkg/services/operarius.go` and requires no new operational
   component — it is durable "for free" because etcd already is.

2. **Bounded, not unbounded.** Like the current memberlist cap, the
   CRD-based history keeps a bounded window per `Operarius` (configurable,
   analogous to `-alertStoreSize`). This respects etcd's known constraints
   (object size limits, cost of watching/listing many objects at scale) and
   avoids turning OpenFero into a general-purpose time-series store. Bulk,
   long-term history (e.g. full job logs) is explicitly left to
   infrastructure the cluster operator already runs (log aggregation,
   Job pod logs), not owned by OpenFero.

3. **Structured retrieval instead of vector search, initially.** Given a
   typical cluster has a low-to-mid number of distinct `Operarius`
   definitions, retrieval for AI context can reuse the *same* matching
   primitives already used for alert routing (`alertname`, labels) instead
   of requiring embeddings and a vector database. The last N records for a
   matching `Operarius`/alert pattern are stuffed into the LLM prompt. This
   keeps the "no extra infrastructure" property intact for the default
   installation.

4. **Pluggable escape hatch.** A `KnowledgeStore` interface, modeled after
   the existing `alertstore.Store` interface
   (`pkg/alertstore/alertstore.go`), will define the read/write contract
   for incident records. The default implementation is CRD-backed. Should a
   future need for semantic similarity search across a very large number of
   distinct incident types arise, an alternative implementation (e.g.
   backed by a vector database) can be added *without changing callers*,
   exactly as `memberlist`/`memory` are interchangeable implementations of
   `alertstore.Store` today. Adopting that alternative implementation is a
   deliberate, separate decision an operator opts into — never the default.

5. **memberlist is left untouched.** It continues to serve its original,
   narrow purpose (cross-replica UI cache of recent alert activity) and is
   not repurposed, extended, or relied upon by the AI knowledge base.

## Consequences

**Positive:**
- No new stateful service to operate, monitor, or back up — consistent with
  OpenFero's "just run it on Kubernetes" story.
- Reuses a pattern (CRD status as durable store) already proven in
  production for deduplication.
- The `KnowledgeStore` interface keeps the door open for a vector-backed
  implementation later without an architectural rewrite.
- Auditable: incident records living as Kubernetes objects can be inspected
  with `kubectl`, same as everything else in OpenFero today.

**Negative / trade-offs:**
- etcd is not designed for high-cardinality or high-write-volume history;
  the bounded-window approach limits how much context AI features can draw
  on compared to a purpose-built store. This is an intentional simplicity
  trade-off, not an oversight.
- Structured (label/alertname-based) retrieval is weaker than true semantic
  search — it will miss cross-alert-type similarity (e.g. two differently
  named alerts with a related root cause). Acceptable for the initial
  scope; revisit if AI feature usage shows this is a real limitation.
- Introduces a new CRD (or status extension) that needs its own RBAC,
  CRD versioning, and migration story, similar to what `Operarius` already
  required.

## Alternatives Considered

- **Extend memberlist's cap/retention to serve as the knowledge base.**
  Rejected: still volatile across full-cluster restarts, still an
  awkward fit for structured queries (linear scan/substring only), and
  conflates two different concerns (cheap UI cache vs. durable knowledge).
- **Require an external vector database (e.g. pgvector, Qdrant) from day
  one.** Rejected for the default path: adds a mandatory operational
  dependency that contradicts OpenFero's simplicity goal. Kept as an
  optional future `KnowledgeStore` implementation instead.
- **SQLite on a PersistentVolumeClaim.** Rejected: reintroduces
  single-writer/PVC-affinity constraints in a system designed to run
  multiple stateless replicas, and duplicates durability etcd already
  provides.

## References

- `pkg/alertstore/alertstore.go`, `pkg/alertstore/memberlist/memberlist.go`,
  `pkg/alertstore/memory/memory.go`
- `pkg/services/operarius.go` (`CheckDeduplication`,
  `UpdateOperariusDedupStatus`)
- `api/v1alpha1/operarius_types.go`
- ADR 0002: Layered, Opt-in Architecture for AI-Assisted Remediation
