# STAGE 4C — READ-ONLY AUDIT REPORT
## Next P2/P3 UX / Validation / Profile Convergence Candidate

**Date:** 2026-08-25
**Type:** Read-only audit. No production code, tests, schema, or migrations were modified.
**Scope:** Client-side auth/profile/address/settings form UX + shared input components + dead/zombie client logic.

---

## 1. VERDICT

**The next highest-value bounded stage is: Canonicalize remaining client-side field validation on the auth + profile + address surfaces, and remove the dead/contradictory profile-state and address-form residue that currently blocks consistent UX.**

Concretely, three coherent, safe, bounded workstreams were found, all sharing one canonical authority (`CanonicalPhoneValidator` / `CanonicalEmailValidator` / `CanonicalUrlValidator` from Stage 4B-1) and all proven by existing consumer patterns:

1. **A1 — Canonical validation convergence (auth + profile + seller wizard + address residue)** — 6 P1/P2 validator gaps where the canonical authority exists but is bypassed by hand-rolled regexes, `contains('@')` gates, or no validator at all.
2. **A2 — Profile save-state hardening (Unified Edit Profile)** — one P1 double-submit defect (unguarded Save button) and one P2 cover-photo round-trip hole (client writes a field the mapper hardcodes to `null`).
3. **A3 — Settings fake-toggle removal** — two user-visible switches that persist nothing (Public Profile, Allow Messages) plus one stale test that asserts a mutation path the shipped widget no longer has.

The audit deliberately stops after recommending the **single top candidate** (A1). A2 and A3 are documented as clean follow-on bounded stages, not folded in.

---

## 2. AUDIT SCOPE

Audited (client only):
- `apps/mobile/lib/domains/user/identity/authentication/presentation/**` (sign_in, sign_up, forgot_password, security screens; auth shared widgets)
- `apps/mobile/lib/domains/user/profile/presentation/**` (profile_screen, unified_edit_profile, personal_information, security, settings screens; address dialogs; profile widgets)
- `apps/mobile/lib/domains/user/preference/seller/presentation/**` (seller upgrade wizard, seller wizard step widgets) — validation only
- `apps/mobile/lib/shared/helpers/**`, `apps/mobile/lib/shared/services/**`, `apps/mobile/lib/shared/widgets/**`
- `apps/mobile/lib/domains/user/profile/data/**` (mappers, repositories, DTOs, sync service) — read path only
- `apps/mobile/test/**` — stale-test detection only (no test modified)
- Consumer tracing via grep across `lib/` and `test/`, DI/provider wiring, barrels, routes.

**Explicitly NOT audited (boundaries respected):**
- Commerce product/FPS/Auction/quantity, Search, Feed, Chat, Notification, backend moderation, password policy/strength/match internals, username business truth, startup availability (Stage 3B done), Payment, Seller subscription, KYC business logic, financial/ledger, schema/migrations, unrelated full-project baseline failures.
- Backend code was read only to confirm client-side facts (e.g. whether a field round-trips); no backend changes recommended.

---

## 3. BUSINESS TRUTHS USED

| # | Truth | Status | Source |
|---|-------|--------|--------|
| B1 | Username is immutable after establishment; profile/settings must never edit it | Locked (Stage 1) | Auth/Profile contracts |
| B2 | Indonesian phone format: `+62`/`62`/`0` prefix + 9–12 subscriber digits (up to 13 total); spaces/hyphens tolerated; format-only, no normalization | Locked (Stage 4B-1) | `canonical_phone_validator.dart` |
| B3 | Email format is a single canonical rule | Locked (Stage 4B-1) | `canonical_email_validator.dart` |
| B4 | URL format is a single canonical rule | Locked (Stage 4B-1) | `canonical_url_validator.dart` |
| B5 | Address phone/recipient-name are required for buyer shipping and seller sender address | Established by active consumers | `AddressFormDialog`, `AddEditAddressDialog` |
| B6 | Cover photo is a profile field the client can upload; whether the backend round-trips it is a server contract question | **UNCERTAIN — business decision required (see §8)** | `user_api_mapper.dart:61` |
| B7 | `show_activity_status` / `allow_messages_from` are real backend DTO fields (present in models/sync) but have no UI consumer wiring | **Contradiction — decision required** | `user_api_models.dart:239-263` |

**FACT vs INFERENCE:** B1–B5 are facts from locked contracts and current code. B6 and B7 are facts about client code with an open ownership decision — flagged in §8, not invented.

