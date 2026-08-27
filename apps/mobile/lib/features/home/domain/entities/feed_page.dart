import 'feed_item.dart';

/// Single page of feed results returned by the repository.
///
/// Carries the authoritative `hasMore` signal from the backend so the
/// notifier no longer has to derive exhaustion from `items.length < limit`
/// or `newItems.isEmpty` — both heuristics diverge from backend authority
/// once the response is post-filtered (future feed evaluator) or short
/// at the exact boundary.
///
/// `nextCursor` is exposed so the notifier may guard against the same
/// cursor being returned twice in a row (defensive infinite-loop guard).
/// The cursor itself remains opaque — UI must not parse or interpret it.
class FeedPage {
  final List<FeedItem> items;
  final bool hasMore;
  final String? nextCursor;

  const FeedPage({required this.items, required this.hasMore, this.nextCursor});
}
