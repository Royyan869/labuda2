/// Canonical Commerce Resource Picker for Comment flow.
///
/// Two tabs: For Sale (paginated + owned + active) and
/// Auction (paginated + promotable lifecycle).
/// Returns [CommerceResourceSelection] with typed [ResourceIdentity].
library;

import 'dart:async';

import 'package:flutter/material.dart';
import 'package:labuda/shared/widgets/app_image.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/shared/domain/entities/resource_projection.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/seller_auctions_pager.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/seller_fps_pager.dart';
import 'package:labuda/domains/social/comment/presentation/widgets/resource_identity.dart';
import 'package:labuda/shared/utils/media_extensions.dart';

class CommerceResourceSelection {
  final ResourceIdentity resource;
  final String title;
  final int? price;
  final String? imageUrl;
  const CommerceResourceSelection({
    required this.resource,
    required this.title,
    this.price,
    this.imageUrl,
  });
}

class CommerceResourcePicker extends ConsumerStatefulWidget {
  final String sellerId;
  final String? selectedResourceId;
  final Future<void> Function()? onCreateNewForSale;
  const CommerceResourcePicker({
    super.key,
    required this.sellerId,
    this.selectedResourceId,
    this.onCreateNewForSale,
  });

  @override
  ConsumerState<CommerceResourcePicker> createState() =>
      _CommerceResourcePickerState();

  static Future<CommerceResourceSelection?> show(
    BuildContext context, {
    required String sellerId,
    String? selectedResourceId,
    Future<void> Function()? onCreateNewForSale,
  }) {
    return showModalBottomSheet<CommerceResourceSelection>(
      context: context,
      isScrollControlled: true,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(16)),
      ),
      builder: (_) => CommerceResourcePicker(
        sellerId: sellerId,
        selectedResourceId: selectedResourceId,
        onCreateNewForSale: onCreateNewForSale,
      ),
    );
  }
}

