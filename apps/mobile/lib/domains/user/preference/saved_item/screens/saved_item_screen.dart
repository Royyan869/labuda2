import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/navigation/navigation_provider.dart';
import 'package:labuda/shared/domain/entities/resource_projection.dart';
import 'package:labuda/shared/widgets/empty_state.dart';
import 'package:labuda/domains/user/preference/saved_item/models/saved_item_model.dart';
import 'package:labuda/domains/user/preference/saved_item/data/providers/saved_item_query_providers.dart';
import 'package:labuda/domains/user/preference/saved_item/data/repositories/saved_item_repository_provider.dart';
import 'package:labuda/shared/widgets/app_image.dart';
import 'package:labuda/core/src/theme/app_theme.dart';

/// Saved-items authority screen.
///
/// The list is driven by [savedItemsProvider] — the canonical seam that
/// CommerceSavedItemActionButton invalidates after every save/unsave — so the
/// page refreshes itself from ONE source instead of owning a private
/// repository instance and its own manual reload loop.
class SavedItemScreen extends ConsumerStatefulWidget {
  const SavedItemScreen({super.key});

  @override
  ConsumerState<SavedItemScreen> createState() => _SavedItemScreenState();
}

class _SavedItemScreenState extends ConsumerState<SavedItemScreen> {
  String? _selectedType;

  /// Saved-row thumbnail edge: content/media geometry, not a spacing gap —
  /// named (not a literal) so the geometry census reads one decision.
  static const double _thumbEdge = 56;

  Future<void> _removeItem(SavedItemModel item) async {
    try {
      await ref.read(savedItemRepositoryProvider).removeSavedItem(
        targetType: item.targetType == TargetType.forSale
            ? 'for_sale'
            : 'auction',
        targetId: item.targetId,
      );
      ref.invalidate(savedItemsProvider);
      ref.invalidate(savedItemsCountProvider);
    } catch (e) {
      // Handle error
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Disimpan'),
        actions: [
          PopupMenuButton<String?>(
            initialValue: _selectedType,
            onSelected: (value) {
              // Client-side filter over the canonical saved-items list.
              setState(() => _selectedType = value);
            },
            itemBuilder: (context) => [
              const PopupMenuItem(value: null, child: Text('Semua')),
              const PopupMenuItem(value: 'for_sale', child: Text('ForSale')),
              const PopupMenuItem(value: 'auction', child: Text('Auction')),
            ],
          ),
        ],
      ),
      body: ref.watch(savedItemsProvider).when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (error, stackTrace) => Center(
          child: EmptyState.error(
            title: 'Data belum bisa dimuat.',
            subtitle: 'Periksa koneksi kamu lalu coba lagi.',
            onRetry: () => ref.invalidate(savedItemsProvider),
          ),
        ),
        data: (allItems) {
          final items = _selectedType == null
              ? allItems
              : allItems
                    .where(
                      (item) =>
                          (item.targetType == TargetType.forSale
                              ? 'for_sale'
                              : 'auction') ==
                          _selectedType,
                    )
                    .toList();
          if (items.isEmpty) {
            return const Center(
              child: EmptyState(
                icon: Icons.bookmarks_outlined,
                title: 'Belum ada item yang disimpan',
                subtitle:
                    'Item For Sale dan lelang yang kamu simpan akan muncul di sini.',
              ),
            );
          }
          return ListView.builder(
            itemCount: items.length,
            itemBuilder: (context, index) =>
                _buildSavedItemCard(items[index]),
          );
        },
      ),
    );
  }

  Widget _buildSavedItemCard(SavedItemModel item) {
    // Photo first, icon only as fallback: the model already carries
    // forSaleMediaUrls — the old icon-only leading hid it.
    final photoUrl = item.isForSale &&
            item.forSaleMediaUrls != null &&
            item.forSaleMediaUrls!.isNotEmpty
        ? item.forSaleMediaUrls!.first
        : null;
    return Card(
      margin: const EdgeInsets.symmetric(horizontal: AppMetrics.p16, vertical: AppMetrics.p8),
      child: ListTile(
        leading: photoUrl != null
            ? ClipRRect(
                borderRadius: BorderRadius.circular(AppShape.r6),
                child: AppImage(
                  imageUrl: photoUrl,
                  width: _thumbEdge,
                  height: _thumbEdge,
                  fit: BoxFit.cover,
                  errorWidget: _savedItemPlaceholder(context, item),
                ),
              )
            : _savedItemPlaceholder(context, item),
        title: Text(
          item.isForSale
              ? item.forSaleTitle ?? 'Untitled'
              : item.auctionTitle ?? 'Untitled',
        ),
        subtitle: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(
              item.isForSale
                  ? 'Rp ${formatGroupedAmount(item.forSalePrice ?? 0)}'
                  : 'Rp ${formatGroupedAmount(item.currentBid ?? item.startPrice ?? 0)}',
            ),
            const SizedBox(height: 4),
            Container(
              padding: const EdgeInsets.symmetric(
                horizontal: AppMetrics.p8,
                vertical: AppMetrics.p4,
              ),
              decoration: BoxDecoration(
                color: Theme.of(
                  context,
                ).colorScheme.primary.withValues(alpha: 0.1),
                borderRadius: BorderRadius.circular(AppShape.r4),
              ),
              child: Text(
                item.isForSale ? 'For Sale' : 'Lelang',
                style: Theme.of(context).textTheme.labelSmall?.copyWith(
                  color: Theme.of(context).colorScheme.primary,
                  fontWeight: FontWeight.w500,
                ),
              ),
            ),
          ],
        ),
        trailing: IconButton(
          icon: const Icon(Icons.bookmark_remove),
          onPressed: () => _removeItem(item),
        ),
        onTap: () {
          // Canonical navigation seam: NavigationHandler, never a raw
          // context.push in the UI layer.
          final navigationHandler = ref.read(navigationHandlerProvider);
          if (item.isForSale) {
            navigationHandler.navigateToForSaleDetail(item.targetId);
          } else {
            navigationHandler.navigateToAuction(item.targetId);
          }
        },
      ),
    );
  }

  /// Icon fallback for saved rows without a photo (all auctions today —
  /// the model carries no auction media — plus for-sales with empty media).
  Widget _savedItemPlaceholder(BuildContext context, SavedItemModel item) {
    return Container(
      width: _thumbEdge,
      height: _thumbEdge,
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.surfaceContainerHighest,
        borderRadius: BorderRadius.circular(AppShape.r6),
      ),
      child: Icon(
        item.isForSale ? Icons.list_alt : Icons.gavel,
        color: Theme.of(context).colorScheme.primary,
      ),
    );
  }
}
