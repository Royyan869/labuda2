# `backend/validation/` — Development SQL Validation Flows

These are **developer-only** SQL validation flows and a query runner.

They are **not** runtime source, not part of any production binary, and not
imported by any other package.

## Contents

| File | Purpose |
|---|---|
| `flow1_negotiation_chat_order_notification.sql` | Validates negotiation→chat→order→notification chain |
| `flow2_moderation_content_effect.sql` | Validates moderation effects on content |
| `flow3_moderation_listing_order_safety.sql` | Validates moderation effects on listing/order safety |
| `flow4_retry_idempotency_validation.sql` | Validates retry and idempotency contracts |
| `flow5_outbox_health_check.sql` | Validates outbox queue health |
| `query_db.go` | Helper Go program to run SQL flows against a local dev DB |
| `run_all_flows.ps1` | Windows runner script |
| `run_all_flows.sh` | Unix runner script |

## NOT the runtime source

Do not patch SQL queries here to fix production bugs. Runtime SQL is in the
service/repository layer under `backend/internal/`.

## Canonical patch target

Fix production SQL in the relevant repository implementation:
`backend/internal/<domain>/infrastructure/repository/`.
