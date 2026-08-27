# Dev Seed Guide

**Status: STABLE**
Last reviewed: 2026-07-03

> **Canonical accounts:** Owner visual/internal test uses **only** the 3 Gmail
> accounts defined in [`owner-test-runtime.md`](../owner-test-runtime.md). The
> `buyer.a@test.com` / `seller.a@test.com` / `admin@test.com` style emails below
> are flow-structure placeholders for building out seed *states* (order states,
> dispute states, etc.) — substitute the real owner-test accounts, or throwaway
> accounts you create ad hoc, as needed. `*.test.local` (Step 3) are separate
> `cmd/seed` fixtures, also not owner-test accounts.

Step-by-step instructions for creating seed data in a local or staging environment so all owner-test flows can be executed.

---

## Prerequisites

| Requirement | Notes |
|---|---|
| Docker running | PostgreSQL + Redis via `docker-compose up -d` from repo root |
| Migrations applied | `cd backend && go run ./cmd/migrate` — applies canonical baseline (`000001_canonical_schema`) |
| Backend server running | `cd backend && go run ./cmd/core_server` (works on Windows and everywhere else). If you prefer Make on Linux/Mac, `cd backend && make run` is also available. Run migrations first. |
| Admin panel running | `cd apps/admin && npm run dev` (port 5173) |
| Mobile app (real Android device) | Flutter debug build: edit `apps/mobile/lib/core/api/config/api_config.dart` dev URLs to your LAN IP (see current value in `owner-test-runtime.md`, e.g. `http://192.168.1.7:8080` — re-check with `ipconfig` if it has changed), then `flutter run -d <device-id>`. Do NOT use `localhost` or `10.0.2.2` for real devices — those only work on emulators. |
| Mobile app (Android emulator) | Replace LAN IP with `10.0.2.2` in api_config.dart dev URLs, then `flutter run`. |
| Windows Firewall (real device) | Run in **Administrator** PowerShell: `netsh advfirewall firewall add rule name="Labuda Backend Port 8080" dir=in action=allow protocol=TCP localport=8080 profile=any`. Required if Wi-Fi profile is Public. |
| Firebase project | Real Firebase credentials for auth — `DEV_MOCK_FIREBASE_AUTH=true` bypasses token validation for local only |
| Midtrans sandbox | Set `MIDTRANS_SERVER_KEY` and `MIDTRANS_CLIENT_KEY` to sandbox values in backend-only env files; mobile docs/env must use client-safe values only. Webhook delivery needs a public URL or the dev hot-arm/manual path. |

---

## Step 1 — Start Infrastructure

```bash
# From repo root
docker-compose up -d

# Verify
docker ps  # should show postgres and redis containers
```

---

## Step 2 — Run Migrations

```bash
cd backend
go run ./cmd/migrate
```

Expected last line: `Migration complete. Current version: 212`.
Confirm the replay reached `schema_migrations.max(version) = 212` and includes `000212_add_admin_audit_logs`.

Migrations 000175 and 000176 auto-seed platform configs and seller subscription configs — no manual step needed.

---

## Step 3 — Run Reference Data Seeder

```bash
cd backend
go run ./cmd/seed
```

> **Warning printed by the seeder:** "THIS SCRIPT BYPASSES BUSINESS LOGIC AND EVENT SYSTEM". This is expected.
>
> The seeder creates: 3 fixed test users (buyer/seller/admin), 25 content items, 15 comments, and 2 follow relationships. Users use fixed UUIDs (`000...001`, `000...002`, `000...003`) for local auth bypass. Platform configs and seller subscription configs are seeded by migrations 000175/000176 — no separate step needed.
>
> The seeder is idempotent for users and admin capabilities (upsert). Content, comments, and follows are appended on each run (random IDs). Running seed twice gives 50 content items, which is fine.

---

## Step 4 — Create Admin User

Follow [admin-bootstrap.md](./admin-bootstrap.md) to create the first admin user. You need at minimum:

- One super-admin Firebase account for owner testing
- All 38 capabilities granted

---

## Step 5 — Create Test User Accounts

> **Note:** `cmd/create_test_users` does not exist. The `go run ./cmd/seed` in Step 3 already creates 3 fixed test accounts (`buyer@test.local`, `seller@test.local`, `admin@test.local`) with known UUIDs for local auth bypass. For real Firebase auth flows, create accounts manually through the mobile app.

