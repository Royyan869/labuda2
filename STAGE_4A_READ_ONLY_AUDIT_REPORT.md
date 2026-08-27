# STAGE 4A — READ-ONLY AUDIT REPORT

## STAGE 4A — READ-ONLY AUDIT REPORT

### VERDICT

Read-only audit complete. No production code, tests, schema, or configuration
was modified. The working tree was left exactly as found (baseline recorded
below).

The highest-leverage, lowest-conflict next bounded convergence candidate is:

**Canonicalize mobile client-side field validation (email / phone / URL) and
remove the dead client-side content-moderation (anti-circumvention) stack.**

---

### 1. Candidate findings

#### Candidate A — Dead client-side content-moderation stack (anti-circumvention) + orphan ValidationService

**Problem:** There are TWO full implementations of `IAntiCircumventionService`
(both live in the tree), one is `@Deprecated`, its tracking method is a no-op,
a third interface (`IContentModerationService`) has **zero implementations**,
and the single consumer path (`ValidationService.validateContent`) appears to
have **no production caller**. Meanwhile the backend has a real, authoritative
moderation domain (`backend/internal/governance/moderation/`) with
`moderation_service.go`, handlers, workers, and migrations — so the client-side
"circumvention" stack is a zombie simulation of an authority that actually
lives server-side.

**Factual evidence:**
- `lib/shared/services/anti_circumvention_service.dart` — class
  `AntiCircumventionService implements IAntiCircumventionService`, annotated
  `@Deprecated('Move to features/moderation/ or domains/system/admin/ ...')`,
  `_trackCircumventionAttempt()` body is **empty** (comment-only, lines 348-355).
- `lib/core/src/services/basic_anti_circumvention_service.dart` — second full
  implementation (`BasicAntiCircumventionService`), in-memory/session-only
  tracking ("resets on restart", "No database persistence"), exported via
  `core.dart:76`.
- `lib/shared/shared.dart:155-158` — comment says
  `// ✅ REMOVED: anti_circumvention_service.dart ... Currently accessed via
  core interface IAntiCircumventionService` — yet the deprecated file still
  exists and is still compiled.
- `lib/core/src/interfaces/services/i_content_moderation_service.dart` —
  `IContentModerationService` (moderateText/moderateImage/moderateVideo) has
  **no `implements` anywhere** in `apps/mobile/lib` (grep found only the
  interface file itself). Pure zombie interface.
- `lib/shared/services/validation_service.dart:15-18,183-199` —
  `ValidationService` optionally holds `IAntiCircumventionService` and calls
  `checkComment` inside `validateContent`.
- **`ValidationService` has zero production callers:** grep for
  `validateEmail|validatePhoneNumber|validateForm|validateUsername|
  validateContent` found only `validate_comment_content_use_case.dart:30`,
  which calls `_repository.validateContent` (the comment repository's local
  length check at `comment_repository_impl.dart:134-149`), **not**
  `ValidationService`. `ValidationService` is constructed in `main.dart:150`
  and injected, but never invoked.
- Backend authority: `backend/internal/governance/moderation/` — real
  `ModerationService`, `moderation_handler.go`, `moderation_event_handler.go`,
  `domain_action.go`, `moderation_resource_type.go`, migrations 000042/000043
  nearby, plus comment-create strict binding
  (`comment_handler_security_closure_test.go`).

**Affected surfaces:**
- `lib/shared/services/anti_circumvention_service.dart` (deprecated impl)
- `lib/core/src/services/basic_anti_circumvention_service.dart` (active impl)
- `lib/core/src/interfaces/services/i_anti_circumvention_service.dart`
- `lib/core/src/interfaces/services/i_content_moderation_service.dart` (zombie)
- `lib/shared/services/validation_service.dart` (orphan consumer)
- `lib/core/core.dart` / `lib/shared/shared.dart` exports