---

## 4. SURFACES AUDITED

### 4a. Auth / profile form UX (validation, requiredness, realtime vs submit, error presentation)
- `sign_in_screen.dart`, `sign_up_screen.dart`, `forgot_password_screen.dart`, `security_screen.dart`
- `auth_text_field.dart`, `auth_password_field.dart`, `auth_button.dart`, `username_field.dart`, `auth_state_view.dart`
- `personal_information_screen.dart` + `personal_information_section.dart`
- `unified_edit_profile_screen.dart` + `edit_profile/*` sections + `edit_profile_save_handler.dart`
- `validation_service.dart` (delegation seam audit)

### 4b. Form state / submission UX
- Same screens as 4a; submit-button enable predicates, loading guards, error mechanisms
- `settings_screen.dart` toggle wiring

### 4c. User profile / settings
- `profile_screen.dart`, `profile_about_tab.dart`, `profile_share_builder.dart`
- `settings_screen.dart` + settings sections widgets
- `edit_profile_personal_section.dart` vs `personal_information_section.dart`
- `seller_identity_view.dart`, `seller_dual_avatar.dart`, `seller_identity_data.dart`, `user_identity_formatter.dart`
- `user_api_mapper.dart`, `user_sync_service.dart`, `profile_repository_api.dart`

### 4d. Address / contact UX (post 4B-1)
- `address_list_screen.dart`, `address_form_dialog.dart`, `add_edit_address_dialog.dart` (+ sub-widgets), `address_screen.dart`
- `address_form/*` widgets, `shipping_address_section_widget.dart`, `address_form_widget.dart`
- Checkout address picker (display-only, confirmed correct by design)
- `phone_verification_service.dart`, `phone_verification_provider.dart`

### 4e. Shared input components + dead logic
- Full inventory of `shared/widgets`, `shared/services`, `shared/helpers`
- Full inventory of `profile/presentation/widgets`, auth `widgets` + `shared/widgets`
- Stale-test scan across `test/`

---

## 5. FINDINGS

### 5a. VALIDATION CONVERGENCE GAPS (candidate stage A1)

**F-1 — P1 · Sign-in email validator is hand-rolled and differs from canonical.**
- Surface: `sign_in_screen.dart:227-237` — `RegExp(r'^[\w-\.]+@([\w-]+\.)+[\w-]{2,4}$')`.
- Consumers: sign-in form field.
- Canonical authority: `CanonicalEmailValidator` (used by sign-up `sign_up_screen.dart:300-301`).
- Classification: **DUPLICATE BUT ACTIVE / CONTRADICTORY** (3 rules for the same field: canonical, sign-in regex, forgot `contains('@')`).
- Note: same regex also differs from canonical on TLD rules (2-4 chars vs 2+ chars).

**F-2 — P1 · Forgot-password email validator is `contains('@')` only.**
- Surface: `forgot_password_screen.dart:191-199`.
- Classification: **DUPLICATE BUT ACTIVE** (weakest gate of all; `a@b` passes).
- Fix: delegate to `CanonicalEmailValidator.validationMessage`.

**F-3 — P1 · Seller-upgrade wizard phone field has required-only validation (no format gate).**
- Surface: `seller_upgrade_wizard_screen.dart:1092-1100` — `validator: (value) => value == null || value.trim().isEmpty ? 'Required' : null`, and `SellerWizardHelpers.isAccountStepValid` (`seller_wizard_helpers.dart:41-53`) checks only `isNotEmpty`.
- Consumers: seller onboarding account step; value sent to `authController.updateProfile(phoneNumber:)`.
- Canonical authority: `CanonicalPhoneValidator` — used by the **other** seller wizard widget (`SellerWizardStep1Widget`).
- Classification: **DUPLICATE BUT ACTIVE** (same field, two wizard surfaces, one canonical, one not).
- Fix: swap validator to `CanonicalPhoneValidator.validationMessage`.

**F-4 — P1 · `PhoneVerificationService.isValidPhoneNumber` re-implements the format rule with its own regex.**
- Surface: `phone_verification_service.dart:69-74` — `RegExp(r'^\+628[0-9]{8,11}$')`; consumed by `phone_verification_provider.dart:86` (`sendOTP`).
- Canonical authority: `CanonicalPhoneValidator` — the canonical file itself states normalization is `PhoneVerificationService.formatPhoneNumber`'s job (`canonical_phone_validator.dart:10-11`).
- Classification: **DUPLICATE BUT ACTIVE** (near-contradictory at edges: accepts `( )`-stripping that canonical rejects; digit rule is equivalent only after normalization).
- Fix: keep `formatPhoneNumber` as normalizer; delegate validity to `CanonicalPhoneValidator`.

