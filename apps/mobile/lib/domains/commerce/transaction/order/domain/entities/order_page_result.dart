library;

import 'order.dart';

/// Canonical paged order result from backend-backed list endpoints.
///
/// The backend owns pagination truth. The UI can only derive `hasMore` from the
/// returned page metadata, never from local list heuristics alone.
class OrderPageResult {
  final List<Order> orders;
  final int page;
  final int pageSize;
  final int? total;

  const OrderPageResult({
    required this.orders,
    required this.page,
    required this.pageSize,
    this.total,
  });

  bool get hasMore {
    if (pageSize <= 0) return false;
    if (total != null) {
      return page * pageSize < total!;
    }
    return orders.length >= pageSize;
  }
}
