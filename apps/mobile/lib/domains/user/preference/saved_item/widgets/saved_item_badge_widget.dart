import 'package:flutter/material.dart';
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
                  horizontal: count > 99
                      ? 3
                      : count > 9
                      ? 4
                      : 4,
                  vertical: 2,
                ),
                decoration: BoxDecoration(
                  color: Colors.red[600],
                  borderRadius: BorderRadius.circular(10),
                  boxShadow: [
                    BoxShadow(
                      color: Colors.black.withValues(alpha: 0.2),
                      blurRadius: 3,
                      offset: const Offset(0, 1),
                    ),
                  ],
                ),
                constraints: const BoxConstraints(minWidth: 16, minHeight: 16),
                child: Text(
                  count > 99 ? '99+' : count.toString(),
                  style: const TextStyle(
                    color: Colors.white,
                    fontSize: 9,
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