**F-5 — P2 · Personal-information phone save path is unvalidated (verify path is canonical, save path is not).**
- Surface: `personal_information_section.dart:192-224` — raw `TextField`, no validator, not wrapped in the `Form`; `personal_information_screen.dart:274-329` saves `_phoneController.text.trim()` directly. Canonical check exists only in `_verifyPhone` (`:130`).
- Classification: **DUPLICATE BUT ACTIVE / CONTRADICTORY** (user can save `"not-a-phone"` via Save Changes).
- Fix: canonical validator on the field or pre-validate non-empty phone in `_savePersonalInformation`.

**F-6 — P2 · Sign-in realtime submit gate uses `contains('@')` while its own field validator is a regex.**
- Surface: `sign_in_screen.dart:69-73` (`hasEmail = ...contains('@')`) feeding `isEnabled: _controller.isFormValid` (`:306-311`); `validate()` at `:112` then blocks the weak value.
- Classification: **CONTRADICTORY** (button enables for `a@b`, submit silently refuses — confusing UX).
- Fix: drive the gate from the same validator as the field.

**F-7 — P2 · Edit-profile website URL fields have no client validator; validation is backend-only.**
- Surface: `edit_profile_store_section.dart:36-42`, `edit_profile_farm_section.dart:36-42` (no validator); `update_profile_use_case.dart:100-107` validates via `CanonicalUrlValidator` only on save.
- Classification: **ACTIVE gap** (canonical authority exists, not wired into the form; error arrives as a late SnackBar).
- Fix: `validator: (v) => CanonicalUrlValidator.validationMessage(v)` on both sections.

**F-8 — P2 · Address surface residue: 3 dialogs + 2 dead address form widgets + inconsistent field rules.**
- Surfaces:
  - `AddressFormDialog` (ACTIVE — `address_list_screen.dart:795-804`): canonical phone; street min-length not enforced (non-empty only, `address_form_dialog.dart:547-552`); id preserved on edit.
  - `AddEditAddressDialog` (ACTIVE — `seller_upgrade_wizard_screen.dart:611`): canonical phone via `AddressFormFields`; uses `Uuid().v4()` for new ids; `forcedPurpose: sender`; street min-length 5 (`address_form_fields.dart:180-182`); postal digits-only + 5 limit formatter (`:201-204`).
  - `address_screen.dart` (LEGACY/ZOMBIE — no route wires it; exported from `profile_feature.dart:100`): **no phone or recipient-name field at all**; saves phone from profile/old entity silently; street non-empty only.
  - `address_form/*` widgets (`address_form_recipient.dart` canonical-validated, `address_form_detail.dart`, `address_form_header.dart`, `address_form_wilayah.dart`): **zero consumers** (only `address_form_widget.dart`, which is also dead).
  - `address_form_widget.dart` (ZOMBIE, exported `profile_feature.dart:124`), `shipping_address_section_widget.dart` (used only by dead `AddressScreen`) — ~95% identical wilayah form; postal/required rules duplicated 3×.
- Classification: **DUPLICATE BUT ACTIVE + LEGACY/ZOMBIE + CONTRADICTORY** (same semantic field `AddressEntity.phone` required in dialogs, absent in `AddressScreen`).
- Fix: consolidate on `AddressFormDialog`; delete `AddressScreen`, `AddressFormWidget`, `AddressFormWidget`'s section twin, and `address_form/*`; unify street/postal rules.

**F-9 — P2 · Phone display/verification formatting helpers diverge from canonical prefix set.**
- Surface: `phone_verification_service.dart:42-66` `formatPhoneNumber` handles `08…`, `8…`, `628…`, `+628…`, `0…` — a superset of canonical prefixes, but complementary by design (normalizer).
- Classification: **CANONICAL/ACTIVE** (complementary; the only issue is F-4 re-implementing policy). Documented for completeness.

### 5b. PROFILE SAVE-STATE / SUBMISSION (candidate stage A2)

