# Doctrine — Trust Escalation Safety

> **Status:** CANONICAL v1
> **Scope:** Verification approval, every trust state transition (forward and downgrade), DevOps environment configuration.

## Canonical Wording

> *"Trust escalation must be explicit, auditable, and environment-aware."*

> *"Testing convenience must never become hidden production authority."*

## Core Principle

A trust state transition that grants real-world authority (most importantly: opening payout authority via `approved`) MUST be **attributable** to a real operator with a real reason at a real time. Testing shortcuts MAY exist in non-production environments, but they MUST NOT silently leak into production governance.

## Mandatory Rules

### 1. Hidden production auto-approve is forbidden

A production transition that opens trust authority MUST NOT be triggered by an environment flag, configuration toggle, or background process unless that path is itself attributable, audited, and documented as canonical.

### 2. Production transitions MUST be attributable

Every production trust transition (approve, reject, needs_resubmission, suspend, revoke, under_investigation, lift) MUST record:

- **operator id** — who performed the action;
- **reason** — why (mandatory on negative outcomes and downgrades; recommended on approval);
- **timestamp** — when;
- **audit log entry** — persisted, queryable.

### 3. Sandbox / staging shortcuts MAY exist — with triple guard

A non-production auto-approve flag MAY exist for testing. It MUST be guarded by **all three**:

- **Hard config-time check** — the flag cannot be set in production builds / configs.
- **Runtime alert** — if the flag is somehow active in production, the system raises an alert.
- **Audit log tag** — any transition produced by the flag is tagged distinctly (e.g. `operator=AUTO_DEV_FLAG`) so audit consumers can filter or surface the anomaly.

### 4. Environment-awareness is required

Documentation, configuration, and runtime behavior MUST be explicit about which environment they describe. A transition path that exists in sandbox MUST NOT be implicitly assumed to exist in production.

## Cross-Domain Impacts

- **Verification (KYC)** — every `approved` transition in production is bound by these rules.
- **Trust downgrade** — every `suspended` / `revoked` / `under_investigation` transition is bound by these rules. See [Revocable Trust Model](./revocable-trust-model.md).
- **Audit / Governance** — audit log MUST capture operator + reason + timestamp on every transition.
- **DevOps / Environment** — production configuration MUST exclude any code path that could silently escalate trust without operator attribution.
- **Security** — leakage of a sandbox shortcut into production governance is a security incident, not a configuration drift.

## Forbidden Behaviors (doctrine-level)

- The system MUST NOT contain any production code path that mutates trust state to `approved` without an attributable operator.
- The system MUST NOT permit an auto-approve config flag to be set in production builds.
- The system MUST NOT silently run a sandbox shortcut in a production environment without raising an alert and tagging the audit log.
- The system MUST NOT record a trust transition without operator id, reason (where mandatory), and timestamp.
- Documentation MUST NOT describe a production trust path as "auto" or "automatic" without specifying the attributable operator surrogate (e.g. system-batch-job with operator id).

## Related Doctrine

- [Verification Review Governance](./verification-review-governance.md) — every transition this doctrine governs.
- [Revocable Trust Model](./revocable-trust-model.md) — downgrade transitions also bound by these rules.
- [Capability Matrix](./capability-matrix.md) — `approved` opens payout authority; this is precisely why the transition must be attributable.