class _CommerceResourcePickerState extends ConsumerState<CommerceResourcePicker>
    with SingleTickerProviderStateMixin {
  late TabController _tabController;

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 2, vsync: this);
  }

  @override
  void dispose() {
    _tabController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      height: MediaQuery.of(context).size.height * 0.7,
      padding: const EdgeInsets.only(top: 8),
      child: Column(
        children: [
          Container(
            width: 40,
            height: 4,
            margin: const EdgeInsets.only(bottom: 8),
            decoration: BoxDecoration(
              color: scheme.outlineVariant,
              borderRadius: BorderRadius.circular(2),
            ),
          ),
          Text(
            'Pilih Produk',
            style: TextStyle(fontSize: 16, fontWeight: FontWeight.w600),
          ),
          const SizedBox(height: 8),
          TabBar(
            controller: _tabController,
            labelColor: scheme.primary,
            unselectedLabelColor: scheme.onSurfaceVariant,
            tabs: const [
              Tab(text: 'Fixed Price'),
              Tab(text: 'Lelang'),
            ],
          ),
          Expanded(
            child: TabBarView(
              controller: _tabController,
              children: [
                _FPSTab(
                  sellerId: widget.sellerId,
                  selectedResourceId: widget.selectedResourceId,
                  onCreateNewForSale: widget.onCreateNewForSale,
                  onSelected: (s) => Navigator.of(context).pop(s),
                ),
                _AuctionTab(
                  sellerId: widget.sellerId,
                  selectedResourceId: widget.selectedResourceId,
                  onSelected: (s) => Navigator.of(context).pop(s),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

// ── FPS Tab (paginated) ───────────────────────────────────────────────

class _FPSTab extends ConsumerWidget {
  final String sellerId;
  final String? selectedResourceId;
  final Future<void> Function()? onCreateNewForSale;
  final Function(CommerceResourceSelection) onSelected;
  const _FPSTab({
    required this.sellerId,
    this.selectedResourceId,
    this.onCreateNewForSale,
    required this.onSelected,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final scheme = Theme.of(context).colorScheme;
    final createNewForSale = onCreateNewForSale;
    final pagerState = ref.watch(sellerFPSPagerProvider);
    final active = pagerState.items
        .where(
          (l) =>
              l.status == ForSaleStatus.active && l.forSaleId.isNotEmpty,
        )
        .toList();

    if (pagerState.isInitialLoading) {
      return const Center(child: CircularProgressIndicator());
    }

    if (pagerState.initialError != null && active.isEmpty) {
      return _EmptyTab(
        message: pagerState.initialError!,
        actionLabel: 'Coba Lagi',
        onAction: () =>
            ref.read(sellerFPSPagerProvider.notifier).retryInitial(),
      );
    }
    if (active.isEmpty && !pagerState.hasMore) {
      return _EmptyTab(
        message: 'Belum ada For Sale aktif',
        actionLabel: createNewForSale != null ? 'Buat Produk Baru' : null,
        onAction: createNewForSale == null
            ? null
            : () {
                unawaited(createNewForSale.call());
              },
      );
    }

    final showLoader = pagerState.isLoadingMore;
    final showCreateNewForSale = createNewForSale != null;
    final itemOffset = showCreateNewForSale ? 1 : 0;
    return ListView.builder(
      itemCount: active.length + itemOffset + (showLoader ? 1 : 0),
      itemBuilder: (context, index) {
        if (showCreateNewForSale && index == 0) {
          return ListTile(
            leading: Icon(
              Icons.add_circle_outline,
              color: scheme.primary,
            ),
            title: Text(
              'Buat Produk Baru',
              style: TextStyle(color: scheme.primary),
            ),
            onTap: () {
              unawaited(createNewForSale.call());
            },
          );
        }
        final forSaleIndex = index - itemOffset;
        if (forSaleIndex >= active.length) {
          return const Center(
            child: Padding(
              padding: EdgeInsets.all(16),
              child: CircularProgressIndicator(),
            ),
          );
        }
        final l = active[forSaleIndex];
        return _Tile(
          title: l.title,
          price: l.formattedPrice,
          imageUrl: l.media.isNotEmptyUrls ? l.media.firstUrl : null,
          isSelected: l.forSaleId == selectedResourceId,
          onTap: () => onSelected(
            CommerceResourceSelection(
              resource: ResourceIdentity(
                resourceType: ResourceType.forSale,
                resourceId: l.forSaleId,
              ),
              title: l.title,
              imageUrl: l.media.isNotEmptyUrls ? l.media.firstUrl : null,
            ),
          ),
        );
      },
    );
  }
}

// ── Auction Tab (paginated) ───────────────────────────────────────────

class _AuctionTab extends ConsumerStatefulWidget {
  final String sellerId;
  final String? selectedResourceId;
  final Function(CommerceResourceSelection) onSelected;
  const _AuctionTab({
    required this.sellerId,
    this.selectedResourceId,
    required this.onSelected,
  });

  @override
  ConsumerState<_AuctionTab> createState() => _AuctionTabState();
}

class _AuctionTabState extends ConsumerState<_AuctionTab> {
  @override
  void initState() {
    super.initState();
    Future.microtask(
      () => ref.read(sellerAuctionsPagerProvider.notifier).loadInitial(),
    );
  }

  @override
  Widget build(BuildContext context) {
    final pagerState = ref.watch(sellerAuctionsPagerProvider);
    final auctions = pagerState.visibleAuctions;
    final promotable = auctions
        .where(
          (a) =>
              a.status == AuctionStatus.scheduled ||
              a.status == AuctionStatus.active,
        )
        .toList();

    if (pagerState.isInitialLoading) {
      return const Center(child: CircularProgressIndicator());
    }

    if (pagerState.initialError != null && auctions.isEmpty) {
      return _EmptyTab(
        message: pagerState.initialError!,
        actionLabel: 'Coba Lagi',
        onAction: () =>
            ref.read(sellerAuctionsPagerProvider.notifier).retryInitial(),
      );
    }
    if (promotable.isEmpty) {
      return const _EmptyTab(
        message: 'Belum ada Lelang yang bisa dipromosikan',
      );
    }

    return ListView.builder(
      itemCount: promotable.length,
      itemBuilder: (context, index) {
        final a = promotable[index];
        final priceText = a.currentBid > 0
            ? 'Rp ${formatGroupedAmount(a.currentBid.round())}'
            : 'Rp ${formatGroupedAmount(a.openingBid.round())}';
        return _Tile(
          title: a.title,
          price: priceText,
          imageUrl: a.media.isNotEmptyUrls ? a.media.firstUrl : null,
          isSelected: a.id == widget.selectedResourceId,
          onTap: () => widget.onSelected(
            CommerceResourceSelection(
              resource: ResourceIdentity(
                resourceType: ResourceType.auction,
                resourceId: a.id,
              ),
              title: a.title,
              imageUrl: a.media.isNotEmptyUrls ? a.media.firstUrl : null,
            ),
          ),
        );
      },
    );
  }
}

class _EmptyTab extends StatelessWidget {
  final String message;
  final String? actionLabel;
  final VoidCallback? onAction;
  const _EmptyTab({required this.message, this.actionLabel, this.onAction});
  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(
            Icons.inventory_2_outlined,
            size: 48,
            color: scheme.outline,
          ),
          const SizedBox(height: 12),
          Text(message, style: TextStyle(color: scheme.onSurfaceVariant)),
          if (actionLabel != null && onAction != null) ...[
            const SizedBox(height: 12),
            ElevatedButton(onPressed: onAction, child: Text(actionLabel!)),
          ],
        ],
      ),
    );
  }
}

class _Tile extends StatelessWidget {
  final String title;
  final String? price;
  final String? imageUrl;
  final bool isSelected;
  final VoidCallback onTap;
  const _Tile({
    required this.title,
    this.price,
    this.imageUrl,
    this.isSelected = false,
    required this.onTap,
  });
  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return ListTile(
      selected: isSelected,
      selectedTileColor: scheme.primary.withValues(alpha: 0.05),
      leading: ClipRRect(
        borderRadius: BorderRadius.circular(6),
        child: imageUrl != null
            ? AppImage(
                imageUrl: imageUrl,
                width: 48,
                height: 48,
                fit: BoxFit.cover,
                errorWidget: _placeholder(context),
              )
            : _placeholder(context),
      ),
      title: Text(title, maxLines: 1, overflow: TextOverflow.ellipsis),
      subtitle: price != null
          ? Text(
              price!,
              style: TextStyle(
                color: scheme.primary,
                fontWeight: FontWeight.w600,
              ),
            )
          : null,
      trailing: isSelected
          ? Icon(Icons.check_circle, color: scheme.primary)
          : null,
      onTap: onTap,
    );
  }

  Widget _placeholder(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      width: 48,
      height: 48,
      color: scheme.surfaceContainerHighest,
      child: Icon(Icons.image, color: scheme.onSurfaceVariant),
    );
  }
}