**F-10 — P1 · Unified Edit Profile Save button has no loading guard — double-submit risk.**
- Surface: `unified_edit_profile_screen.dart:495-499` — `onPressed: save` (no `_isLoading ? null` guard); mixin `save()` (`edit_profile_save_handler.dart:58-94`) runs two async phases (personal + profile fields) and `Navigator.pop()`.
- Contrast: `personal_information_screen.dart:370-372` and `security_screen.dart:221` both guard with `onPressed: _controller.isLoading ? null : ...`.
- Classification: **ACTIVE / defect** (double avatar/cover upload, double `updateFields`, double pop on rapid double-tap).
- Fix: `onPressed: _isLoading ? null : save`.

**F-11 — P2 · Cover-photo edit writes a field the canonical mapper never reads back (round-trip hole).**
- Surface: `edit_profile_save_handler.dart:116-118` sets `fields['coverPhotoUrl']`; `profile_core_provider.dart:120-134` applies it locally; `profile_notifier.dart:91-93` same; BUT `user_api_mapper.dart:61` hardcodes `coverPhotoUrl: null // Not in API yet` — so on next refetch (`profileStreamProvider`), the cover URL is lost. `_profileEntityToUpdateRequest` (`profile_repository_api.dart:285-296`) also omits `coverPhotoUrl`.
- Classification: **CONTRADICTORY / ACTIVE gap** (client persists locally, backend response never returns it; cover edit is effectively live-fire into a void on refetch).
- **Business decision required** (see §8 D-2): either the backend exposes cover photo (add request+response field), or the client should gate/hide the cover editor.

**F-12 — P2 · Sign-up password-match gate compares trimmed text while the widget indicator compares raw text.**
- Surface: `sign_up_screen.dart:117-119` (`trim()` compare) vs `auth_password_field.dart:241-244, 316-319` (raw equality, gated on `hasConfirm`); `security_screen.dart:210-211` uses raw compare (agrees with widget).
- Classification: **CONTRADICTORY** (whitespace edge: indicator says "match", submit stays disabled).
- Fix: canonicalize trim-before-compare inside the widget (or the gate) — one definition of "match".

**F-13 — P2 · `validation_service.dart` retains dead divergent rules (username regex, content, form-map, userId).**
- Surface: `validation_service.dart:61-86` (`validateUsername` hand-rolls `^[a-zA-Z0-9_]+$`, uppercase-allowed — conflicts with lowercase-only `CanonicalUsernameValidator`); `validateContent`/`validateForm`/`isValidUserId`/`validateRequired`/`validateMinLength`/`validateMaxLength`/`validateEmail`/`validatePassword`/`validatePhoneNumber` — **no callers found** (only `validateUrl` is live via `update_profile_use_case.dart:67,101`).
- Classification: **LEGACY/ZOMBIE** (thin seam for url; dead divergent rules for everything else).
- Fix: delete dead methods or re-delegate username to canonical (username decision is out of scope — see §8 D-1).

### 5c. PROFILE / SETTINGS SURFACE (candidate stage A3)

**F-14 — P2 · Settings "Public Profile" toggle is a non-persisted fake switch.**
- Surface: `settings_screen.dart:32, 90-91` — `_profilePublic = true` hardcoded; `onChanged: setState` only; never hydrated from `UpdateProfileApiRequest.visibility` (`user_api_models.dart:235`), never written.
- Classification: **LEGACY/CONTRADICTORY** (toggle implies writability; nothing persists).
- **Decision required** (see §8 D-3): wire to backend or remove.

**F-15 — P2 · Settings "Allow Messages" toggle is a non-persisted fake switch.**
- Surface: `settings_screen.dart:33, 94-95` — same pattern; backend field `allow_messages_from` exists (`user_api_models.dart:239`, `user_sync_service.dart:281-304`) but nothing writes it; `profile_repository_api.dart:294-295` writes only `showPhoneNumber`/`showEmail`.
- Classification: **LEGACY/CONTRADICTORY**.
- **Decision required** (see §8 D-3).

**F-16 — P2 · Settings "Activity Status" tests are stale against the shipped widget.**
- Surface: `test/.../settings_screen_activity_status_test.dart:198-212` drives a `SwitchListTile` "Activity Status" and asserts `{'show_activity_status': false}` is sent; the shipped widget renders **"Show Online Status"** (`settings_security_privacy_section.dart:64`) wired to local `presenceManagerProvider.setEnabled` → local storage (`presence_provider.dart:40,103-119`) — the asserted mutation path does not exist in production. `settings_security_privacy_section_test.dart` also passes `showActivityStatus:` params the widget no longer accepts.
- Classification: **STALE TEST** (tests a removed surface; likely already failing or testing a shim).
- Fix: rewrite against current `showOnlineStatus` wiring.

