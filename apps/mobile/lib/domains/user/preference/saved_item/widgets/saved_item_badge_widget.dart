import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/src/theme/app_theme.dart';
import 'package:labuda/domains/user/preference/saved_item/data/providers/saved_item_query_providers.dart';

/// Saved-items count badge for the app bar.
///
/// Driven by [savedItemsCountProvider] — the canonical count seam that
/// CommerceSavedItemActionButton invalidates after every save/unsave — so the
/// badge updates live instead of freezing the value fetched once in
/// initState, and the repository always arrives through the DI provider
/// (apiClientProvider) instead of a private ApiClient().
class SavedItemBadgeWidget extends ConsumerStatefulWidget {
  final Widget child;

  const SavedItemBadgeWidget({super.key, required this.child});

  @override
  ConsumerState<SavedItemBadgeWidget> createState() =>
      _SavedItemBadgeWidgetState();
}

class _SavedItemBadgeWidgetState extends ConsumerState<SavedItemBadgeWidget> {
  @override
  Widget build(BuildContext context) {
    final count = ref.watch(savedItemsCountProvider).asData?.value ?? 0;
    final colorScheme = Theme.of(context).colorScheme;
    if (count <= 0) {
      return widget.child;
    }

    return Stack(
      clipBehavior: Clip.none,
      children: [
        widget.child,
        Positioned(
          right: -6,
          top: -6,
          child: Container(
            padding: EdgeInsets.symmetric(
              horizontal: count > AppMetrics.p99
                  ? AppMetrics.p3
                  : count > AppMetrics.p9
                  ? AppMetrics.p4
                  : AppMetrics.p4,
              vertical: AppMetrics.p2,
            ),
            decoration: BoxDecoration(
              color: context.statusColors.error,
              borderRadius: BorderRadius.circular(AppShape.r10),
              boxShadow: [
                BoxShadow(
                  color: colorScheme.shadow.withValues(alpha: 0.2),
                  blurRadius: 3,
                  offset: const Offset(0, 1),
                ),
              ],
            ),
            constraints: const BoxConstraints(minWidth: 16, minHeight: 16),
            child: Text(
              count > 99 ? '99+' : count.toString(),
              style: TextStyle(
                color: colorScheme.onError,
                fontSize: AppType.s9,
                fontWeight: FontWeight.w600,
                height: 1.1,
              ),
              textAlign: TextAlign.center,
            ),
          ),
        ),
      ],
    );
  }
}
