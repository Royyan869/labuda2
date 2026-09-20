import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/auction/auction.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/for_sale_providers.dart';

/// Home Commerce Preview - "Sedang Laku Hari Ini"
///
/// Shows 3-5 trending items (forSales and auctions) above the feed
/// to demonstrate active marketplace activity.
///
/// GOAL: User sees "ini marketplace aktif" immediately
class CommercePreviewSection extends ConsumerWidget {
  const CommercePreviewSection({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Container(
      color: isDark ? AppColors.darkGray800 : AppColors.neutralGray50,
      padding: const EdgeInsets.symmetric(vertical: 16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Section Header
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 16),
            child: Row(
              children: [
                const Text(
                  '🔥 Sedang Laku Hari Ini',
                  style: TextStyle(fontSize: 16, fontWeight: FontWeight.w700),
                ),
                const SizedBox(width: 8),
                Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: 8,
                    vertical: 2,
                  ),
                  decoration: BoxDecoration(
                    color: AppColors.primaryRed.withValues(alpha: 0.1),
                    borderRadius: BorderRadius.circular(10),
                  ),
                  child: Text(
                    'Live',
                    style: TextStyle(
                      fontSize: 11,
                      fontWeight: FontWeight.w600,
                      color: AppColors.primaryRed,
                    ),
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: 12),

          // Horizontal scrollable items
          SizedBox(
            height: 140,
            child: _buildCommerceItems(context, ref, isDark),
          ),
        ],
      ),
    );
  }

  Widget _buildCommerceItems(BuildContext context, WidgetRef ref, bool isDark) {
    // Fetch active forSales and auctions
    final forSalesAsync = ref.watch(
      forSalesProvider(
        const ForSalesParams(status: ForSaleStatus.active, limit: 3),
      ),
    );

    final auctionsAsync = ref.watch(exploreAuctionsStreamProvider);

    return forSalesAsync.when(
      data: (forSales) {
        return auctionsAsync.when(
          data: (auctions) {
            // Combine and limit to 5 items total
            final items = _buildMixedItems(forSales, auctions);

            if (items.isEmpty) {
              return const SizedBox.shrink();
            }

            return ListView.separated(
              scrollDirection: Axis.horizontal,
              padding: const EdgeInsets.symmetric(horizontal: 16),
              itemCount: items.length,
              separatorBuilder: (_, _) => const SizedBox(width: 12),
              itemBuilder: (context, index) {
                final item = items[index];
                return _CommercePreviewCard(
                  item: item,
                  onTap: () => _navigateToDetail(context, item),
                );
              },
            );
          },
          loading: () =>
              const Center(child: CircularProgressIndicator(strokeWidth: 2)),
          error: (_, _) => const SizedBox.shrink(),
        );
      },
      loading: () =>
          const Center(child: CircularProgressIndicator(strokeWidth: 2)),
      error: (_, _) => const SizedBox.shrink(),
    );
  }

  /// Mix forSales and auctions, max 5 items
  List<_CommercePreviewItem> _buildMixedItems(
    List<ForSale> forSales,
    List<Auction> auctions,
  ) {
    final items = <_CommercePreviewItem>[];

    // Add up to 3 forSales
    for (final forSale in forSales.take(3)) {
      items.add(_CommercePreviewItem.forSale(forSale));
    }

    // Add up to 2 auctions
    for (final auction in auctions.take(2)) {
      items.add(_CommercePreviewItem.auction(auction));
    }

    // Shuffle slightly for variety, but keep forSale first
    if (items.length > 2) {
      final first = items.removeAt(0);
      items.shuffle();
      items.insert(0, first);
    }

    return items.take(5).toList();
  }

  void _navigateToDetail(BuildContext context, _CommercePreviewItem item) {
    switch (item.type) {
      case _CommercePreviewType.forSale:
        context.push(
          RoutePaths.forSaleDetail.replaceFirst(
            ':forSaleId',
            item.forSale!.forSaleId,
          ),
        );
        break;
      case _CommercePreviewType.auction:
        context.push('${RoutePaths.auctionDetails}/${item.auction!.id}');
        break;
    }
  }
}