**F-17 — P3 · `isSocialMediaPublic` toggle has no backend field.**
- Surface: `edit_profile_contact_section.dart:111-117` toggle; `user_api_mapper.dart:215` hardcodes `true`; entity default `true`; `profile_about_tab.dart:548-549` reads it.
- Classification: **STALE CONTROL** (no-op toggle).
- **Decision required** (see §8 D-4).

**F-18 — P3 · Dual identity renderers with divergent display order/fallbacks.**
- Surface: `profile_screen.dart:987-1022` renders `@username` as primary line; `SellerIdentityView._buildProfile` (`seller_identity_view.dart:68-90`) renders storeName first then handle (drawer `drawer_header.dart:210-213`); `profile_info.dart:137,222` re-derives `@` ad hoc instead of `UserIdentityFormatter.formatHandle` (`user_identity_formatter.dart:40`).
- Classification: **DUPLICATE BUT ACTIVE** (canonical formatter bypassed).
- Fix: route all through `UserIdentityFormatter`/`SellerIdentityView`.

**F-19 — hygiene · Settings identity tile copy over-promises KTP.**
- Surface: `settings_profile_identity_section.dart:30-33` "Date of birth, phone, and KTP verification"; `personal_information_section.dart:6-8` says KYC/KTP handled separately.
- Classification: **HYGIENE** (copy fix).

### 5d. SHARED INPUT / DEAD LOGIC (inventory — hygiene, not stage candidates)

**Duplicate active field widgets:**
- Phone: 9 input sites, 3 validation regimes (see F-1/F-3/F-4/F-5).
- Email: 4 wrappers (`AuthTextField.email`, `AppTextField.email`, `ProfileTextField.email`, raw) — hygiene.
- Username: `UsernameField` (sign-up) vs `AuthTextField.username` factory (no consumers) vs `ValidationService.validateUsername` (divergent) — see F-13.
- Form controllers: `AuthFormController` + `ProfileFormController` + `SecurityFormController` — 3 overlapping state models — hygiene.

**Zombie widgets (0 production consumers; exported in barrels):**
- `shared/widgets`: `media_carousel_widget.dart` (+container/content/video/indicators), `custom_image_cropper.dart`, `flutter_crop_image.dart`, `online_avatar_widget.dart`, `user_header_widget.dart`, `status_badge.dart`, `payment_method_card.dart`, `card_header.dart`, `base_metric_card.dart`, `engagement_section.dart`, `hashtag_input_widget.dart`, `image_with_badge.dart`, `address_display_section.dart`, `polling_status_indicator.dart`.
- `profile/presentation/widgets`: `profile_info.dart`, `profile_stat_card.dart`, `security_email_section_widget.dart`, `address_card_widget.dart`, `address_empty_state_widget.dart`, `address_form_widget.dart`, `ktp_preview_section.dart`, `ktp_upload_section.dart`.
- `shared/services`: `media_upload_service.dart`, `mention_notification_service.dart`, `firebase_wilayah_service.dart` (barrel-only).
- `shared/helpers`: `canonical_user_id_validator.dart` (no consumers in lib).

**Zombie profile sections:**
- `about_sections/about_section_about/farm/contact.dart` unreferenced; `profile_about_tab.dart` inlines private `_ProfileSectionCard`/`_ProfileInfoRow`/`_SocialMediaChip` duplicates.

**Stale tests:** none orphaned to deleted files. `confirm_password_consumer_convergence_test.dart` asserts absence of the deleted `password_match_indicator.dart` (passes; relic). No imports of deleted files remain in `lib/` or `test/`.

---

## 6. AUTHORITY MAP

