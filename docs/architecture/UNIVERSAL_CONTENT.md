# Universal Content — Canonical Architecture

## Status

`CANONICAL` — this document is the single authoritative description of the Content domain.

## Business Truth

Labuda has one universal social entity: **Content**.

There is no Post, Request, Content subtype, fulfilled Content, or fulfillment action.

Content meaning comes from caption, media, hashtags, mentions, comments, and community context — not from a type classification.

## Schema

### `contents` table

| Column | Type | Notes |
|--------|------|-------|
| `id` | `uuid` | PK |
| `author_id` | `uuid` | FK → `users(id)` |
| `status` | `content_status_enum` | `active`, `deleted` |
| `caption` | `text` | |
| `city` | `text` | Optional location |
| `province` | `text` | Optional location |
| `visibility` | `content_visibility_enum` | `public`, `followers_only`, `private` |
| `is_hidden` | `boolean` | Moderation flag |
| `original_author_id` | `uuid` | Repost attribution |
| `share_reference` | `jsonb` | Repost/share target reference |
| `created_at` | `timestamptz` | |
| `updated_at` | `timestamptz` | |
| `deleted_at` | `timestamptz` | Soft delete |
| `search_vector` | `tsvector` | Full-text search |

### `content_status_enum`

```sql
CREATE TYPE content_status_enum AS ENUM ('active', 'deleted');
```

Transitions: `active → deleted` (terminal).

### `comments` table — Comment Commerce

| Column | Type | Notes |
|--------|------|-------|
| `id` | `uuid` | PK |
| `author_id` | `uuid` | FK → `users(id)` |
| `body` | `text` | |
| `type` | `comment_type_enum` | `normal`, `listing_reference` |
| `fixed_price_sale_id` | `uuid` | FK → `fixed_price_sales(id)`, only for `listing_reference` |
| `target_id` | `uuid` | Content ID |
| `target_type` | `comment_target_type_enum` | Always `content` |
| `parent_id` | `uuid` | For replies (max depth 1) |

Constraints:
- `comments_fixed_price_sale_id_fkey` — FK to `fixed_price_sales(id)` ON DELETE RESTRICT
- `comments_listing_ref_consistency_check` — `normal` comments have NULL `fixed_price_sale_id`; `listing_reference` comments have NOT NULL

## API

### Create Content

```
POST /api/v1/contents
```

Required: `caption`, `Idempotency-Key` header.
Optional: `visibility`, `media`, `tags`, `location` (city/province), `share_reference`.

**Strict JSON**: unknown fields (including old `type`, `fulfilled_at`, `fulfilled_by`) are rejected with 400.

### Response

Content responses do not contain a `type` field. Status is the coarsened public lifecycle (`active`/`removed`).

## Forbidden Resurrection Patterns

The following must never appear in source, schema, tests, routes, or active documentation:

- `TypePost`, `TypeRequest`, `ContentTypeContent`
- `StatusFulfilled`, `FulfillRequest`, `PostCannotBeFulfilled`
- `IsFulfillable`, `CanBeFulfilled`, `WasFulfilled`
- `content_type_enum`, `contents.type`, `fulfilled_at`, `fulfilled_by`
- `idx_contents_type`, `contents_fulfilled_by_fkey`
- `POST /api/v1/contents/:id/fulfill`
- `FeedItem.Type`, `'content' AS type`, `AS _unused`, `_unusedType`
- `comments.share_reference`, `listing_origin_enum`
- `request_context`, `chat_context`, `direct_create`
- Notification `targetType: post` or `targetType: request`
- `postId`, `requestId`

## Comment Commerce Boundary

- Comments reference listings via `fixed_price_sale_id` FK — never via JSONB `share_reference`
- Listing/fixed_price_sale is the product authority
- Comment response carries live Listing projection
- Seller may attach a Listing to any commentable Content
- Event: `comment.listing_reference.created`

## Negative Contracts (Enforced)

1. Create Content without `type` — succeeds
2. Old `{"type":"post"}` — rejected (400, strict JSON)
3. Old `{"type":"request"}` — rejected (400, strict JSON)
4. Content detail response — no `type` key
5. Feed response — no `type` key
6. Search response — no `type` key
7. Public card — no Content subtype
8. `POST /contents/:id/fulfill` — route not registered
9. Service `CreateContent` — no `contentType` parameter
10. Repository `ListFeed` — no `filterType` parameter

## Migration

`000001_canonical_schema.up.sql` directly creates the final schema. No migration creates Post/Request, Comment JSONB, or Listing origin objects and removes them later.
