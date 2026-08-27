# STAGE 2A — PASSWORD UX / VALIDATION AUDIT

> PHASE 1 — READ-ONLY AUDIT ONLY. No code was modified. No files were changed.

## 1. VERDICT

**AUDIT COMPLETE.**

Every password surface, validator, strength engine, and confirm-password
implementation in the current filesystem has been mapped and classified. The
audit produced one confirmed root-cause explanation for the reported Register
confirm-password behavior, identified a **three-way contradiction in the
password policy authority**, and found **three duplicate strength engines** plus
**two dead password components**. No implementation was changed; no business
decision was made.

---

## 2. PASSWORD SURFACE MAP

### Production surfaces (mobile app)

| # | Surface | File | Enter pw | Confirm pw | Strength | Match feedback |
|---|---|---|---|---|---|---|
| 1 | **Sign In** | `auth/presentation/screens/sign_in_screen.dart` | ✅ `AuthPasswordField` | ❌ | ❌ | ❌ |
| 2 | **Sign Up (Register)** | `auth/presentation/screens/sign_up_screen.dart` | ✅ `AuthPasswordField` | ✅ `AuthConfirmPasswordField` | ✅ inline `_buildPasswordStrengthIndicator` (screen-local) | ✅ via `AuthConfirmPasswordField` (buggy — see §6) |
| 3 | **Forgot Password** | `auth/presentation/screens/forgot_password_screen.dart` | ❌ (email only) | ❌ | ❌ | ❌ |
| 4 | **Change Password** | `profile/presentation/screens/security_screen.dart` | ✅ `AuthPasswordField` ×2 (current + new) | ✅ `AuthConfirmPasswordField` | ✅ `shared.PasswordStrengthIndicator` | ✅ via `AuthConfirmPasswordField` (buggy — see §6) |
| 5 | **Change Email (re-auth)** | `profile/presentation/widgets/security_email_section_widget.dart` | ✅ `AppTextField.password` | ❌ | ❌ | ❌ |
| 6 | **Verify Email** | `auth/presentation/screens/verify_email_screen.dart` | ❌ | ❌ | ❌ | ❌ |
| 7 | **Help Center article** | `system/support/presentation/screens/help_center_screen.dart` | ❌ (static copy) | ❌ | ❌ | ❌ |

### Production surfaces (admin app — separate scope)

| # | Surface | File | Notes |
|---|---|---|---|
| 8 | **Admin Login** | `apps/admin/src/pages/LoginPage.tsx` | Plain `<input type="password">`, no validation/strength/confirm. Firebase `signInWithEmailAndPassword`. |

### Non-production / dead surfaces

| # | Surface | File | Status |
|---|---|---|---|
| 9 | Auth-domain strength widget | `auth/presentation/widgets/password_strength_indicator.dart` | exported via `authentication.dart`, **zero production consumers** |
| 10 | Shared match widget | `shared/widgets/password_match_indicator.dart` | exported via `shared.dart`, **zero production consumers** |

---

## 3. PASSWORD INPUT MATRIX

| Widget | File | Production callers | Shared/reusable | Show/hide | Validator source | Realtime validation | Strength | Match feedback | Classification |
|---|---|---|---|---|---|---|---|---|---|
| `AuthPasswordField` | `auth/presentation/shared/widgets/auth_password_field.dart` | SignIn, SignUp, SecurityScreen | ✅ (auth-domain shared, exported via `presentation/shared/shared.dart`) | ✅ external `isPasswordVisible` + `onToggleVisibility` (caller-owned state) | Injected per-screen inline validator | Via `onChanged` only if caller wires it (SecurityScreen does; SignIn/SignUp do not) | Optional `strengthIndicator` slot | ❌ | **CANONICAL password input** |
| `AuthConfirmPasswordField` | same file | SignUp, SecurityScreen | ✅ (auth-domain shared) | ✅ external `isVisible` + toggle | Injected per-screen inline validator (match check) | ❌ **no internal controller listener** (root cause, see §6) | ❌ | ✅ inline (buggy) | **CANONICAL confirm input** (with a realtime bug) |
| `AppTextField.password` | `shared/widgets/app_text_field.dart` | SecurityEmailSectionWidget (change-email re-auth) | ✅ (shared, exported via `shared.dart`) | ✅ internal `_obscureText` state | None (no validator wired) | ❌ | ❌ | ❌ | **Specialized but legitimate** (re-auth confirmation) — but a duplicate password-input implementation |
| `AuthTextField` | `auth/presentation/shared/widgets/auth_text_field.dart` | email/username fields | ✅ | ❌ (not password) | Injected | ❌ | ❌ | ❌ | Not a password widget; doc says "use AuthPasswordField" |
| `ProfileTextField` | `profile/presentation/shared/widgets/profile_text_field.dart` | profile forms | ✅ | Has `obscureText` but no password factory | Injected | ❌ | ❌ | ❌ | Not used for passwords; doc explicitly says "Password fields → Create specialized widget" |
| `AppTextField` (base) | `shared/widgets/app_text_field.dart` | many forms | ✅ | `isPassword` internal toggle | Injected | ❌ | ❌ | ❌ | Generic field |

