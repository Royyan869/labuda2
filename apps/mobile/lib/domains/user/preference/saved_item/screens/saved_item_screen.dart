import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/shared/domain/entities/resource_projection.dart';
import 'package:labuda/domains/user/preference/saved_item/models/saved_item_model.dart';
import 'package:labuda/domains/user/preference/saved_item/data/providers/saved_item_query_providers.dart';
import 'package:labuda/domains/user/preference/saved_item/data/repositories/saved_item_repository_provider.dart';
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
        error: (error, stackTrace) =>
            const Center(child: Text('Belum ada item yang disimpan')),
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
            return const Center(child: Text('Belum ada item yang disimpan'));
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
    return Card(
      margin: const EdgeInsets.symmetric(horizontal: AppMetrics.p16, vertical: AppMetrics.p8),
      child: ListTile(
        leading: Icon(
          item.isForSale ? Icons.list_alt : Icons.gavel,
          color: Theme.of(context).colorScheme.primary,
        ),
        title: Text(
          item.isForSale
              ? item.forSaleTitle ?? 'Untitled'
              : item.auctionTitle ?? 'Untitled',
        ),
        subtitle: Text(
          item.isForSale
              ? 'Rp ${formatGroupedAmount(item.forSalePrice ?? 0)}'
              : 'Rp ${formatGroupedAmount(item.currentBid ?? item.startPrice ?? 0)}',
        ),
        trailing: IconButton(
          icon: const Icon(Icons.bookmark_remove),
          onPressed: () => _removeItem(item),
        ),
        onTap: () {
          // Navigate to detail
        },
      ),
    );
  }
}
