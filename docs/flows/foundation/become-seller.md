# Become Seller

> **Status:** STABLE
> **Domain:** Seller Capability
>
> **Doctrine references:**
> - [Seller Authority Separation](../doctrine/seller-authority-separation.md) - Become Seller opens selling sub-gate, not payout sub-gate.
> - [Layered Identity & Trust Model](../doctrine/layered-identity-trust-model.md) - this flow sits in Layer D; Layer A + B + C must pass first.
> - [Email Gating Matrix](../doctrine/email-gating-matrix.md) - `become_seller` is in the BLOCKED list.
> - [Capability Matrix](../doctrine/capability-matrix.md) - Layer 4 / Layer 5 / Layer 6 capability lookup.

## Purpose

Give a Registered User seller capability in Labuda through one paid activation path. Seller activation opens selling authority after seller subscription payment succeeds. Payout authority stays separate and is granted by verification approval (see [Submit Seller Verification](./submit-seller-verification.md)).

## Actors

- **Registered User** - activation initiator.
- **Seller** - the role the user receives after activation.
- **System** - creates seller profile, subscription, and initial verification record.
- **Payment Gateway** - processes seller subscription/activation payment.

## Preconditions

- **Layer A - Authentication:** user is signed in.
- **Layer B - Identity Completion:** `profile_completed=true`.
- **Layer C - Email Verification:** user email is verified (`become_seller` is BLOCKED in the canonical matrix).
- **Layer D - pre-state:** account is not suspended, revoked, or under investigation.

## Main Flow

1. User opens the **Seller Upgrade Wizard**.
2. **Step 0 - Pricing**: user sees the seller subscription/activation price.
3. **Step 1 - Account Prerequisites**: user confirms or updates:
   - Username (read-only if already saved; required only when missing)
   - Bio (required)
   - Phone number (required)
   - Sender address (required; structured fields: province, city/regency, district, village/subdistrict, street address, postal code)
   - Email verification status (read-only gate)
4. **Step 2 - Store Info**: user fills in:
   - Farm/store name (required)
   - Farm/logo photo (optional)
5. **Step 3 - Preview and Terms**: user reviews the summary and accepts the terms.
6. User taps **Submit**.
7. System:
   - Creates or updates the Seller Profile with initial tier `basic`
   - Creates the Seller Subscription
   - Adds the `seller` role to the user account
   - Creates the initial Seller Verification record with status `not_submitted`
8. User is sent to the seller subscription/activation payment flow.
9. After payment succeeds, subscription becomes `active` for 1 year.

## Alternate Flows

- Subscription expires, then the user loses selling authority for public publishing until renewal succeeds.
- Payment fails, then Seller Profile and verification record still exist, but subscription remains inactive until payment succeeds.
- User submits again, then the wizard loads existing farm data and updates it.

## Failure / Rejection Cases

- Email not verified - onboarding is rejected by backend.
- Account is suspended or banned - onboarding is rejected.
- Username, bio, phone number, or sender address is empty - rejected before onboarding can continue.
- Farm name is empty - rejected.
- Payment fails - Seller Profile is still created, but selling authority is not granted until payment succeeds.

## State Changes

- **Seller Profile**: none -> populated (`store_name`, tier `basic`)
- **Seller Subscription**: none -> `active` for 1 year after successful payment
- **User role**: `seller` is added
- **Seller Verification**: none -> initial `not_submitted` record
- **Selling capability**:
  - Private listings can be created once Seller Profile exists
  - Public listings can be created only while subscription is `active`
- **Payout capability**: remains locked until verification is approved
- **Account stage**: Email Verified Account -> Subscribed Seller (Unverified)

## Financial Impact

- User pays the seller subscription/activation fee via Payment Gateway.
- Payment status is authoritative in the Gateway; backend syncs via webhook.
- Subscription status is authoritative in backend `seller_subscriptions`.
- Coins are not involved.

## Notifications

- No special system notification beyond normal success or failure feedback.
- After activation, user starts receiving commerce-side notifications as usual.

## Cross-Domain Relations

- **Email Verification**: prerequisite
- **Submit Seller Verification (KYC)**: separate from Become Seller and controls payout authority
- **Seller Verification Review**: admin action
- **Listing / Auction**: publish capability depends on active subscription

## Business Rules

- Initial seller tier is `basic`
- Private listings can be created without active subscription
- Public listings can be published only while subscription is `active`
- Subscription is active for 1 year after payment, then expires if not renewed
- When expired, seller features for public publishing are disabled until renewal succeeds
- Seller Profile survives trust downgrade
## Forbidden Behaviors

- Do not allow seller onboarding if email is not verified.
- Do not grant selling authority if subscription is not `active`.
- Do not grant payout authority just because Become Seller finished.
- Do not skip creation of the initial verification record.
- Do not claim there is a free seller branch, lifetime seller branch, or separate onboarding fee.

## Notes

- "Become Seller" covers four concepts: role, profile, subscription, and verification status.
- Subscription details such as billing cycle, refund policy, and renewal behavior are tracked elsewhere.