**There is already a canonical password field: `AuthPasswordField` /
`AuthConfirmPasswordField` (auth presentation shared).** `AppTextField.password`
is a legacy parallel password input used only for email re-auth.

---

## 4. PASSWORD POLICY AUTHORITY

### The policy contradiction (3 conflicting rule sets)

| Implementation | Defined | Enforced rules | Used by | Authority type |
|---|---|---|---|---|
| **A. `ValidationService.validatePassword`** | `shared/services/validation_service.dart:40` | min **8**, must contain `[a-z]`, `[A-Z]`, `\d` (lookahead regex) | **ZERO production callers** (only `validateForm` switch, which also has zero production callers) | Business/policy authority **by intent** (platform validation service), but **dead** |
| **B. Screen inline validators** | `sign_up_screen.dart:318-326` (`value.length < 8` only) | min **8** only | SignUp | UI feedback |
| | `security_screen.dart:186-194` | min **8** only | SecurityScreen (new pw) | UI feedback |
| **C. Strength widgets** | `shared/widgets/password_strength_indicator.dart` + auth-domain duplicate | min **6** + upper + lower + digit (score 0-4, valid = 4) | SecurityScreen only (shared one); auth-domain one has zero consumers | UI feedback only |
| **D. sign_up inline strength** | `sign_up_screen.dart:379-509` | min **8** + upper + lower + digit (score 0-4) | SignUp | UI feedback |
| **E. i18n copy** | `app_en.arb:744` / `app_id.arb:153` | "at least **8** characters, including uppercase, lowercase, and numbers" | Help center + security tip | **Documented business policy** (UX copy) |

**Contradictions:**
1. **Min length**: shared/auth strength widgets say **6**; everything else says **8**.
2. **The sign-up screen shows a strength meter (min 8) but its form validator only checks min 8** — a 7-char password with all character classes passes the validator but shows "Weak" on the meter.
3. `ValidationService.validatePassword` (the only centralized policy) is **unused by every production screen** — screens hand-roll inline validators.
4. Firebase enforces its own **min 6** (`weak-password` error) which is *looser* than the app's min-8 claim — a 6-char password accepted by Firebase would be rejected by the app's validator (and vice-versa the app blocks 6-char before Firebase ever sees it).

**Backend:** NO password policy exists. `auth_handler.go` only stamps usernames on the Firebase exchange; no password is ever sent to the backend. `pkg/firebase.CreateUser` (used by scripts only) passes a password to Firebase Admin.

**Security authority:** **Firebase Auth is the sole security authority** for acceptance/rejection (`weak-password`, `wrong-password`, `email-already-in-use`, etc.). The backend never sees passwords. UI validation is purely cosmetic pre-filtering.

---

## 5. PASSWORD STRENGTH

### Existing implementations (3 engines + 1 model)

| Engine | File | Min length | Labels | Consumers | Status |
|---|---|---|---|---|---|
| `PasswordStrengthIndicator` (Stateless) | `shared/widgets/password_strength_indicator.dart` | **6** | `Lemah/Sedang/Kuat/Sangat Kuat` (ID) | SecurityScreen only | DUPLICATE (live in 1 screen) |
| `PasswordStrengthIndicator` (Stateful) | `auth/presentation/widgets/password_strength_indicator.dart` | **6** | `Lemah/Sedang/Kuat/Sangat Kuat` (ID) | **none** (zero consumers; exported via `authentication.dart`) | LEGACY/ZOMBIE |
| Inline `_buildPasswordStrengthIndicator` | `sign_up_screen.dart:379` | **8** | `Weak/Fair/Good/Strong` (EN) | SignUp | SCREEN-LOCAL DUPLICATE |
| `PasswordStrength` model | both widget files (duplicated class) | — | — | internal to each widget | DUPLICATED MODEL |

