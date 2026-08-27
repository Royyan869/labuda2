# Doctrine — Username Lifecycle (Immutable After Establishment)

> **Status:** CANONICAL v2
> **Scope:** every flow that creates, displays, or reads `username` — Foundation, Social, Search, Notification, Moderation.

## Canonical Wording

> *"Username adalah identitas kanonik pengguna. Ia dipilih saat registrasi dan tidak dapat diubah setelah ditetapkan."*

## Canonical Truth

- Username is selected during registration (email/password path) or at Complete Profile (Google path).
- Username is the **canonical user identity**.
- Username is **immutable after establishment**.
- Edit Profile / Settings cannot mutate username — the field is displayed read-only.
- Registration uses canonical local format validation (`CanonicalUsernameValidator` on mobile; `identityusername` on backend).
- The backend is the final authority for reserved / taken / acceptance.
- No anonymous / pre-auth availability endpoint exists; `/users/check-username` requires an authenticated session.
- `USERNAME_TAKEN` recovery retries the authenticated exchange with a corrected username **without** recreating the Firebase account.
- Empty username establishment through the canonical profile establishment path remains valid where a canonical consumer requires it.

## Lifecycle

| Operation | When | Rules that apply |
|-----------|------|------------------|
| **Initial establishment** | First time the username is set: at sign-up (email/password path) or at Complete Profile (Google path). | Uniqueness + format + reserved-list only. |
| **Rename** | Does not exist. There is no rename capability anywhere in the codebase. | N/A — immutable after establishment. |

## Backend Enforcement

- `PATCH /api/v1/users/me/profile` assigns a username only when the profile has no username yet.
- Once a username is set, any attempt to change it returns `409 USERNAME_ALREADY_SET` (mobile: `USERNAME_IMMUTABLE` at the exchange).
- The exchange path (`POST /api/v1/auth/firebase/exchange`) assigns the username exactly once; an existing user with a different username receives `409 USERNAME_IMMUTABLE`.
- This is a deliberate safety property: renaming would silently break identity continuity across moderation cases, disputes, and every other domain that resolves username live by `user_id`, since none of them capture a historical snapshot.

## What Must NOT Be Destroyed

The platform invariants below hold regardless of username immutability:

- social graph;
- trust graph;
- moderation traceability;
- historical attribution;
- seller reputation continuity (review, rating, listing aggregations stay attached to account, not handle).

## Cross-Domain Impacts

- **Search** — usernames resolve to the canonical account; because usernames never change after establishment, `@handle` references remain stable.
- **Mention (Chat / Comment / Post)** — `@handle` references in content remain stable because the username cannot change.
- **Seller** — store name / farm name is a separate entity from username; the personal username is immutable, and store name changes never mutate it.
- **Moderation** — historical handles are stable by construction because the username never changes.

## Forbidden Behaviors (doctrine-level)

- The system MUST NOT allow a username to change after it has been established.
- The system MUST NOT advertise a rename / change-username capability in Edit Profile, Settings, or any other surface.
- Edit Profile MUST display the username read-only and MUST NOT submit it as a mutation.
- The system MUST NOT reintroduce a local reserved-name list on mobile; reserved names are backend authority.

## Related Doctrine

- [Layered Identity & Trust Model](./layered-identity-trust-model.md) — username is the proof of Layer B.
- [Capability Matrix](./capability-matrix.md) — username affects display; trust authorities are not derived from handle.