**Authority currently used:** Backend moderation domain is the factual
authority for content policy. Client-side circumvention is not wired into any
production content-submission path (comment creation goes through
`CommentRepositoryImpl` → datasource → backend; the only local check is
empty/2000-char length).

**Duplicate/zombie evidence:** two implementations of one interface + one
unimplemented interface + a deprecated file that "was removed" but still
compiles + an orphan validation service. Strong.

**User impact:** none directly (dead code), but the *existence* of the stack is
a correctness trap: a future dev wiring `validateContent` will silently invoke
a fake moderation engine instead of the backend authority.

**Development impact:** high — removes 3 files of misleading duplicate
authority; prevents re-introduction of client-side moderation.

**Estimated bounded scope:** small. Delete/extract + re-point `validateContent`
(or leave a documented stub delegating to backend).

**Risk:** low. Must verify `ValidationService.validateContent` truly has no
callers and decide whether `ValidationService` itself stays (it is injected in
`main.dart` and used by profile use cases for `validateUrl`).

**Runtime proof feasible:** yes — unit tests proving no production path calls
the dead stack; grep-based caller proof.

#### Candidate B — Duplicated, divergent email/phone/username/URL validators

**Problem:** The same field validations are re-implemented inline at each
surface with **different regexes and different accepted lengths**, producing
inconsistent UX (a phone accepted in one form is rejected in another) and
repeated bug surface. There is no canonical client-side validator for
email/phone other than the unused `ValidationService`.

