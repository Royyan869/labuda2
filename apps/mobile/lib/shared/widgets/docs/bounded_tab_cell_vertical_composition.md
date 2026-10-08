# Bounded Tab-Cell Vertical Composition — Canonical Contract

**Status:** LOCKED
**Scope:** Mobile UI foundation (`apps/mobile/lib`)
**Authority:** Flutter framework source (`SliverFillRemaining`) + this contract
**Enforced by:** `test/shared/widgets/bounded_tab_cell_vertical_composition_contract_test.dart`

---

## 0. The truth in one line

A bounded tab cell has **ONE** vertical scroll owner. Fixed/intrinsic chrome and
every semantic state (content, loading, error, empty) live **inside that same
scroll view**. Non-scrollable states are declared with
`SliverFillRemaining(hasScrollBody: false)`.

This is not a style preference. It is the only composition that cannot clamp a
state smaller than its intrinsic height when the remaining viewport is short —
which is the exact class of failure that produced the Profile Reviews
`BOTTOM OVERFLOWED BY 30 PIXELS` device symptom.

---

## 1. Framework proof (why `hasScrollBody` is the whole rule)

From `packages/flutter/lib/src/rendering/sliver_fill.dart`:

```
hasScrollBody: true  → RenderSliverFillRemainingWithScrollable
    extent = remainingPaintExtent
    child.layout(minExtent = extent, maxExtent = extent)   // TIGHT clamp
    geometry.scrollExtent = viewportMainAxisExtent         // fixed viewport

hasScrollBody: false → RenderSliverFillRemaining
    extent = max(viewportMainAxisExtent - precedingScrollExtent,
                 child.getMaxIntrinsicHeight(crossAxisExtent))
    child.layout(minExtent = extent, maxExtent = extent)   // defer to intrinsic
    geometry.scrollExtent = extent                          // grows, outer scrolls
```

Consequences:

- A **scrollable** child works under `true`: it is given the leftover viewport
  height and scrolls inside it. `true` is intended for exactly this.
- A **non-scrollable** child taller than the leftover **overflows** under
  `true`, because it is force-clamped to `remainingPaintExtent`.
- A **non-scrollable** child under `false` is given
  `max(remaining, intrinsic)`; the sliver's scroll extent grows and the outer
  scroll view reveals the full state. It can never overflow or clip.
- `false` is **illegal** on a scrollable child (its intrinsic height is
  infinite → framework assert).

The framework's own `NestedScrollView` uses `SliverFillRemaining(hasScrollBody:
true)` to wrap its scrollable inner body
(`packages/flutter/lib/src/widgets/nested_scroll_view.dart`). That is the
reference legitimate use of `true`.

---

## 2. Rules

### Rule 1 — Scroll owner

Inside a bounded tab cell, the `CustomScrollView` / `ListView` that is the
cell's body is the single vertical scroll owner. Chrome and semantic states
belong to that one scroll composition. Do not introduce a second vertical
scrollable.

### Rule 2 — Non-scrollable state

For a non-scrollable child, `hasScrollBody: false` is **required**:

```dart
SliverFillRemaining(
  hasScrollBody: false,
  child: ...,
)
```

Canonical non-scrollable children:

- `EmptyState`
- `LoadingIndicator` / `Center(child: LoadingIndicator())`
- `PageErrorState`
- `Center > Column` (e.g. a bespoke empty state)
- any other non-scrollable semantic state box

### Rule 3 — Scrollable child exception

`hasScrollBody: true` is valid **only** when the child is itself scrollable and
owns a scroll position (e.g. the `NestedScrollView` inner body). `true` is not
"generally forbidden" — it is simply the wrong choice for a non-scrollable
child.

### Rule 4 — Short viewport

A non-scrollable state must never be forced into a remaining viewport smaller
than its intrinsic height. The canonical primitive (`hasScrollBody: false`)
grows the scroll extent instead. No fixed height, padding, padding-compensation,
font shrink, or extra scroll wrapper may be used to suppress the symptom.

### Rule 5 — Empty / loading / error

When the parent body is a `CustomScrollView` inside a bounded tab cell, the
loading, error, and empty states that are part of that body participate in the
same scroll composition — as `SliverFillRemaining(hasScrollBody: false)` states.
The semantic renderers themselves (`EmptyState`, `LoadingIndicator`,
`PageErrorState`) are unchanged and remain pure renderers.

### Rule 6 — NestedScrollView

When `NestedScrollView` coordinates outer header scrolling with an inner tab
`CustomScrollView`, that outer/inner ownership is framework-coordinated. Do not
introduce another vertical scroll owner. Inner state slivers still obey Rule 2.

---

## 3. Canonical composition

```text
Scaffold / TabBarView cell
└── CustomScrollView                         (Rule 1: the ONE scroll owner)
    ├── SliverToBoxAdapter(chrome)           (sub-tabs / summary / filter)
    ├── SliverFillRemaining(hasScrollBody: false, child: LoadingIndicator)   (Rule 5)
    ├── SliverFillRemaining(hasScrollBody: false, child: PageErrorState)     (Rule 5)
    ├── SliverFillRemaining(hasScrollBody: false, child: EmptyState)         (Rule 2/5)
    └── SliverList(...)                      (actual scrollable content)
```

Reference implementations (canonical): `CommerceMarketplaceGrid`
(`lib/domains/commerce/catalog/shared/presentation/widgets/commerce_marketplace_primitives.dart`),
`order_list_screen.dart`, `seller_auctions_screen.dart`, `my_for_sales_screen.dart`,
`support_tickets_list_screen.dart`, `my_reports_screen.dart`,
`follow_list_screen.dart`, `address_list_screen.dart`.

---

## 4. Legitimate `hasScrollBody: true`

Only when the sliver child is itself scrollable:

```dart
SliverFillRemaining(              // scrollable child owns its own scroll
  hasScrollBody: true,
  child: ListView(...),
)
```

The framework's `NestedScrollView` inner-body wrapper is the canonical example.
No other justification is accepted.

---

## 5. Non-goals

- No new widget, helper, or generic "TabCell"/"SliverStateContainer".
- No fixed heights, padding tweaks, font/icon shrinks, or extra scroll wrappers
  to hide an overflow.
- No change to `EmptyState`, `LoadingIndicator`, `PageErrorState`,
  `BottomActionBar`, the Safe Area foundation, or the BottomSheet foundation.
- No global "replace every `SliverFillRemaining` with false".