/// Preview item wrapper
enum _CommercePreviewType { forSale, auction }

class _CommercePreviewItem {
  final _CommercePreviewType type;
  final ForSale? forSale;
  final Auction? auction;

  _CommercePreviewItem.forSale(this.forSale)
    : type = _CommercePreviewType.forSale,
      auction = null;

  _CommercePreviewItem.auction(this.auction)
    : type = _CommercePreviewType.auction,
      forSale = null;
}

/// Compact card for commerce preview
class _CommercePreviewCard extends StatelessWidget {
  final _CommercePreviewItem item;
  final VoidCallback onTap;

  const _CommercePreviewCard({required this.item, required this.onTap});

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return GestureDetector(
      onTap: onTap,
      child: Container(
        width: 160,
        decoration: BoxDecoration(
          color: isDark ? AppColors.darkGray700 : AppColors.neutralWhite,
          borderRadius: BorderRadius.circular(12),
          border: Border.all(
            color: isDark ? AppColors.darkGray600 : AppColors.neutralGray200,
          ),
        ),
        clipBehavior: Clip.antiAlias,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // Image
            Expanded(child: _buildImage(context, isDark)),

            // Info
            Padding(
              padding: const EdgeInsets.all(8),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  // Type badge
                  _buildTypeBadge(isDark),
                  const SizedBox(height: 4),
                  // Price
                  _buildPrice(context, isDark),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildImage(BuildContext context, bool isDark) {
    String? imageUrl;
    String placeholderText;

    if (item.type == _CommercePreviewType.forSale) {
      imageUrl = item.forSale?.media.isNotEmpty == true
          ? item.forSale!.media.first.originalUrl
          : null;
      placeholderText =
          item.forSale?.title.substring(0, 1).toUpperCase() ?? 'K';
    } else {
      imageUrl = item.auction?.media.isNotEmpty == true
          ? item.auction!.media.first.originalUrl
          : null;
      placeholderText =
          item.auction?.title.substring(0, 1).toUpperCase() ?? 'L';
    }

    if (imageUrl != null) {
      return Image.network(
        imageUrl,
        width: double.infinity,
        fit: BoxFit.cover,
        errorBuilder: (context, error, stackTrace) =>
            _buildPlaceholder(isDark, placeholderText),
      );
    }

    return _buildPlaceholder(isDark, placeholderText);
  }

  Widget _buildPlaceholder(bool isDark, String text) {
    return Container(
      width: double.infinity,
      color: isDark ? AppColors.darkGray600 : AppColors.neutralGray200,
      child: Center(
        child: Text(
          text,
          style: TextStyle(
            fontSize: 32,
            fontWeight: FontWeight.w700,
            color: isDark ? AppColors.neutralGray500 : AppColors.neutralGray400,
          ),
        ),
      ),
    );
  }

  Widget _buildTypeBadge(bool isDark) {
    String label;
    Color color;

    switch (item.type) {
      case _CommercePreviewType.forSale:
        label = 'Beli Sekarang';
        color = AppColors.primaryGreen;
        break;
      case _CommercePreviewType.auction:
        label = 'Lelang';
        color = AppColors.statusWarning;
        break;
    }

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(4),
      ),
      child: Text(
        label,
        style: TextStyle(
          fontSize: 10,
          fontWeight: FontWeight.w600,
          color: color,
        ),
      ),
    );
  }

  Widget _buildPrice(BuildContext context, bool isDark) {
    String priceText;

    if (item.type == _CommercePreviewType.forSale) {
      priceText = item.forSale?.formattedPrice ?? '-';
    } else {
      priceText = item.auction?.currentBid.toStringAsFixed(0) ?? '-';
    }

    return Text(
      priceText,
      style: TextStyle(
        fontSize: 14,
        fontWeight: FontWeight.w700,
        color: AppColors.primaryRed,
      ),
      maxLines: 1,
      overflow: TextOverflow.ellipsis,
    );
  }
}
