/// Canonical Commerce Resource Picker for Comment flow.
///
/// Two tabs: For Sale (paginated + owned + active) and
/// Auction (paginated + promotable lifecycle).
/// Returns [CommerceResourceSelection] with typed [ResourceIdentity].
library;

import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
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
  final Future<void> Function()? onCreateNewListing;
  const CommerceResourcePicker({
    super.key,
    required this.sellerId,
    this.selectedResourceId,
    this.onCreateNewListing,
  });

  @override
  ConsumerState<CommerceResourcePicker> createState() =>
      _CommerceResourcePickerState();

  static Future<CommerceResourceSelection?> show(
    BuildContext context, {
    required String sellerId,
    String? selectedResourceId,
    Future<void> Function()? onCreateNewListing,
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
        onCreateNewListing: onCreateNewListing,
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
    final isDark = Theme.of(context).brightness == Brightness.dark;
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
              color: AppColors.neutralGray300,
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
            labelColor: AppColors.primaryRed,
            unselectedLabelColor: isDark
                ? AppColors.neutralGray400
                : AppColors.neutralGray600,
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
                  onCreateNewListing: widget.onCreateNewListing,
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
  final Future<void> Function()? onCreateNewListing;
  final Function(CommerceResourceSelection) onSelected;
  const _FPSTab({
    required this.sellerId,
    this.selectedResourceId,
    this.onCreateNewListing,
    required this.onSelected,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final createNewListing = onCreateNewListing;
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
        actionLabel: createNewListing != null ? 'Buat Produk Baru' : null,
        onAction: createNewListing == null
            ? null
            : () {
                unawaited(createNewListing.call());
              },
      );
    }

    final showLoader = pagerState.isLoadingMore;
    final showCreateNewListing = createNewListing != null;
    final itemOffset = showCreateNewListing ? 1 : 0;
    return ListView.builder(
      itemCount: active.length + itemOffset + (showLoader ? 1 : 0),
      itemBuilder: (context, index) {
        if (showCreateNewListing && index == 0) {
          return ListTile(
            leading: const Icon(
              Icons.add_circle_outline,
              color: AppColors.primaryRed,
            ),
            title: const Text(
              'Buat Produk Baru',
              style: TextStyle(color: AppColors.primaryRed),
            ),
            onTap: () {
              unawaited(createNewListing.call());
            },
          );
        }
        final listingIndex = index - itemOffset;
        if (listingIndex >= active.length) {
          return const Center(
            child: Padding(
              padding: EdgeInsets.all(16),
              child: CircularProgressIndicator(),
            ),
          );
        }
        final l = active[listingIndex];
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
            ? 'Rp ${a.currentBid}'
            : 'Rp ${a.openingBid}';
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
  Widget build(BuildContext context) => Center(
    child: Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(
          Icons.inventory_2_outlined,
          size: 48,
          color: AppColors.neutralGray400,
        ),
        const SizedBox(height: 12),
        Text(message, style: TextStyle(color: AppColors.neutralGray500)),
        if (actionLabel != null && onAction != null) ...[
          const SizedBox(height: 12),
          ElevatedButton(onPressed: onAction, child: Text(actionLabel!)),
        ],
      ],
    ),
  );
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
    return ListTile(
      selected: isSelected,
      selectedTileColor: AppColors.primaryRed.withValues(alpha: 0.05),
      leading: ClipRRect(
        borderRadius: BorderRadius.circular(6),
        child: imageUrl != null
            ? Image.network(
                imageUrl!,
                width: 48,
                height: 48,
                fit: BoxFit.cover,
                errorBuilder: (_, _, _) => _placeholder(),
              )
            : _placeholder(),
      ),
      title: Text(title, maxLines: 1, overflow: TextOverflow.ellipsis),
      subtitle: price != null
          ? Text(
              price!,
              style: TextStyle(
                color: AppColors.primaryRed,
                fontWeight: FontWeight.w600,
              ),
            )
          : null,
      trailing: isSelected
          ? const Icon(Icons.check_circle, color: AppColors.primaryRed)
          : null,
      onTap: onTap,
    );
  }

  Widget _placeholder() => Container(
    width: 48,
    height: 48,
    color: AppColors.neutralGray200,
    child: const Icon(Icons.image, color: AppColors.neutralGray400),
  );
}