| Business field | Canonical authority | Consumers using it | Consumers bypassing it |
|---|---|---|---|
| Phone format | `CanonicalPhoneValidator` | seller_wizard_step1, address_form_dialog, address_form_fields, address_form_recipient, personal_information verify path, validation_service | seller_upgrade_wizard phone (required-only), PhoneVerificationService (own regex), personal-info save path (none) |
| Email format | `CanonicalEmailValidator` | sign_up, personal_information guard, seller_wizard_step1, validation_service | sign_in (own regex), forgot_password (`contains('@')`), sign_in/sign_up gates (`contains('@')`) |
| URL format | `CanonicalUrlValidator` | validation_service → update_profile_use_case (save-time) | edit_profile store/farm website fields (no client validator) |
| Username format | `CanonicalUsernameValidator` (lowercase-only) | username_field, username_validation_service | validation_service.validateUsername (uppercase-allowed, dead) |
| Password match | (no shared authority) | sign_up gate (trimmed), security gate (raw), widget indicator (raw) | — (3 definitions, see F-12) |
| Address (wilayah/postal) | none (3 duplicated impls) | address_form_dialog, add_edit_address_dialog, (dead: address_form_widget, shipping_address_section_widget) | — |
| Cover photo | none (client-local only) | profile_core_provider, profile_notifier | user_api_mapper (hardcodes null) |

---

## 7. DUPLICATE / ZOMBIE MAP

| Item | Status | Consumers | Notes |
|---|---|---|---|
| `AddressFormDialog` | ACTIVE | address_list_screen | canonical phone; keep |
| `AddEditAddressDialog` | ACTIVE | seller_upgrade_wizard | canonical phone; merge target |
| `AddressScreen` | ZOMBIE (exported) | none | contradictory if revived; delete |
| `AddressFormWidget` + `shipping_address_section_widget` | ZOMBIE | none (section used only by dead screen) | delete |
| `address_form/*` (recipient/detail/header/wilayah) | ZOMBIE | only dead parent | delete after merge |
| `profile_info.dart`, `profile_stat_card.dart`, `security_email_section_widget.dart`, `address_card_widget.dart`, `address_empty_state_widget.dart`, `ktp_*_section.dart` | ZOMBIE | none | delete |
| `media_carousel_widget*`, `custom_image_cropper`, `flutter_crop_image`, `online_avatar_widget`, `user_header_widget`, `status_badge`, `payment_method_card`, `card_header`, `base_metric_card`, `engagement_section`, `hashtag_input_widget`, `image_with_badge`, `address_display_section`, `polling_status_indicator` | ZOMBIE | none | delete (hygiene, not in A1) |
| `validation_service` dead methods (username/content/form/userId/min/max/required) | ZOMBIE | none | delete or re-delegate |
| `edit_profile_validators.dart` `validateDisplayName`/`validateFarmName` | ZOMBIE | none (sections re-inline) | delete or wire |
| `settings` fake toggles (Public Profile, Allow Messages) | LEGACY | tile only | remove or wire (A3) |
| `profile_screen` inline identity map vs `SellerIdentityData` | DUPLICATE BUT ACTIVE | profile_screen / drawer | converge via formatter (P3) |

---

## 8. CONTRADICTIONS / BUSINESS DECISIONS REQUIRED

**D-1 — Username rule duplication (dead path).** `ValidationService.validateUsername` (`^[a-zA-Z0-9_]+$`, allows uppercase) contradicts the canonical lowercase-only `CanonicalUsernameValidator`. The method has no callers. Since username business truth is locked (out of scope), the safe move is to delete the dead method rather than change semantics. **Owner decision:** delete vs re-delegate.

**D-2 — Cover photo round-trip (F-11).** Client uploads cover photo to S3, writes `coverPhotoUrl` into the local `ProfileEntity`, but `UserApiMapper.toProfileEntity` hardcodes `coverPhotoUrl: null` and `UpdateProfileApiRequest` omits it — so the backend never receives/returns it, and the cover vanishes on next refetch. **Owner decision required:** does the backend expose a cover-photo field (add to request + response + migration), or should the client gate the cover editor behind an available field? This is the single most important business decision in this report.

**D-3 — Settings fake toggles (F-14/F-15).** Backend DTO fields exist (`visibility`, `allow_messages_from`) but nothing reads/writes them from settings. **Owner decision:** wire the toggles to the backend (via `UpdateProfileApiRequest`/`user_sync_service`), or remove the toggles from UI. Wiring `allow_messages_from` may overlap the chat/messaging privacy domain (flagged overlap risk).

**D-4 — `isSocialMediaPublic` (F-17).** Toggle has no backend field; mapper hardcodes `true`. **Owner decision:** add backend field or remove toggle.

**D-5 — Autovalidate strategy (architectural).** No `autovalidateMode` anywhere in `lib/`; realtime feedback is hand-rolled per screen (sign-in listeners, sign-up `_isFormValid`). A deliberate architecture-lock comment (`auth_form_controller.dart`) governs this. **Owner decision:** keep submit-time validation + per-screen gates (recommended for bounded scope), or introduce a shared realtime mechanism. Not required for A1.