**Factual evidence (regex inventory):**
- Email:
  - `sign_up_screen.dart:303` — `RegExp(r'^[^@]+@[^@]+\.[^@]+')` ("Invalid email format")
  - `seller_wizard_step1_widget.dart:55` — `RegExp(r'^[\w-\.]+@([\w-]+\.)+[\w-]{2,4}$')`
  - `personal_information_screen.dart:176` — same `[\w-\.]+@...` pattern
  - `ValidationService.validateEmail` — `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
  - `sign_up_screen.dart:112` — ad-hoc `contains('@')` gate for button enablement
- Phone (Indonesian):
  - `ValidationService.validatePhoneNumber` / `string_extensions.dart:12` — `^(\+62|62|0)[0-9]{9,13}$`
  - `address_form_fields.dart:115` / `address_form_dialog.dart:465` / `address_form_recipient.dart:63` — `^(\+62|62|0)[0-9]{9,12}$` (**9..12** — differs!)
  - `seller_wizard_step1_widget.dart:76` / `personal_information_screen.dart:127` — `^\+?[0-9]{10,15}$` (no Indonesian constraint at all)
  - `phone_verification_service.dart:69-74` — `^\+628[0-9]{8,11}$` (Firebase E.164-ish, different again)
- Username: `ValidationService` allows uppercase `[a-zA-Z0-9_]` while the
  canonical `canonical_username_validator.dart:26` and `complete_profile_screen.dart:76`
  and `mention_providers.dart:132` all use lowercase `^[a-z0-9_]+$` — the
  unused ValidationService is **already out of sync** with the Stage 1 canonical
  username truth (lowercase-only).
- URL: `ValidationService.validateUrl` enforces HTTPS-unless-localhost; profile
  use cases call `validateUrl` — but the inline form validators don't.

**Affected surfaces:** auth (sign-up), profile (personal info, seller wizard,
address forms ×3, phone verification), shared helpers, mention/search.

**Authority currently used:** none — each surface is its own authority; the
only "shared" authority (`ValidationService`) is unused and internally
inconsistent with Stage 1 canonical username rules.

**Duplicate/zombie evidence:** 4+ email regexes, 5+ phone regexes, 3 username
regexes, all divergent.

**User impact:** high-visible — the same "nomor HP" is valid in the address
form (9..12) but rejected in phone verification (8..11 after +628) or accepted
loosely in seller wizard (10..15 generic).

**Development impact:** high — new screens copy-paste a regex and risk picking
the "wrong" one.

**Estimated bounded scope:** medium — introduce canonical email/phone/URL
helpers (mirroring the Stage 2 pattern of `canonical_password_policy.dart` /
`canonical_password_strength.dart`), converge the inline call sites, keep
`ValidationService` delegating to them (or remove it).

**Risk:** medium-low — touches several auth/profile surfaces; must not touch
Commerce surfaces; each change is a mechanical regex swap.

**Runtime proof feasible:** yes — widget/unit tests per surface + a
convergence test proving all surfaces accept/reject the same inputs.

#### Candidate C — Startup/session: post-3B residue

The Stage 3B runtime proof is still the open closure item (owner-side cold
start). No new authority duplication found beyond what 3B removed. The
`RouterErrorPage` "Go to Home → /splash" path noted in Stage 3A remains a
curiosity but is not a duplicate authority. Lower leverage than A/B.

---

### 2. Recommended next stage

**Recommend Candidate B (canonical field validators) as the primary next
stage, with Candidate A (dead moderation stack removal) folded in as its
scoped cleanup companion.**

Rationale:

- **Highest leverage:** every auth/profile/address form in the app currently
  re-implements validation with divergent rules. Converging email/phone/URL
  under canonical helpers (exactly the pattern proven in Stage 2 for
  password/username) eliminates a whole class of repeated, inconsistent-bug
  surface and makes future screens correct by construction.
- **Directly aligned with the established convergence doctrine:** Stage 1/2
  already canonicalized username/password; email/phone/URL is the same kind of
  shared input-validation authority, and the existing `ValidationService` was
  *already designed to be* that authority but was never wired and drifted out
  of sync (uppercase username vs canonical lowercase). This is a "duplicate
  authority / stale implementation" candidate by the exact criteria requested.
- **Lowest conflict risk:** it touches auth/profile/address (not Commerce
  Product/FPS/Auction/quantity — which is the parallel chat's boundary). The
  only shared-surface overlap to guard is `address_form_*` (profile domain,
  not Commerce).
- **Candidate A as cleanup:** removing the dead circumvention stack is the
  surgical-cleanup phase of the same "client-side validation authority"
  scope — `ValidationService` is the orphan that hosts the circumvention
  call, so converging validation naturally forces the decision on whether the
  circumvention stack stays (it should go).

---

### 3. Proposed bounded stage (Stage 4B)

**What will be audited/changed:**
- Audit every email/phone/URL/username validation call site in `apps/mobile/lib`
  (auth, profile, address, seller wizard, phone verification, shared helpers).
- Introduce canonical helpers following the Stage 2 pattern, e.g.
  `lib/shared/helpers/canonical_email_validator.dart`,
  `canonical_phone_validator.dart`, `canonical_url_validator.dart` (pure
  functions, no widgets, no service).
- Wire `ValidationService.validateEmail/validatePhoneNumber/validateUrl/
  validateUsername` to delegate to the canonical helpers (so the existing
  interface stays the single service authority and stops drifting).
- Converge inline screen validators to the canonical helpers.
- Remove the dead client-side moderation stack (Candidate A):
  `AntiCircumventionService` (deprecated file), the circumvention hook inside
  `ValidationService.validateContent`, and evaluate
  `IContentModerationService` (zombie interface) for deletion — after
  caller/consumer proof.
- Align `string_extensions.dart` phone helper with the canonical phone
  validator.

**What will NOT be touched:**
- Commerce Product / FPS / Auction / forsale / quantity_available / product
  discovery / seller-scoped browse / inventory / related migrations (parallel
  chat boundary).
- Username canonical rules (Stage 1 truth stays: lowercase `[a-z0-9_]`).
- Password policy/strength/confirm (Stage 2 truth stays).
- Backend code (phone format is not validated server-side; the canonical
  client format should match Firebase E.164 for verification and the
  `(+62|62|0)` prefix forms for display — decision below).
- No new packages, no new architecture, no new services.

**Expected canonical behavior:**
- One email validator, one Indonesian phone validator (accepts `+62`/`62`/`0`
  prefixes, digit count converged), one URL validator (HTTPS-or-localhost),
  one username validator (delegates to Stage 1 canonical).
- Every form field produces identical accept/reject results for identical
  input, regardless of surface.
- `ValidationService` becomes a thin delegating wrapper (single service
  authority), and the dead circumvention stack no longer exists in the tree.

**Expected cleanup boundary:**
- Delete: `shared/services/anti_circumvention_service.dart`,
  `core/src/services/basic_anti_circumvention_service.dart`,
  `core/src/interfaces/services/i_anti_circumvention_service.dart` (and the
  `IContentModerationService` zombie if caller-proof holds), plus their
  exports in `core.dart`/`shared.dart`.
- Update: `ValidationService` (remove circumvention field), the inline
  validator call sites, `string_extensions.dart`, and any test that referenced
  the removed classes.
- No global cleanup, no unrelated files.

**Required proof:**
- Unit tests for each canonical helper (valid/invalid matrix).
- Convergence test: the same input accepted/rejected consistently across every
  converged surface (address form, seller wizard, personal info, sign-up,
  phone verification).
- Caller-proof grep: no production path references the removed classes.
- `flutter analyze` clean on all touched files; affected widget tests green.
- Regression: sign-up, complete-profile, address add/edit, personal info,
  seller wizard, phone verification all still validate as before (or with the
  converged rule, explicitly documented).

---

### 4. Business decisions required

Two decisions are needed before Stage 4B implementation:

1. **Phone format authority:** Should the canonical Indonesian phone validator
   accept the local prefixes `08…` / `62…` (display/forms) and normalize to
   `+628…` for verification (Firebase E.164), or require `+628…` everywhere?
   Recommendation: accept `(+62|62|0)` prefixes and digit-count `9..12` after
   prefix (matches the 3 address forms), and let the existing
   `PhoneVerificationService.formatPhoneNumber` handle normalization to E.164 —
   but the digit count must be converged (currently `{9,12}` vs `{9,13}` vs
   `{8,11}` after +628 vs generic `{10,15}`).
2. **`ValidationService` fate:** keep it as the thin delegating service wrapper
   (recommended — it is already injected in `main.dart` and used by profile
   use cases for `validateUrl`), or remove it and let call sites use canonical
   helpers directly. This is a bounded architecture choice, not a business
   rule.

No other business ambiguity was found. If the owner prefers to defer either
decision, Stage 4B can proceed on the recommendation and the decisions can be
recorded.

---

### 5. Parallel-boundary verification

The proposed stage touches only:

- `lib/shared/helpers/` (new canonical validators)
- `lib/shared/services/validation_service.dart` (+ `string_extensions.dart`)
- Auth screens (`sign_up_screen.dart`), profile screens/forms
  (`personal_information_screen.dart`, `seller_wizard_step1_widget.dart`,
  `address_form_*`, `phone_verification_service.dart`),
- the dead circumvention/moderation files.

It does **not** touch: Commerce Product identity/model semantics,
`quantity_available`, FPS public availability, Auction public availability,
product discovery queries, seller-scoped listing browse, Product reuse,
inventory/quantity redesign, or related schema/migrations. The only Commerce
directory with address-ish widgets (`commerce/transaction/...`) is not in the
change set; the address forms are in the `user/profile` domain.

**Confirmed: no overlap with Commerce Product Model B Stage 6B.**

---

### 6. STOP

Stage 4A read-only audit complete.
No implementation performed.
Working tree left exactly as found (pre-existing dirty baseline + Stage 3B
deliverables; nothing added or modified by this stage).
Awaiting owner decision on the recommended Stage 4B scope and the two business
decisions above before implementation.