**Semantics comparison:**
- All three score `minLength + uppercase + lowercase + number` (0–4).
- Shared/auth widgets: `isValid = score == 4` with min 6.
- sign_up inline: same scoring but min 8, different label mapping (Fair/Good vs Sedang/Kuat), English vs Indonesian.

**No zxcvbn, no entropy, no rules checklist service exists.** The "requirements checklist" in the widgets IS the de-facto rules display.

---

## 6. CONFIRM-PASSWORD MATCH

### Implementations

| Mechanism | File | Realtime? | How |
|---|---|---|---|
| `AuthConfirmPasswordField` inline match UI | `auth_password_field.dart:182-303` | ⚠️ partial | Computes `isMatch` in `build()` from both controllers; renders indicator when `confirmPassword.isNotEmpty` |
| Form validator (submit-time) | `sign_up_screen.dart:337-345`, `security_screen.dart:211-219` | Submit/validate-time | `value != passwordController.text` → error string |

### Root cause of "no match feedback until field loses focus"

`AuthConfirmPasswordField` is a `StatefulWidget` whose `_AuthConfirmPasswordFieldState`:
- has **no `TextEditingController` listener** on either controller, and
- **does not call `setState`** when the confirm text changes.

It therefore only rebuilds when its **parent** rebuilds. The parent screens:

- **SignUp**: `_passwordController.addListener(_onPasswordChanged)` → `setState` — so typing in the *password* field updates the match indicator in real time. But `_confirmPasswordController` has **no listener** — typing in the *confirm* field does NOT rebuild the parent, so the indicator does not update while typing.
- **SecurityScreen**: `onChanged: (value) => setState(() {})` is wired on the *new-password* field only. Same gap for the confirm field.

When the confirm field loses focus, Flutter Form validation runs → parent rebuilds → the indicator finally appears/updates. That is exactly the reported "until the user leaves the field" behavior.

There is **no shared, self-contained realtime match mechanism** — the reusable `PasswordMatchIndicator` widget exists but is **unused** by both `AuthConfirmPasswordField` (which has its own inline version) and every screen.

---

## 7. AUTHORITY MAP

```
SignIn / SignUp / Security / EmailChange
  UI screen
    └─ AuthPasswordField / AuthConfirmPasswordField / AppTextField.password
         └─ validator: SCREEN-INLINE (min 8; match check; required)   ← UI feedback
              └─ (unused) ValidationService.validatePassword           ← dead policy
                   └─ AuthController.signInWithEmail / signUpWithEmail / changePassword / resetPassword
                        └─ IAuthRepository → AuthRepositoryImpl
                             ├─ AuthCoreRepository.signInWithEmail → FirebaseAuth.signInWithEmailAndPassword
                             ├─ AuthSignupRepository → FirebaseAuth.createUserWithEmailAndPassword
                             ├─ AuthProfileRepository.changePassword → Firebase reauthenticate + updatePassword
                             └─ resetPassword → FirebaseAuth.sendPasswordResetEmail
                                  └─ Firebase Auth  ← SOLE SECURITY AUTHORITY (policy + acceptance)
                                       └─ Backend: NO password involvement (exchange only, username stamping)
```

| Responsibility | Layer | Notes |
|---|---|---|
| Password policy (min length, classes) | **Undefined/contradictory** — 3 competing rule sets; Firebase min-6 is the only enforced truth | Business decision required |
| Strength feedback | UI widgets (3 duplicate engines) | Pure UX |
| Confirm-password matching | Screen inline validators + `AuthConfirmPasswordField` inline UI | Pure UX |
| Actual acceptance/rejection | **Firebase Auth only** | Security authority |
| Backend | None — never sees passwords | |

---

## 8. DUPLICATE / LEGACY / ZOMBIE INVENTORY