For owner test, use the 3 canonical accounts for the primary roles:

| Role | Owner-test account | Notes |
|---|---|---|
| Buyer A | `nurroyyan89@gmail.com` (username `royyan`) | Standard buyer, no seller capability |
| Seller A | `qiqiaja7799@gmail.com` (username `qiqijho`) | Must complete seller onboarding (Step 6) and have a reviewed default bank account before withdrawal testing |
| Admin | `busiyono79@gmail.com` (username `yayan`) | Bootstrap via `admin-bootstrap.md` |

Some flows need a second buyer or seller (dispute counterparty, bid competition,
search/browse with multiple sellers). These are **not** pre-provisioned owner-test
identities — sign up a throwaway account through the mobile app for the duration
of that flow only:

| Ad hoc role | Notes |
|---|---|
| Buyer B | Used as dispute counterparty / bid competition only |
| Seller B | Second seller for search/browse flows only |

---

## Step 6 — Seller Onboarding (Seller A)

Log in as Seller A in the mobile app.

### 6a — Pay Seller Subscription

1. Profile → Settings → Upgrade to Seller
2. Complete the seller wizard (username if missing, bio, phone number, sender address, then store/farm name)
3. Proceed to subscription payment
4. Use Midtrans sandbox card: `4811 1111 1111 1114` / exp: `01/39` / CVV: `123`
5. Webhook delivers payment confirmation (requires ngrok or manual trigger)

**Manual webhook trigger (local dev without public URL):**
```bash
# Get the pending payment ID from DB
SELECT id, external_id, status FROM seller_subscription_payments
ORDER BY created_at DESC LIMIT 1;

# Call the dev-only hot-arm endpoint (only works in DEV_MODE=true)
POST http://localhost:8080/dev/webhooks/midtrans/arm
{"external_id": "<external_id>"}
```

### 6b — Submit KTP Verification

1. Profile → Seller Dashboard → Submit Verification
2. Upload a placeholder KTP image (any JPEG)
3. Fill in the KTP number and full name
4. Submit — status becomes `pending_review`

### 6c — Admin Approves Verification

1. Log in to admin panel
2. **Seller Verification** → find Seller A → **Approve**
3. Seller A's status becomes `verified`

### 6d — Review Seller Bank Account for Payout

1. Seller A adds at least one active bank account in the mobile app
2. Admin marks that bank account reviewed for payout
3. If Seller A has multiple bank accounts, set the reviewed one as the default before withdrawal testing
4. Verify the withdrawal path uses the reviewed default bank account

---

## Step 7 — Create Listings and Auctions

Log in as Seller A.

### 7a — Fixed-Price Listing

1. My Listings → Create Listing
2. Fill in: title, description, price (e.g., Rp 500.000), stock (5), category (Koi), photos
3. Submit — status becomes `active`

Create at least **3 active listings** for search/browse flow.

### 7b — Auction

1. My Listings → Create Auction
2. Fill in: title, starting bid (e.g., Rp 200.000), duration (choose shortest available), photos
3. Submit — status becomes `scheduled`

The auction transitions to `active` when the scheduled start time passes (AuctionWorker runs every minute).

---

## Step 8 — Order States for Owner Test Flows

Create orders in multiple states so all order-path flows can be tested without waiting for real timelines.

### 8a — Pending Payment Order

1. Log in as Buyer A
2. Add Buyer A's shipping address (Profile → Addresses)
3. Go to any active listing → Buy Now → Checkout
4. Do NOT complete payment — leave the Midtrans payment page
5. Order is now in `pending_payment` state

### 8b — Paid / Awaiting Shipment Order

1. Start a new checkout (separate listing)
2. Complete Midtrans sandbox payment
3. Order transitions to `paid` / awaiting shipment after webhook delivery

### 8c — Shipped Order

1. Log in as Seller A
2. Orders → find the paid order → Ship → enter tracking number (any string)
3. Order status: `shipped`

### 8d — Completed Order

1. Log in as Buyer A
2. Orders → find the shipped order → Confirm Receipt
3. Order status: `completed`
4. Seller earnings credited to seller balance

