# LABUDA — ZERO-TO-ONE DEVELOPMENT DOCTRINE

## Canonical Operating Condition, Cleanup Policy, and Execution Discipline

**Status:** ACTIVE / CANONICAL

## 1. CURRENT APP CONDITION — FACTUAL AUTHORITY

Labuda is currently being built **from zero**.

It must be treated as a product under active construction, not as a mature production system that must preserve historical implementations.

Current operating assumptions:

- There are **no real production users whose existing data must be preserved**.
- There is **no production history that must be protected as design authority**.
- There is **no backward-compatibility obligation** for obsolete internal code or architecture.
- Old code does not remain authoritative merely because it already exists.
- Git history is **not** design authority.
- Old documentation is **not** design authority.
- Existing residue is **not** design authority.
- The owner decides business truth.
- The factual current filesystem is the technical object being changed.
- Once canonical business truth and technical authority are proven, obsolete alternatives must be removed.

Labuda must **NOT** be handled as though it were serving millions of active users with years of production data and strict compatibility obligations.

---

## 2. CORE PRINCIPLE

## IF IT IS NO LONGER CANONICAL, DO NOT PRESERVE IT.

When a business rule, domain authority, architecture, or implementation has been replaced:

> **The replacement becomes authority. The obsolete implementation dies.**

Do not keep obsolete code because:

- “maybe something still needs it”
- “for safety”
- “for compatibility”
- “just in case”
- “maybe old clients use it”
- “it used to work”
- “Git has history”
- “we may need rollback”
- “let us keep an adapter first”

Those are not valid reasons unless the owner explicitly establishes a current requirement.

---

## 3. ZERO-TO-ONE DELETION POLICY

The default policy for proven obsolete code is:

# DELETE TOTAL.

This includes, when no longer canonical:

- legacy implementations;
- duplicate repositories;
- proxy repositories;
- shadow authorities;
- compatibility layers;
- compatibility shims;
- adapters whose only purpose is preserving obsolete architecture;
- fallback paths;
- deprecated DTO fields;
- phantom domain entities;
- unused state;
- dead providers;
- old terminology;
- old routes;
- obsolete API handlers;
- duplicate persistence paths;
- zombie feature flags;
- dead comments documenting dead architecture;
- obsolete tests;
- unused exports;
- stale documentation.

No “soft preservation” by default.

No parallel authority.

No zombie compatibility.

No “keep it until later” residue.

If it has been proven unnecessary, **remove it now as part of the same convergence work**.

---

## 4. CLEAN MEANS TOTAL CLEAN

A domain is not clean merely because the new implementation works.

It is clean only when:

1. The canonical authority is singular.
2. Obsolete parallel implementations are gone.
3. Dead references are gone.
4. Dead imports are gone.
5. Dead exports are gone.
6. Old terminology is removed where it represents obsolete authority.
7. Compatibility shims are removed unless explicitly required.
8. Duplicate state authority is removed.
9. Proxy layers with no independent responsibility are removed.
10. Compiler/analyzer/test validation confirms the remaining implementation is coherent.

The target is not:

> “The new code works while the old code still exists.”

The target is:

> **“Only the code that represents current truth remains.”**

---

## 5. BUSINESS TRUTH FIRST

Labuda has a strict authority hierarchy.

The owner decides business truth.

Development must not replace an owner decision with:

- old code;
- old documents;
- Git history;
- previous architecture;
- legacy behavior;
- assumptions about production compatibility.

If the owner establishes a new or corrected business truth, development must converge to it.

Do not ask obsolete code for permission to continue existing.

---

## 6. WHEN AUDIT IS REQUIRED

Audit is useful only when a material fact is not yet known.

Examples:

- Which implementation is currently authoritative?
- Are there duplicate write paths?
- Is a field actually persisted?
- Which references remain?
- Is there more than one source of truth?

Audit is **not** a recurring ritual.

Once required facts are proven and the implementation inventory is clear:

> **STOP AUDITING. START EXECUTING.**

Do not repeatedly re-read surrounding code to rediscover facts already established.

---

## 7. EXECUTION MODE AFTER AUTHORITY IS CLEAR

Once business truth and technical authority are clear:

```text
DECIDE / PROVE
      ↓
DELETE what is obsolete
      ↓
EDIT what remains but is incorrect
      ↓
REMOVE broken references
      ↓
FORMAT
      ↓
ANALYZE / COMPILE
      ↓
FIX CONCRETE ERRORS
      ↓
RUN RELEVANT TESTS
      ↓
FINAL REFERENCE SWEEP
      ↓
DONE
```

Do not replace this with:

```text
delete one field
→ read surrounding architecture
→ search again
→ reconsider business meaning
→ inspect alternatives
→ audit again
→ delete another field
```

That is over-exploration after the decision is already complete.

---

## 8. COMPILER/ANALYZER IS A CLEANUP TOOL

After an approved purge:

1. Delete the proven obsolete implementation.
2. Compile/analyze.
3. Follow concrete errors.
4. Remove or correct remaining references.
5. Repeat until clean.

