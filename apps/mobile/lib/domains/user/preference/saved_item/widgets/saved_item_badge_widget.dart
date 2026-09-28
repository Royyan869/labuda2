import 'package:flutter/material.dart';
import 'package:labuda/core/src/theme/app_theme.dart';
import 'package:labuda/domains/user/preference/saved_item/data/repositories/saved_item_repository.dart';
import 'package:labuda/domains/user/preference/saved_item/data/services/saved_item_service.dart';

class SavedItemBadgeWidget extends StatefulWidget {
  final Widget child;

  const SavedItemBadgeWidget({super.key, required this.child});

  @override
  State<SavedItemBadgeWidget> createState() => _SavedItemBadgeWidgetState();
}

class _SavedItemBadgeWidgetState extends State<SavedItemBadgeWidget> {
  late final SavedItemService _savedItemService;
  late final Future<int> _countFuture;

  @override
  void initState() {
    super.initState();
    _savedItemService = SavedItemService(repository: SavedItemRepository());
    _countFuture = _savedItemService.getSavedItemsCount();
  }

  @override
  Widget build(BuildContext context) {
    return FutureBuilder<int>(
      future: _countFuture,
      builder: (context, snapshot) {
        final count = snapshot.data ?? 0;
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
      },
    );
  }
}
