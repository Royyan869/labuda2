import 'package:flutter/material.dart';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:labuda/domains/user/preference/saved_item/data/providers/saved_item_query_providers.dart';
import 'package:labuda/shared/widgets/count_badge.dart';

/// Saved-items count badge for the app bar.
///
/// Driven by [savedItemsCountProvider] — the canonical count seam that
/// CommerceSavedItemActionButton invalidates after every save/unsave — so the
/// badge updates live instead of freezing a value fetched once in initState,
/// and the repository always arrives through the DI provider instead of a
/// private ApiClient(). Layout belongs to the one renderer
/// [CountBadgeOverlay]; this file used to carry its own copy of the whole
/// Stack, and that copy is dead.
class SavedItemBadgeWidget extends ConsumerWidget {
  const SavedItemBadgeWidget({super.key, required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final count = ref.watch(savedItemsCountProvider).asData?.value ?? 0;
    return CountBadgeOverlay(count: count, child: child);
  }
}