| Candidate | Definition | Callers | Consumers | Status | Recommendation (do NOT act now) |
|---|---|---|---|---|---|
| `ValidationService.validatePassword` | `shared/services/validation_service.dart:40` | `validateForm` switch | **0 production** | DEAD policy engine | Decide: make it the canonical policy + wire screens, or delete |
| `ValidationService.validateForm` | same file:226 | **0 production callers** | — | DEAD | Same decision |
| `PasswordStrengthIndicator` (auth-domain) | `auth/presentation/widgets/password_strength_indicator.dart` | exported via `authentication.dart:23` | **0** | ZOMBIE duplicate (identical logic to shared) | Delete in cleanup phase |
| `PasswordStrength` (auth-domain) | same file | — | — | Duplicated model | Delete with widget |
| `PasswordMatchIndicator` (shared) | `shared/widgets/password_match_indicator.dart` | exported via `shared.dart:43` | **0** (AuthConfirmPasswordField has its own inline UI) | ZOMBIE | Delete or adopt as the single match mechanism |
| `PasswordStrengthIndicator` (shared, Stateless) | `shared/widgets/password_strength_indicator.dart` | SecurityScreen | SecurityScreen | DUPLICATE (min 6, contradicts policy) | Replace with canonical engine |
| Inline `_buildPasswordStrengthIndicator` (sign_up) | `sign_up_screen.dart:379` | SignUp | SignUp | SCREEN-LOCAL DUPLICATE (min 8, EN labels) | Replace with canonical engine |
| `AppTextField.password` | `shared/widgets/app_text_field.dart:80` | SecurityEmailSectionWidget | change-email re-auth | LEGACY parallel password input | Decide: migrate to `AuthPasswordField` or keep as specialized |
| `ARCHITECTURE.md` | `auth/docs/ARCHITECTURE.md` | — | — | STALE DOC (describes provider/use-case/pages architecture that does not exist; references `login_page.dart`, `register_page.dart`, `forgot_password_page.dart`, `LoginUseCase`, `AuthenticationProvider`) | Rewrite or delete in cleanup phase |
| Help-center "logged out from other devices" claim | `help_center_screen.dart:184` + `app_en.arb:1894` | — | — | DOC/UX claim NOT backed by implementation (`changePassword` does not revoke sessions) | Verify intended behavior; fix copy or implement revocation |

---

## 9. TEST COVERAGE

| Area | Tests found | Classification |
|---|---|---|
| Password validation (policy) | **None** — zero tests reference `validatePassword`, `ValidationService` password rules, or min-length policy | MISSING |
| Password strength | **None** — zero tests for `PasswordStrengthIndicator` / `PasswordStrength` / scoring | MISSING |
| Confirm-password match | **None** — zero tests for `AuthConfirmPasswordField`, `PasswordMatchIndicator`, or match behavior | MISSING |
| SignUp password form | `auth_email_signup_behavioral_test.dart` (compensation, exchange contract, username) | CANONICAL/CURRENT but **not** about password UX |
| SignUp recovery | `registration_username_recovery_test.dart` | Canonical (username, not password) |
| Login password | `auth_core_repository` covered indirectly; no widget test | MISSING (widget-level) |
| Reset/change password | **None** | MISSING |
| Shared password widgets | **None** | MISSING |
| `update_profile_use_case_test.dart` | uses `_FakeValidationService` — does not exercise password rules | UNRELATED |
| `settings_security_privacy_section_test.dart` | section navigation only, no password tests | UNRELATED |

**Summary: password UX has essentially zero dedicated test coverage.** The only password-adjacent tests exercise Firebase repository resilience and username exchange, not validation/strength/match behavior.

---

## 10. DOCUMENTATION / CONTRACTS

| Doc | Content | Contradiction |
|---|---|---|
| `app_en.arb:744` / `app_id.arb:153` `strongPasswordMessage` | "at least 8 characters, including uppercase, lowercase, and numbers" | Contradicts strength widgets (min 6); matches `ValidationService` (dead) and sign-up inline (min 8) |
| `help_center_screen.dart:175-184` article | "min 8 characters" + "logged out from other devices after changing password" | Min-8 matches copy; **session revocation claim is not implemented** |
| `auth/docs/ARCHITECTURE.md` | Old provider/use-case architecture; references non-existent files | Entirely stale — does not match current Riverpod/state architecture |
| `auth/docs/API.md`, `README.md` | High-level flows (reset password = email) | Consistent with current email-only reset |
| Widget doc comments | `auth_text_field.dart` says "Password fields → Use AuthPasswordField" | Consistent with canonical choice |
| `security_screen.dart` doc comment | "Password fields use shared AuthPasswordField" | Consistent |

