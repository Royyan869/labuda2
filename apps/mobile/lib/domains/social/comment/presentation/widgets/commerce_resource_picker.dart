/// Canonical Commerce Resource Picker for Comment flow.
///
/// Two tabs: For Sale (paginated + owned + active) and
/// Auction (paginated + promotable lifecycle).
/// Returns [CommerceResourceSelection] with typed [ResourceIdentity].
library;

import 'dart:async';

import 'package:flutter/material.dart';
import 'package:hishumi/shared/widgets/app_bottom_sheet_base.dart';
import 'package:hishumi/shared/widgets/app_image.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:hishumi/shared/domain/entities/resource_projection.dart';
import 'package:hishumi/domains/commerce/catalog/auction/domain/domain.dart';
import 'package:hishumi/domains/commerce/catalog/auction/presentation/providers/seller_auctions_pager.dart';
import 'package:hishumi/domains/commerce/catalog/for_sale/domain/domain.dart';
import 'package:hishumi/domains/commerce/catalog/for_sale/presentation/providers/seller_fps_pager.dart';
import 'package:hishumi/domains/social/comment/presentation/widgets/resource_identity.dart';
import 'package:hishumi/shared/utils/media_extensions.dart';
import 'package:hishumi/core/src/theme/app_theme.dart';

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
    // Presentation authority is the canonical base; this file owns NO
    // bottom-sheet renderer, no surface, no shape, no handle.
    return AppBottomSheetBase.show<CommerceResourceSelection>(
      context: context,
      title: 'Pilih Produk',
      padding: EdgeInsets.zero,
      content: CommerceResourcePicker(
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
    // Surface, shape, handle, scroll and safe area come from the base.
    // Body height — a share of the space the sheet ACTUALLY has, asked of
    // the sheet authority itself ([AppBottomSheetBase.contentAllocationOf]:
    // the live content region, with the sheet chrome and the system spacer
    // already spent — a share of the raw CEILING shared the budget with the
    // sheet chrome and could outgrow the region at larger insets,
    // BOTTOMSHEET-04, geometry-proven). The finite box stays here: the base
    // scrolls its content, so the `Expanded > TabBarView` below needs this
    // bounded slot to lay out.
    return SizedBox(
      height: AppBottomSheetBase.contentAllocationOf(context) * 0.7,
      child: Column(
        children: [
          const SizedBox(height: 8),
          TabBar(
            controller: _tabController,
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
          (l) => l.status == ForSaleStatus.active && l.forSaleId.isNotEmpty,
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
            leading: Icon(Icons.add_circle_outline, color: scheme.primary),
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
              padding: EdgeInsets.all(AppMetrics.p16),
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
              price: l.price.toInt(),
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
              price: (a.currentBid > 0 ? a.currentBid : a.openingBid).round(),
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
            size: AppIconSize.display,
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
        borderRadius: BorderRadius.circular(AppShape.r6),
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
