/// SINGLE SOURCE OF TRUTH for marketplace grid & card geometry.
///
/// Owner-locked (2026-09-27):
///   - card radius 12 — aligned with `AppShape.r12`, feed cards, search
///     results (`BaseCard` was purged as obsolete and is no longer a reference);
///   - grid: edge padding 12, gap 8 (edge > gap so the rhythm stays even);
///   - content rhythm inside the card: padding 8, gap 8;
///   - media frame fixed 4:5 rendered with `BoxFit.contain` so portrait AND
///     landscape koi photos stay fully visible;
///   - title capped at one line.
///
/// WHY THIS FILE EXISTS: every marketplace card — For Sale, Auction, and the
/// promotion grid that reuses both — must render with identical geometry.
/// Surfaces read these values instead of hardcoding literals;
/// `marketplace_card_layout_contract_test.dart` fails when a literal drifts
/// back in.
class CommerceMarketplaceMetrics {
  const CommerceMarketplaceMetrics._();

  /// Outer corner radius of every marketplace card.
  static const double cardRadius = 12;

  /// Left/right (and top) padding of the 2-column grid.
  static const double gridEdgePadding = 12;

  /// Bottom padding of the grid before the safe-area inset.
  static const double gridBottomPadding = 16;

  /// Gap between cards, both axes. Also the target rhythm inside cards.
  static const double gridGap = 8;

  /// Vertical margin for STACKED list cards (feed). Grid rows apply [gridGap]
  /// once between cells, while a stacked list adds two neighbouring margins —
  /// so each card carries half a gap and adjacent cards still sit [gridGap]
  /// apart. This is the single answer to "jarak antar kartu feed".
  static const double stackedCardMargin = gridGap / 2;

  /// Padding around the text block under the media.
  static const double contentPadding = 8;

  /// Vertical gap between text rows inside the card.
  static const double contentGap = 8;

  /// Fixed media frame (width / height).
  static const double mediaAspectRatio = 4 / 5;

  /// Titles never wrap — long koi names ellipsize on discovery cards.
  static const int titleMaxLines = 1;

  /// Cell aspect (width / height) of the grid. Tuned so the cell matches the
  /// natural card height (4:5 media + one-line title + one-line value) with
  /// headroom for accessibility text scaling.
  static const double childAspectRatio = 0.58;
}
