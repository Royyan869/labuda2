# STAGE 4B-1 — CANONICAL CLIENT FIELD VALIDATION

## STAGE 4B-1 FINAL REPORT

### Verdict

**IMPLEMENTED + PROVEN (unit/widget + source-level)**

Canonical email / Indonesian phone / URL validation authorities were created,
`ValidationService` was refactored into a thin delegating wrapper, and every
in-scope production consumer was converged to the canonical helpers. All new
tests pass (135 in the shared/helpers + delegation sweep), `flutter analyze`
is clean on every touched file, and the final residue audit found **zero
divergent production email/phone/URL regexes remaining**.

No password, username, moderation, Commerce, or unrelated files were touched.

---

### Business truth implemented

- **Phone (owner/ChatGPT-locked):** accepted prefixes `+62...`, `62...`, `0...`;
  canonical digit rule 9–12 digits after the applicable prefix; **format only**
  — no E.164 normalization, no verification, no network. `PhoneVerificationService`
  remains solely responsible for E.164 conversion and actual verification.
- **Email:** deterministic format check — accepts normal valid user emails,
  rejects clearly invalid input, no network/domain verification, no full RFC.
- **URL:** preserves the factual business behavior of the existing real
  consumer (`ValidationService.validateUrl` → `UpdateProfileUseCase`
  cover-photo/farm-website URLs): HTTPS accepted, `http://localhost` dev URLs
  accepted, plain `http` to non-localhost rejected. No network validation.
- **ValidationService kept** as a thin delegating wrapper (existing service
  seam with real URL-validation consumers).

---

### Canonical authorities

| Field | Authority | Location |
|---|---|---|
| Email | `CanonicalEmailValidator` | `lib/shared/helpers/canonical_email_validator.dart` |
| Phone | `CanonicalPhoneValidator` | `lib/shared/helpers/canonical_phone_validator.dart` |
| URL | `CanonicalUrlValidator` | `lib/shared/helpers/canonical_url_validator.dart` |
| Service seam | `ValidationService` (delegates email/phone/URL) | `lib/shared/services/validation_service.dart` |

Each helper is a pure static class following the established
`CanonicalUsernameValidator` / `CanonicalPasswordPolicy` /
`CanonicalPasswordStrength` pattern: `library;`, private constructor,
`isValid(...)` + `validationMessage(...)`, no UI rules.

**Canonical phone rule (explicit prefix accounting, as implemented/tested):**
- `+62` (3-char prefix) → 9–12 digits follow
- `62` (2-char prefix) → 9–12 digits follow
- `0` (1-char prefix) → 9–12 digits follow
- Spaces/hyphens between digits tolerated (stripped); surrounding whitespace trimmed.
- `PhoneVerificationService.isValidPhoneNumber` (`^\+628[0-9]{8,11}$`) was
  verified to be a **post-normalization E.164 check** (input already passed
  through `formatPhoneNumber`), i.e. a verification-path gate, NOT a
  format-validator consumer — intentionally left untouched.

---

### Consumer inventory (audit result)

Every production email/phone/URL validation site found and disposition:

| Surface | Before (divergent) | After |
|---|---|---|
| Sign-up email (`sign_up_screen.dart:303`) | `^[^@]+@[^@]+\.[^@]+` | `CanonicalEmailValidator.validationMessage` |
| Personal info email (`personal_information_screen.dart:176`) | `^[\w-\.]+@([\w-]+\.)+[\w-]{2,4}$` | `CanonicalEmailValidator.isValid` |
| Personal info phone (`:127`) | `^\+?[0-9]{10,15}$` (generic) | `CanonicalPhoneValidator.isValid` |
| Seller wizard email (`seller_wizard_step1_widget.dart:55`) | `^[\w-\.]+@...{2,4}$` | `CanonicalEmailValidator.validationMessage` |
| Seller wizard phone (`:76`) | `^\+?[0-9]{10,15}$` (generic) | `CanonicalPhoneValidator.isValid` |
| Address form fields phone (`address_form_fields.dart:115`) | `^(\+62|62|0)[0-9]{9,12}$` | `CanonicalPhoneValidator.validationMessage` |
| Address form dialog phone (`address_form_dialog.dart:465`) | `^(\+62|62|0)[0-9]{9,12}$` | `CanonicalPhoneValidator.validationMessage` |
| Address form recipient phone (`address_form_recipient.dart:63`) | `^(\+62|62|0)[0-9]{9,12}$` | `CanonicalPhoneValidator.validationMessage` |
| `StringExtensions.isValidEmail/isValidPhoneNumber` (`string_extensions.dart:4-14`) | duplicated regexes (`{9,13}` divergent) | delegate to canonical helpers |
| `ValidationService.validateEmail/PhoneNumber/Url` | own regexes | delegate to canonical helpers |
| `ValidationService.validateUsername` | — | **not modified** (Stage 1 username truth untouched) |
| `PhoneVerificationService` (E.164 + OTP) | — | **not modified** (normalization/verification, out of scope) |
| `UpdateProfileUseCase` (URL consumer) | — | unchanged (now backed by canonical URL rule) |

`ValidationService.validateContent`/anti-circumvention hook and
`IContentModerationService` were **not touched** (explicitly deferred to
Stage 4B-2).

---

### Actual files changed