### 8e — Order for Refund Flow

1. Complete another order through `shipped` state (do not confirm receipt yet)
2. Log in as Buyer A → Orders → Refund Request → fill in reason → submit
3. Order status: `refund_requested`

### 8f — Order for Dispute Flow

1. Have a refund-requested order
2. Log in as Seller A → reject the refund request
3. Log in as Buyer A → escalate to dispute
4. Order status: `disputed`

---

## Step 9 — Admin Dispute Resolution

Ensure at least one order is in `disputed` state (Step 8f above).

In the admin panel: **Disputes** → find the dispute → Review evidence → Resolve (full refund / partial / reject buyer claim).

---

## Step 10 — Seller Withdrawal

Ensure Seller A has completed at least one order (Step 8d). The seller balance should show earnings.

1. Log in as Seller A → Seller Dashboard → Earnings → Request Withdrawal
2. Add a bank account first if not done (Profile → Bank Accounts), then wait for admin review so it becomes the default withdrawal bank
3. Submit withdrawal request against the reviewed default bank account

In admin panel: **Withdrawals** → find the request → Approve, Reject, or Process.

If the withdrawal is submitted from an unreviewed default bank account, the owner-test script should expect `BANK_ACCOUNT_NOT_REVIEWED` and the current Indonesian copy if shown.

---

## Owner Test Flow Checklist

| # | Flow | Seed Required | State to Prepare |
|---|---|---|---|
| 1 | Sign Up (email) | Fresh email not in DB | — |
| 2 | Sign In (email) | Buyer A account | completed |
| 3 | Sign In (Google) | Google Firebase account | — |
| 4 | Complete Profile | Any account with incomplete profile | — |
| 5 | Become Seller (subscription) | Fresh seller account | Step 6a |
| 6 | Submit Verification | Seller account with active subscription | Step 6b |
| 7 | Admin Approve/Reject Verification | Pending verification in DB | Step 6b |
| 8 | Create Listing | Verified seller with reviewed default bank account | Step 6c + 6d + 7a |
| 9 | Create Auction | Verified seller with reviewed default bank account | Step 6c + 6d + 7b |
| 10 | Buy Now / Checkout | Active listing + buyer with address | Step 8b |
| 11 | Place Bid | Active auction | Step 7b + Buyer B |
| 12 | Auction Settlement | Ended auction with winning bid | Step 7b wait |
| 13 | Seller Ships Order | Order in `paid` state | Step 8b |
| 14 | Buyer Confirms Receipt | Order in `shipped` state | Step 8c |
| 15 | Buyer Refund Request | Order in `shipped` state | Step 8e |
| 16 | Seller Refund Decision | Order in `refund_requested` state | Step 8e |
| 17 | Dispute + Admin Resolution | Order in `disputed` state | Step 8f + Step 9 |
| 18 | Seller Withdrawal + Admin Approval | Seller with reviewed default bank account | Step 10 |

---

## Useful SQL Queries

```sql
-- Check seller subscription status
SELECT u.email, ss.status, ss.expires_at
FROM seller_subscriptions ss
JOIN users u ON u.id = ss.seller_id
ORDER BY ss.created_at DESC;

-- Check order states
SELECT o.id, o.status, o.order_number, u.email as buyer_email
FROM orders o JOIN users u ON u.id = o.buyer_id
ORDER BY o.created_at DESC
LIMIT 20;

-- Check seller balance
SELECT u.email, w.balance
FROM wallets w JOIN users u ON u.id = w.user_id
WHERE u.email LIKE '%seller%';

-- Force auction to active (bypass schedule — local dev only)
UPDATE auctions
SET status = 'active', started_at = NOW(), ends_at = NOW() + INTERVAL '1 hour'
WHERE id = '<AUCTION_UUID>'
AND status = 'scheduled';
```

---

## Known Gaps (No Blocker)

- **No automated seed script** for buyer/seller/order states — all must be created manually via the app or SQL. Creating orders via SQL requires bypassing business logic; the app path is preferred.
- **Midtrans webhook** requires a public URL unless `DEV_MODE=true` and the dev hot-arm endpoint is used.
- **Auction timing** — the shortest auction duration is controlled by `platform_configs`. Check the value and adjust test expectations, or use the SQL shortcut above for local dev.