---

## 11. BUSINESS / UX DECISIONS REQUIRED

These cannot be derived from the current codebase and need an owner decision:

1. **Canonical password policy.** Choose exactly one:
   - min **8** + uppercase + lowercase + digit (matches i18n copy, `ValidationService`, sign-up inline, security-screen validator), or
   - min **6** + 3-of-4 classes (matches the strength widgets and Firebase default).
   The current code contradicts itself; Firebase min-6 is the only thing actually enforced at the security layer.
2. **Where the policy lives.** Decide whether `ValidationService.validatePassword` becomes the single canonical policy used by all screens (recommended), or whether the policy stays in screen validators.
3. **Strength label language.** The shared widgets use Indonesian (`Lemah/Sedang/Kuat/Sangat Kuat`); the sign-up inline uses English (`Weak/Fair/Good/Strong`). Pick one canonical set and localize via ARB (no strength strings currently exist in ARB).
4. **"Logged out from other devices after changing password."** Confirm whether this is intended behavior (requires backend session revocation on password change — currently absent) or copy that must be corrected.
5. **`AppTextField.password`** — migrate the email re-auth field to `AuthPasswordField`, or keep the parallel implementation?
6. **Weak/Medium/Strong label requirement** (from the stage brief) — the current engines produce 4 labels; the brief asks for 3 (Weak/Medium/Strong). Confirm the target label set.

---

## 12. RECOMMENDED IMPLEMENTATION STAGES

Not implemented — for owner/chatgpt decision. Suggested bounded stages:

- **Stage 2B — Policy lock.** Single canonical `PasswordPolicy` (const min/max/classes) + wire `ValidationService.validatePassword` (or a new canonical validator) into all password validators. Delete dead `ValidationService` duplication if replaced. Update i18n copy to the locked policy.
- **Stage 2C — Canonical strength engine.** One `PasswordStrengthEngine` (pure function) + one `PasswordStrengthIndicator` widget consuming it; replace the 3 duplicates; localize labels; add unit tests.
- **Stage 2D — Realtime match fix.** Make `AuthConfirmPasswordField` self-contained: internal controller listeners + `setState` on confirm input (or adopt `PasswordMatchIndicator` with focus tracking). Fixes SignUp + Security realtime feedback; add widget tests.
- **Stage 2E — Residue cleanup.** Delete the auth-domain zombie strength widget, the unused `PasswordMatchIndicator` (if not adopted), stale `ARCHITECTURE.md`; resolve `AppTextField.password`; resolve help-center copy.
- **Stage 2F — Proof.** Focused widget + unit tests, `flutter analyze` on touched scope, regression on Register/Login/Change-Password flows.

---

## 13. FILES MODIFIED

**MUST BE EMPTY — and it is.**

No files were created, modified, or deleted in this audit phase.

---

## 14. BASELINE ISSUES

Encountered (pre-existing, NOT caused by this audit and NOT touched):

- `auth_email_signup_behavioral_test.dart`, `signup_outcome_binding_test.dart`,
  `auth_signup_production_path_test.dart`, `require_email_verification_gate_test.dart`,
  `auth_portal_protected_provider_blocking_test.dart` fail to compile against the
  current working tree (reference removed `AuthStateRequiresEmailVerification` /
  `BackendSyncOutcome` / `trackEngagement` symbols from other concurrent changes).
- The wider `flutter analyze` baseline (1989 issues) is unchanged; none are in
  password-scope files.
- `docs/flows/doctrine/username-lifecycle.md` etc. were updated in Stage 1D (prior
  stage, unrelated to this audit).

None of these affect the audit findings.

---

## 15. STOP

Audit complete. **No implementation performed.** Awaiting the owner/chatgpt
business decision (see §11) before any Stage 2B+ implementation. Per the
phase separation rule, AUDIT → DECISION → IMPLEMENTATION → PROOF → CLEANUP are
deliberately not collapsed.