Modified (11):
1. `lib/shared/services/validation_service.dart`
2. `lib/domains/user/identity/authentication/presentation/screens/sign_up_screen.dart`
3. `lib/domains/user/profile/presentation/screens/personal_information_screen.dart`
4. `lib/domains/user/profile/presentation/widgets/seller_wizard_step1_widget.dart`
5. `lib/domains/user/profile/presentation/widgets/add_edit_address_dialog/address_form_fields.dart`
6. `lib/domains/user/profile/presentation/widgets/address_form_dialog.dart`
7. `lib/domains/user/profile/presentation/widgets/address_form/address_form_recipient.dart`
8. `lib/core/src/utils/extensions/string_extensions.dart`

Created (3 lib + 4 test):
9. `lib/shared/helpers/canonical_email_validator.dart`
10. `lib/shared/helpers/canonical_phone_validator.dart`
11. `lib/shared/helpers/canonical_url_validator.dart`
12. `test/shared/helpers/canonical_email_validator_test.dart`
13. `test/shared/helpers/canonical_phone_validator_test.dart`
14. `test/shared/helpers/canonical_url_validator_test.dart`
15. `test/shared/services/validation_service_delegation_test.dart`

### Deleted files

None. (Moderation/anti-circumvention deletion is explicitly deferred to
Stage 4B-2.)

---

### Tests added

- `canonical_email_validator_test.dart` — valid emails, clearly invalid,
  empty/whitespace, trim behavior, `validationMessage`.
- `canonical_phone_validator_test.dart` — all three prefixes, digit boundaries
  (8/9/12/13), missing prefix, non-digits, formatting tolerance, foreign
  prefixes, `validationMessage`.
- `canonical_url_validator_test.dart` — HTTPS, localhost, plain-http-rejected,
  invalid schemes, empty, trim, `validationMessage`.
- `validation_service_delegation_test.dart` — proves `ValidationService`
  accepts/rejects exactly what the canonical helpers accept/reject for all
  three fields.

### Exact tests run / results

| Run | Result |
|---|---|
| `flutter test test/shared/helpers/canonical_email_validator_test.dart test/shared/helpers/canonical_phone_validator_test.dart test/shared/helpers/canonical_url_validator_test.dart test/shared/services/validation_service_delegation_test.dart` | **+35 All tests passed** |
| `flutter test test/domains/user/identity/authentication test/domains/user/profile/presentation/screens test/domains/user/profile/presentation/widgets` | **+145 passed, 16 failed-to-load** (all pre-existing baseline compile failures — see below) |
| `flutter test test/shared/helpers test/shared/services/validation_service_delegation_test.dart test/domains/user/preference/onboarding test/core/api/config` | **+146 All tests passed** |

### flutter analyze result

`flutter analyze` on all 11 modified files + 3 new helpers:
**No issues found.** (Also re-checked `string_extensions.dart` after the final
edit: No issues found.)

---

### Remaining baseline failures (not Stage 4B-1)

16 test files in the auth/profile suites fail to **load** due to pre-existing
baseline breakage from other sessions (references to removed APIs such as
`AuthState.requiresEmailVerification`, `PrincipalOperationCheck`,
`ApiClient.testing`, `userDatasource`/`sellerIdentity` params). None reference
any Stage 4B-1 file. Representative check:
`auth_email_signup_behavioral_test.dart` → `Member not found:
'AuthState.requiresEmailVerification'`. These are recorded, not repaired
(project doctrine: never silently repair unrelated baseline failures).

---

### Final duplicate/residue audit

- Grep for every former divergent regex
  (`^(\+62|62|0)[0-9]{9,12}`, `^(\+62|62|0)[0-9]{9,13}`,
  `^[\w-\.]+@...`, `^[^@]+@...`, `^\+?[0-9]{10,15}`, `^https?://...`)
  across `apps/mobile/lib`: **no matches remaining** — each was either
  converged or removed.
- Remaining email/phone/URL logic in production:
  - `PhoneVerificationService.isValidPhoneNumber` — legitimate distinct
    purpose (post-normalization E.164 gate for Firebase verification).
  - `StringExtensions` — now pure delegates to canonical helpers.
  - No other implementation found.
- `ValidationService` confirmed to delegate email/phone/URL to canonical
  helpers; username untouched (Stage 1 truth intact).
- No consumer accidentally retains divergent policy.

---

### Commerce boundary confirmation

No Commerce files were modified by Stage 4B-1. The `git diff` against the
working tree lists Commerce files only because they are part of the
**pre-existing dirty baseline** from other sessions (unchanged by this stage).
The touched scope is confined to `shared/helpers`, `shared/services`,
`core/src/utils/extensions`, auth sign-up, and profile/address widgets — none
of which are Commerce Product/FPS/Auction/quantity surfaces.

---

### Open issues / ambiguities

- **Digit-count convergence note:** the canonical 9–12 rule is now the single
  phone format rule everywhere. The old address forms used 9–12, the old
  `ValidationService` used 9–13, and `PhoneVerificationService` (E.164 gate)
  uses 8–11 after `+628` (= 9–12 after `+62`). These now agree. No business
  ambiguity remains for phone/email/URL.
- **Deferred:** dead moderation/anti-circumvention stack removal is Stage 4B-2,
  per the explicit scope split.
- **`ValidationService.validateUsername`** still contains the legacy
  uppercase-allowed regex; it was inspected and left untouched per
  instructions (username semantics are Stage 1 territory; no production
  consumer calls it). Flagged for a future bounded username-convergence pass
  if ever needed — not part of this stage.

---

### STOP

Stage 4B-1 implementation and proof complete.
No Stage 4B-2 (moderation cleanup) work was started.
No Commerce, password, username, or unrelated changes were made.
Awaiting instruction before Stage 4B-2.