Do not manually explore every possible dependency before making an approved deletion.

Compiler errors, analyzer errors, test failures, and final targeted grep expose remaining mechanical dependencies.

---

## 9. NO MILLION-USER PARANOIA

Do not assume Labuda needs production-maintenance behavior unless the owner explicitly says the condition has changed.

Do not automatically introduce or preserve:

- backward compatibility;
- dual-read logic;
- dual-write logic;
- legacy endpoint support;
- deprecated DTO compatibility;
- compatibility aliases;
- old route preservation;
- old repository preservation;
- fallback architecture;
- adapter chains;
- version bridges;
- historical behavior preservation.

These mechanisms add complexity.

Complexity must have a current requirement.

In a zero-to-one app, obsolete complexity is technical debt, not safety.

---

## 10. CANONICAL CONVERGENCE RULE

When two implementations represent the same current concept:

1. Identify the canonical authority.
2. Move active callers to that authority.
3. Validate.
4. Delete the other implementation.
5. Remove every remaining reference.
6. Confirm there is only one authority.

Never leave:

```text
Canonical implementation
        +
Old implementation
        +
Adapter
        +
Fallback
        +
“temporary” compatibility path
```

unless the owner explicitly requires temporary coexistence.

For Labuda, temporary coexistence is not the default.

---

## 11. ZOMBIE CODE DEFINITION

“Zombie code” is code that remains alive in the filesystem despite no longer representing current truth.

Examples:

- old repository delegating to a new repository;
- entity fields never persisted or read;
- DTO fields never sent by backend;
- provider for a deleted concept;
- service that only forwards calls without responsibility;
- fallback for a removed architecture;
- compatibility alias;
- dead widget;
- unused route;
- old terminology representing an obsolete domain;
- tests preserving obsolete behavior.

Zombie code must not be normalized.

If proven obsolete:

# KILL IT.

---

## 12. SPEED DISCIPLINE

Speed does not mean careless coding.

Speed means refusing unnecessary work.

Once authority is proven, unnecessary work includes:

- repeated architecture discovery;
- repeated terminology mapping;
- repeated “understanding context”;
- searching for alternative designs;
- re-litigating owner decisions;
- protecting obsolete implementations;
- preserving unused abstractions;
- auditing the same domain again before each deletion.

> **Think deeply only until the truth is clear. Then execute aggressively and mechanically.**

---

## 13. ONE ACTIVE SCOPE

For a current scope:

- delete what is obsolete;
- edit what is incorrect;
- fix errors caused by that scope;
- validate that scope;
- stop.

Do not use cleanup as an excuse to wander into unrelated domains.

Clean aggressively **inside the proven scope**.

---

## 14. AGENT EXECUTION RULE

When an agent receives an implementation task whose business truth and factual audit are already complete, the agent must:

1. Execute approved changes.
2. Delete proven obsolete artifacts directly.
3. Edit proven incorrect implementations directly.
4. Use analyzer/compiler/test failures to finish mechanical cleanup.
5. Avoid exploratory loops.
6. Avoid reopening business decisions.
7. Avoid redesigning architecture.
8. Avoid expanding scope.
9. Stop when the requested scope is clean and validated.

The agent must not spend significant time rediscovering facts already supplied in the task.

---

## 15. CHATGPT OPERATING RULE

ChatGPT must not unconsciously apply large-production-system assumptions to Labuda.

Before recommending preservation, compatibility, migration complexity, fallback behavior, or legacy coexistence, ask:

> **Is there an actual current requirement for this, or am I imagining millions of active users and production data that do not exist?**

If no current requirement exists:

> **Do not preserve obsolete complexity.**

Necessary caution is for genuinely unclear facts.

Preserving obsolete code after replacement is proven is not caution. It is delay and technical debt preservation.

---

## 16. DEFAULT DECISION RULE

For any artifact:

### A. Does it represent current canonical business/technical truth?
- YES → keep and correct if needed.
- NO → continue.

### B. Is it required by the new canonical implementation?
- YES → rework it into canonical form.
- NO → delete it.

### C. Does deletion create errors?
- YES → fix concrete references/errors.
- NO → deletion stands.

### D. Is there an explicit current compatibility requirement?
- YES → implement only the minimum required compatibility.
- NO → do not preserve compatibility.

---

## 17. FINAL DOCTRINE

Labuda is being built.

It is not yet a legacy production platform that must carry historical baggage forever.

Therefore:

> **Business truth changes → code converges.**
>
> **Authority is proven → duplicate authority dies.**
>
> **Obsolete code is proven → delete it.**
>
> **Implementation is wrong → edit it.**
>
> **Deletion causes errors → fix the errors.**
>
> **Validation is green → move on.**

No zombie compatibility.

No residue preservation.

No parallel authority.

No repeated exploration after facts are proven.

No treating an in-construction zero-to-one application as though millions of active users would be harmed by deleting obsolete internal architecture.

# THE TARGET IS TOTAL CONVERGENCE.

**What remains in the filesystem should represent current Labuda truth.**

Everything else must justify its existence.

If it cannot:

# REMOVE IT.
