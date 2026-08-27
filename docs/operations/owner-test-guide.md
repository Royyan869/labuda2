# Owner Test Guide

**Status: OWNER_TEST_READY**
Last reviewed: 2026-07-03

> **Canonical accounts:** Owner visual/internal test uses **only** the 3 Gmail
> accounts defined in [`owner-test-runtime.md`](../owner-test-runtime.md#akun-owner-test-satu-satunya-akun-yang-dipakai-owner).
> `*.test.local` accounts (`buyer@test.local`, `seller@test.local`, `admin@test.local`)
> are `cmd/seed` dev fixtures, not owner-test accounts — do not use them here.

End-to-end test script for the Labuda platform owner test. Covers all 20 functional flows, a detailed order/refund/dispute walkthrough, and the accepted P3 ledger that the tester should know before testing.

---

## Purpose

This guide is for the **platform owner or a delegated QA tester** running a structured first-pass walkthrough of the complete Labuda system. The goal is to confirm that all major flows work as designed before inviting external beta users.

This is **not** a regression or load test. It is a functional and UX coherence check.

Owner test is **not** a production-readiness sign-off.

Severity rules:

- P0 or P1: stop the test and fix the issue before continuing.
- P2: log it and continue only if the flow is still safe to finish.
- P3: record it for reference only.

---

## Environment Checklist

Before starting:

- [ ] Backend running on port 8080 (`cd backend && go run ./cmd/core_server`)
- [ ] Admin panel running on port 5173 (`cd apps/admin && npm run dev`)
- [ ] Mobile app (Flutter debug build) pointing at the backend
- [ ] Migrations applied (`cd backend && go run ./cmd/migrate` — applies `000001_canonical_schema`)
- [ ] Platform configs seeded (run `cd backend && go run ./cmd/seed` after migrate)
- [ ] Admin user bootstrapped (see [`admin-bootstrap.md`](./admin-bootstrap.md))
- [ ] Seed data prepared (see [`dev-seed-guide.md`](./dev-seed-guide.md))
- [ ] Midtrans sandbox keys configured and webhook delivery working

---

## Tester Roles

| Role | Account | Tools |
|---|---|---|
| **Admin** | `busiyono79@gmail.com` (username `yayan`, super-admin) | Admin panel at `localhost:5173` |
| **Seller A** | `qiqiaja7799@gmail.com` (username `qiqijho`) | Mobile app |
| **Buyer A** | `nurroyyan89@gmail.com` (username `royyan`) | Mobile app |
| **Buyer B** | *(ad hoc — see note below)* | Mobile app (for bid competition only) |

> **Buyer B is not a canonical owner-test account.** The 3 Gmail accounts above
> are the only pre-provisioned identities for owner test. Flow 11 (bid
> competition) is the only flow needing a second buyer — sign up a temporary
> account through the mobile app for that flow only, then discard it. Do not
> treat it as a persistent test identity.

---

## Test Account Setup

Before running the seller and withdrawal flows, make sure Seller A has:

- An active seller subscription
- KYC approved
- At least one active bank account
- That bank account reviewed for payout by an admin

If the mobile UI supports multiple bank accounts, set the reviewed account as the default before testing withdrawal.

---

## Flow 1 — Sign Up (Email)

**Actor:** Guest

1. Open mobile app → Welcome screen → Sign Up
2. Enter a fresh email address (not in DB), username, password
3. Verify: registration success, redirect to Complete Profile screen
4. Verify: email verification banner shown

**Pass criteria:** User account created, profile incomplete state shown.

---

## Flow 2 — Sign In (Email)

**Actor:** Buyer A

1. Open mobile app → Sign In → Email/Password
2. Enter Buyer A credentials
3. Verify: home feed loads, bottom nav visible

**Pass criteria:** Successful login, no error screen.

---

## Flow 3 — Sign In (Google)

**Actor:** Any account with Google Firebase linkage

1. Open mobile app → Sign In → Continue with Google
2. Select Google account
3. Verify: if new user → Complete Profile screen; if returning → home feed

**Pass criteria:** Auth flow completes without error.

---

## Flow 4 — Complete Profile

**Actor:** Any newly registered user

1. Profile → Edit Profile
2. Fill in: display name, bio, location
3. Save
4. Verify: profile page reflects updated data

**Pass criteria:** Profile data persisted, no validation errors on valid input.

---

## Flow 5 — Become Seller (Subscription)

**Actor:** Seller A (fresh account, not yet a seller)

1. Profile → Settings → Upgrade to Seller
2. Complete seller wizard: username (read-only if already saved), bio, phone number, sender address, then store/farm name
3. Proceed to subscription payment
4. Use Midtrans sandbox card and complete payment
5. Verify: seller dashboard visible after webhook delivers confirmation
6. Verify: seller subscription status = `active`

**Pass criteria:** Seller subscription created, seller dashboard accessible.

---

## Flow 6 — Submit Seller Verification

**Actor:** Seller A (subscription active)

1. Seller Dashboard → Submit Verification
2. Upload placeholder KTP image
3. Fill in KTP number and full name
4. Submit
5. Verify: verification status shown as `Menunggu Review` (pending_review)

**Pass criteria:** Verification request in DB with `pending_review` status.

---

## Flow 7 — Admin Reviews Seller Verification

**Actor:** Admin

1. Admin panel → Seller Verification
2. Find Seller A's pending request
3. Review uploaded KTP image and data
4. **Approve** (happy path) or **Reject** with a rejection reason
5. Verify (approve path): Seller A's mobile app shows verified status; KTP badge appears on profile
6. Verify (reject path): rejection stored; mobile shows rejected status

**Pass criteria (approve):** Seller A's verification status = `verified`, badge visible on profile.

If the rejection reason is not surfaced in the mobile app, verify that during owner test rather than treating it as an accepted caveat.

---

## Flow 8 — Create Listing

**Actor:** Seller A (verified)

1. My Listings → Create Listing
2. Fill in: title, description, price, stock count, category (Koi), add 1–3 photos
3. Submit
4. Verify: listing appears in My Listings with `active` status
5. Verify: listing appears in search results and on public profile

**Pass criteria:** Listing created and visible in discovery surfaces.

---

## Flow 9 — Create Auction

**Actor:** Seller A

1. My Listings → Create Auction
2. Fill in: title, starting bid, description, duration, add photos
3. Submit
4. Verify: auction appears with `scheduled` or `active` status
5. Verify: countdown timer shown correctly

**Pass criteria:** Auction created, visible on discovery surfaces, bid section shown when active.

**Note:** If auction is `scheduled`, the AuctionWorker (runs every minute) transitions it to `active` at the scheduled start time. In local dev you can force-transition via the SQL shortcut in `dev-seed-guide.md`.

---

## Flow 10 — Buyer Buy Now / Checkout

**Actor:** Buyer A

1. Browse home feed or search → find an active listing
2. Tap listing → Buy Now
3. Select shipping address (create one if not exists: Profile → Addresses)
4. Select shipping option
5. Review order summary (price, shipping cost, platform fee breakdown)
6. Confirm → redirected to Midtrans payment page
7. Complete payment with sandbox card
8. Verify: Payment Result screen shows success
9. Verify: Order created with status `paid`
10. Verify: Seller A sees the order in Orders

**Pass criteria:** Order in `paid` state, buyer and seller both see it.

---

## Flow 11 — Buyer Places Bid

**Actor:** Buyer B

1. Find an active auction (home feed or search)
2. Tap auction → Bid section
3. Enter a bid amount above the current highest bid
4. Confirm bid
5. Verify: bid recorded, current price updated on screen
6. Verify: Buyer A (if previously highest bidder) receives outbid notification

**Pass criteria:** Bid placed, auction price updated.

---

## Flow 12 — Auction Settlement → Order

This requires an auction to end with a winning bid. Either wait for the natural end time or use the SQL shortcut in `dev-seed-guide.md` to set `ends_at = NOW() - INTERVAL '1 minute'` and let the AuctionWorker run.

1. Auction ends → AuctionWorker processes it → winning bidder receives `waiting_settlement` notification
2. Buyer B (winner) → Orders → find the auction order
3. Verify: order status is `pending_payment` or `paid` depending on payment path
4. Complete payment if needed (same Midtrans sandbox flow as Flow 10)

**Pass criteria:** Auction-sourced order created for winning bidder.

---

## Flow 13 — Seller Ships Order

**Actor:** Seller A

1. Orders → find the `paid` order
2. Tap → Ship Order
3. Enter courier name and tracking number (any string for local dev)
4. Submit
5. Verify: order status transitions to `shipped`
6. Verify: Buyer A receives shipping notification

**Pass criteria:** Order in `shipped` state, tracking info visible to buyer.

---

## Flow 14 — Buyer Confirms Receipt

**Actor:** Buyer A

1. Orders → find the `shipped` order
2. Tap → Confirm Receipt
3. Confirm in dialog
4. Verify: order status transitions to `completed`
5. Verify: Seller A's balance increases (visible in Seller Dashboard → Earnings)
6. Verify: rating prompt appears for buyer to rate seller

**Pass criteria:** Order `completed`, seller balance credited.

---

## Flow 17 — Order / Refund / Dispute (Detailed Script)

> This is the most important owner-test flow. Follow every step in order.

### Setup

- Order A is in `shipped` state (Seller A shipped, Buyer A has not confirmed)
- Buyer A is logged in on mobile

### Step 1 — Buyer Requests Refund

1. Buyer A → Orders → Order A → Request Refund
2. Select refund reason (e.g., "Item tidak sesuai deskripsi")
3. Optional: upload evidence photo
4. Submit
5. **Expected:** order status = `refund_requested`, Seller A receives notification

### Step 2 — Seller Accepts Refund

(Happy path — test this first)

1. Seller A → Orders → Order A → Review Refund Request
2. Read buyer's reason and evidence
3. Tap **Approve**
4. **Expected:**
   - Order status = `refund_approved`
   - Gateway refund initiated (Midtrans sandbox)
   - Buyer A receives refund approval notification
   - Seller balance not credited (escrow reversed)

### Step 3 — Seller Rejects Refund (Second test run)

Set up a new order in `shipped` state and repeat Steps 1 above, then:

1. Seller A → Orders → Order A → Review Refund Request → **Reject**
2. Enter rejection reason
3. **Expected:**
   - Order status = `refund_rejected`
   - Buyer A receives rejection notification
   - If the mobile refund label still looks mixed-language, verify during owner test rather than treating it as an accepted caveat.

### Step 4 — Buyer Escalates to Dispute

(After seller rejection)

1. Buyer A → Orders → Order A → Escalate to Dispute
2. Provide dispute description and evidence
3. Submit
4. **Expected:** order status = `disputed`, admin panel shows new dispute

### Step 5 — Admin Reviews Dispute

1. Admin panel → Disputes → find the dispute
2. Review buyer and seller evidence panels
3. Choose resolution:
   - **Full refund to buyer** — full gateway refund initiated
   - **Partial refund** — enter partial amount, split escrow
   - **Reject buyer claim** — escrow released to seller

4. Submit resolution
5. **Expected:** order status = `completed` or `refund_approved` depending on resolution
6. Both parties receive resolution notification

**Pass criteria:** Full refund/partial/reject all result in correct final states; both parties notified; admin audit log entry created.

---

## Flow 15 — Buyer Refund Request (already covered in Flow 17 Step 1)

See Flow 17 above.

---

## Flow 16 — Seller Refund Decision (already covered in Flow 17 Steps 2–3)

See Flow 17 above.

---

## Flow 18 — Seller Withdrawal Request + Admin Approval

**Actor:** Seller A (has completed orders and a positive balance)

### Step 1 — Add Bank Account

1. Seller A → Profile → Bank Accounts → Add
2. Fill in: bank name, account number, account holder name
3. Save
4. Admin panel → Seller Verification → mark the bank account reviewed for payout
5. If multiple bank accounts exist, set the reviewed one as the default bank account in the mobile UI

**Pass criteria:** Seller has a reviewed default bank account available for withdrawals.

### Step 2 — Request Withdrawal

1. Seller Dashboard → Earnings → Request Withdrawal
2. Enter withdrawal amount (must be ≥ minimum withdrawal set in Platform Config)
3. Submit the request against the reviewed default bank account
4. If the default bank account is not reviewed for payout, verify the request is blocked with `BANK_ACCOUNT_NOT_REVIEWED` and the current Indonesian error copy if shown
5. If the app allows switching accounts, select another reviewed bank account and resubmit
6. **Expected:** withdrawal request created with `pending` status

### Step 3 — Admin Approves

1. Admin panel → Withdrawals → find the request
2. Review seller info and amount
3. **Approve**, **Reject with reason**, or **Process** the payout path
4. **Expected (approve):** withdrawal status = `approved` / `processing`; seller receives notification
5. **Expected (reject):** withdrawal status = `rejected`; seller balance restored; seller notified

**Pass criteria:** Both approve and reject paths result in correct states and notifications.

**Negative path:** A withdrawal from an unreviewed default bank account must block with `BANK_ACCOUNT_NOT_REVIEWED` and the current Indonesian error copy if shown.

---

## Flow 19 — Seller Purchases and Activates a Promotion

**Actor:** Seller A (verified, subscription active, with an active listing or auction)

### Step 1 — Browse promotion packages

1. Seller Dashboard → Promotions → Beli Promosi
2. Available packages are listed (name, price, duration, max targets)
3. Select a package and tap Beli

### Step 2 — Complete payment

1. Midtrans payment page loads
2. Complete with sandbox card
3. **Expected:** promotion ownership created with `pending_activation` status; Seller A redirected back to promotion list

### Step 3 — Activate promotion on a listing

1. Seller Dashboard → Promotions → find the pending ownership
2. Tap Aktifkan
3. Select a target: choose an active listing (or auction)
4. Confirm
5. **Expected:** instance status = `active`; listing now appears in feed and search with "Dipromosikan" badge

### Step 4 — Verify promotion is visible to buyers

1. Log in as Buyer A
2. Scroll home feed — promoted listing card appears with "Dipromosikan" badge
3. Search for a listing keyword — promoted result appears at position 3 (injected sidecar)
4. Open Explore → Listing tab — "Listing Dipromosikan" section visible at top

**Pass criteria:** Promoted item visible with badge on feed, search, and Explore. Impression and click events recorded (verify via Step 5 below or admin analytics in Flow 20).

---

## Flow 20 — Admin Views Campaign Analytics

**Actor:** Admin (requires `promotion.campaign.view` capability)

### Step 1 — Grant analytics capability (first time only)

```sql
INSERT INTO user_capabilities (id, user_id, capability, granted_by, granted_at)
VALUES (gen_random_uuid(), '<admin-uuid>', 'promotion.campaign.view', NULL, NOW());
```

### Step 2 — View campaign list

1. Admin panel → Promotions → Campaigns
2. Campaign list shows: seller username, package name, status, start/end time, target
3. Use filters: status (active / expired / stopped) and date range

### Step 3 — View analytics for a campaign

1. Find the campaign from Flow 19 (Seller A's active promotion)
2. Click "Analytics" button on the campaign row
3. Analytics modal shows:
   - Total impressions / clicks
   - Feed impressions, feed clicks
   - Search impressions, search clicks
   - CTR percentage

4. **Expected (if buyer viewed/tapped in Flow 19):** impression count ≥ 1, click count ≥ 1

### Step 4 — Admin force-stop (optional)

1. Find an active campaign in admin
2. Click "Stop" → confirm
3. **Expected:** campaign status = `stopped`; promotion no longer injected into feed/search

**Pass criteria:** Analytics data loads correctly; counts reflect buyer interactions from Flow 19.

---

## Additional Flows to Spot-Check

| Flow | Where | What to Verify |
|---|---|---|
| Create/edit discount code | Seller → Discounts | Code appears, applies at checkout |
| Report content | Any content → ⋯ → Report | Report submitted, moderation case appears in admin |
| Block user | User profile → Block | Content from blocked user no longer appears |
| In-app chat | Any order → Chat | Messages sent/received, order status banner shows |
| Notification settings | Profile → Settings → Notifications | Toggle push notification categories |
| Search | Search bar | Listings, auctions, users, content all return results |
| Admin Platform Config | Admin → Platform Config | Read financial config values; edit general config |
| Admin User Management | Admin → Users | Suspend, activate, view audit log |
| Admin Alerts | Admin → Alerts | Alert list loads, resolve/acknowledge action works |
| Support ticket | Profile → Help → Chat with Support | Ticket created, admin can claim and respond |

---

## Accepted P3 Ledger

These items are accepted for owner test. If any of them needs confirmation in a live run, verify during owner test rather than escalating it as a blocker.

| # | Area | P3 item |
|---|---|---|
| P3-1 | Mobile moderation flow | Full mobile appeal and warning screens are parked. |
| P3-2 | Warning acknowledgement | Warning acknowledge flow is parked. |
| P3-3 | Appeal timing | Appeal deadline handling is parked. |
| P3-4 | Listing / auction moderation | Moderation tombstone handling is parked. |
| P3-5 | Ratings | `getRatingForOrder` remains V1 for now. |
| P3-6 | Admin discount governance | Discount governance remains parked. |
| P3-7 | Coins seller earn | Seller registration / renewal earn is parked. |
| P3-8 | Coins buyer earn/refund | Buyer notification earn / refund is parked. |
| P3-9 | Coins expiry | Coin expiry policy is parked. |
| P3-10 | Flutter linting | Flutter analyzer info lints are parked. |

---

## Verdict Recording

After completing the flows, record:

```
OWNER TEST RUN — <DATE>
Tester: <name>
Environment: local / staging

FLOW RESULTS:
Flow 1  Sign Up         [ PASS / FAIL / SKIP ]
Flow 2  Sign In Email   [ PASS / FAIL / SKIP ]
Flow 3  Sign In Google  [ PASS / FAIL / SKIP ]
Flow 4  Complete Profile [ PASS / FAIL / SKIP ]
Flow 5  Become Seller   [ PASS / FAIL / SKIP ]
Flow 6  Submit KTP      [ PASS / FAIL / SKIP ]
Flow 7  Admin Verify    [ PASS / FAIL / SKIP ]
Flow 8  Create Listing  [ PASS / FAIL / SKIP ]
Flow 9  Create Auction  [ PASS / FAIL / SKIP ]
Flow 10 Buy Now         [ PASS / FAIL / SKIP ]
Flow 11 Place Bid       [ PASS / FAIL / SKIP ]
Flow 12 Auction Order   [ PASS / FAIL / SKIP ]
Flow 13 Ship Order      [ PASS / FAIL / SKIP ]
Flow 14 Confirm Receipt [ PASS / FAIL / SKIP ]
Flow 17 Refund + Dispute [ PASS / FAIL / SKIP ]
Flow 18 Withdrawal      [ PASS / FAIL / SKIP ]
Flow 19 Promotion        [ PASS / FAIL / SKIP ]
Flow 20 Admin Analytics  [ PASS / FAIL / SKIP ]

FAILURES (if any): <describe>
P0/P1 ITEMS OBSERVED: <list any blocking issues>
P2 ITEMS LOGGED: <list any non-blocking issues>
P3 ITEMS OBSERVED: <list which accepted P3 items were encountered>
OVERALL VERDICT: [ OWNER_TEST_PASSED / NEEDS_FOLLOW_UP ]
```