---

## 9. PRIORITIZED CANDIDATES

Ranking applied: user-visible impact, regression risk, number of divergent consumers, strength of canonical authority, cleanup safety, testability, scope isolation.

| Rank | Candidate | Findings | Severity | Canonical authority | Divergent consumers | Proof quality | Overlap risk |
|---|---|---|---|---|---|---|---|
| **1** | **A1 — Canonical validation convergence** (sign-in, forgot-password, seller-wizard phone, phone-verification service, personal-info phone save, sign-in gate, edit-profile URL, address residue) | F-1..F-9 | P1/P2 | **Strong** (Stage 4B-1 helpers) | 8+ | **Excellent** (canonical validators already tested; swaps are 1-line delegations; deterministically testable) | Low (isolated to auth/profile/address/seller-wizard fields) |
| 2 | A2 — Profile save-state hardening | F-10..F-13 | P1/P2 | Partial (loading-guard pattern exists in sibling screens; match rule needs decision) | 3 | Good (widget tests exist for edit profile) | Low |
| 3 | A3 — Settings fake-toggle removal | F-14..F-17 | P2/P3 | Backend DTO fields exist | 2 toggles + 1 stale test | Good (stale test must be rewritten) | **Medium** (allow_messages ↔ chat privacy; visibility ↔ public-profile semantics) |

---

## 10. RECOMMENDED NEXT STAGE

### **Stage 4D: Canonical Validation Convergence (A1)**

Rationale:
- The canonical authority is **already locked and tested** (Stage 4B-1). Every fix is a *delegation swap*, not a new business decision.
- User-visible inconsistencies are immediate: the same email field validates differently across three auth screens; a seller can onboard with `"abc"` as a phone number; a user can save `"not-a-phone"` in Personal Info.
- Regression risk of leaving unresolved: phone/email garbage enters the profile/onboarding data path; two more regexes accumulate drift.
- Scope isolation is strong: no backend change, no schema change, no cross-domain touch.
- Deterministic proof: swap → run existing canonical-validator tests + per-screen widget tests.

---

## 11. EXACT BOUNDED SCOPE FOR STAGE 4D (A1)

**In scope (implementation):**
1. `sign_in_screen.dart` — replace hand-rolled email regex validator with `CanonicalEmailValidator.validationMessage`; drive the `isFormValid` gate from the field validator result (remove `contains('@')`).
2. `forgot_password_screen.dart` — replace `contains('@')` validator with `CanonicalEmailValidator.validationMessage`.
3. `seller_upgrade_wizard_screen.dart` — phone field validator → `CanonicalPhoneValidator.validationMessage` (required + format in one message); align `SellerWizardHelpers.isAccountStepValid` with canonical validity (or rely on form validation).
4. `phone_verification_service.dart` — `isValidPhoneNumber` delegates to `CanonicalPhoneValidator` (keep `formatPhoneNumber` as normalizer; update `sendOTP` call site accordingly).
5. `personal_information_screen.dart` / `personal_information_section.dart` — attach canonical phone validator to the phone field (or pre-validate non-empty phone in `_savePersonalInformation`).
6. `edit_profile_store_section.dart` / `edit_profile_farm_section.dart` — add `CanonicalUrlValidator.validationMessage` to website fields.
7. Address residue (dead code removal + consolidation):
   - Delete `address_screen.dart` (+ its `profile_feature.dart` export).
   - Delete `address_form_widget.dart` and `shipping_address_section_widget.dart`.
   - Delete `address_form/*` widgets.
   - Consolidate street/postal validation rules onto the surviving `AddressFormDialog` (single rule set: street ≥5 chars? — **owner decision needed on street min-length**: dialogs disagree today; propose canonical = non-empty, matching `AddressFormDialog`, or 5 chars matching `AddressFormFields`; do NOT invent).
8. Tests: update/add widget tests proving each surface delegates to the canonical validator; delete tests that referenced deleted address widgets; rewrite the stale `settings_screen_activity_status_test.dart` **only if** it blocks the build (otherwise leave for A3).
9. Delete `ValidationService` dead methods **only** if no consumer is found at implementation time (verify with grep first). Username-method deletion requires D-1 sign-off.

**Decision gates before implementation:**
- D-2 (cover photo) — **NOT in A1**; A1 is validation-only. Documented for A2.
- D-1 (username dead method) — optional; default = delete dead method if no consumers.

**Proof of completion:** `flutter test` on the touched screens' test files + canonical validator tests; `flutter analyze` clean on touched files; grep proof that no hand-rolled email/phone regex remains in the touched surfaces.

---

## 12. EXPLICIT OUT-OF-SCOPE

- Any backend change (including cover-photo field — separate decision D-2).
- Schema/migrations.
- Password policy/strength/match internals (except the trim-mismatch F-12, which is deferred to A2).
- Username business truth (dead-method deletion only, per D-1).
- Startup availability (Stage 3B complete).
- Commerce product/search/feed/chat/notification/payment/KYC/seller-subscription/ledger domains.
- Global zombie-widget purge (`media_carousel_widget*`, croppers, `status_badge`, etc.) — hygiene backlog, not a stage.
- Identity rendering convergence (F-18) — P3, deferred.
- Settings fake toggles (F-14/F-15/F-16/F-17) — candidate stage A3.
- A2 (save-state hardening: F-10 double-submit, F-11 cover round-trip, F-12 match trim, F-13 validation_service dead rules) — candidate stage A2, not authorized.

---

## 13. BASELINE FAILURES

- The stale `settings_screen_activity_status_test.dart` and `settings_security_privacy_section_test.dart` may already be failing at baseline; their production surface (Show Online Status via `presenceManagerProvider`) is live and correct. Classified as A3 scope; do not fix in A1 unless the build gate requires it.
- `backend/vet_errors.txt`, `backend/vet_output.txt`, `backend/build_b.txt`, `backend/build_b2.txt` exist as untracked files — pre-existing backend tooling output, not part of this audit's baseline.
- No other in-scope baseline failures identified. Full-project unrelated failures were not assessed (out of scope per instructions).

---

## 14. RISKS

1. **F-4 phone-verification delegation:** `PhoneVerificationService` is used by OTP flows; changing `isValidPhoneNumber` to delegate to canonical must not change accepted numbers for already-normalized input (equivalence is proven for `+628…` shape). Mitigation: keep `formatPhoneNumber` normalization first, then canonical check; add a test matrix of accepted/rejected numbers.
2. **Address consolidation:** `AddEditAddressDialog` (seller wizard) has `forcedPurpose: sender` + name-lock behavior; merging onto `AddressFormDialog` must preserve forced-purpose semantics. Mitigation: keep both dialogs in A1 if merge risk exceeds value; the P1 fixes do not require the merge. **The merge is optional; deletion of dead widgets is not.**
3. **D-2 cover photo:** if A2 is later authorized and the backend does not expose the field, the client must gate the cover editor; do not ship A1/A2 changes that imply a cover round-trip exists.
4. **Sign-in gate change (F-6):** changing the button-enable predicate may alter UX for users with `a@b`-style input; acceptable and intended.
5. **Test churn:** deleting dead address widgets requires removing their (passing) test files; verify with grep before deletion.

---

## 15. STOPPING POINT

- This report ends the Stage 4C read-only audit. **No production code, tests, schema, migrations, or docs were modified** (only this report was created).
- No cleanup was performed.
- Next action is **human review of this report**, then decision on D-1..D-5 as needed, then authorization of Stage 4D (A1) — or A2/A3 per priority.

---

## APPENDIX: FACT vs INFERENCE

| Claim | Status |
|---|---|
| sign_in uses hand-rolled regex; forgot uses `contains('@')`; sign_up uses canonical | FACT (read) |
| seller wizard phone validator is required-only | FACT (read) |
| `PhoneVerificationService.isValidPhoneNumber` has own regex | FACT (read) |
| personal-info save path does not validate phone | FACT (read: raw TextField not in Form) |
| `profilePublic`/`allowMessages` toggles only call setState | FACT (read) |
| `user_api_mapper.dart:61` hardcodes `coverPhotoUrl: null` | FACT (read) |
| `updateProfile` request omits coverPhotoUrl | FACT (read) |
| settings activity-status test asserts a mutation path that production lacks | FACT (read test + widget) |
| `AddressScreen` has no route consumer | FACT (grep; exported in barrel) |
| zombie widgets have zero consumers | FACT (grep; barrel-only exports counted as zombie) |
| severity/priority rankings | INFERENCE (scored per §9 criteria) |
| "merge address dialogs is optional" | INFERENCE (risk-based recommendation) |
| stale settings tests are failing at baseline | INFERENCE (not executed — read-only audit) |
