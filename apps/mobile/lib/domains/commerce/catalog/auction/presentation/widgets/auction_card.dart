import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction_status.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction_time_extension.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/governance/seller_inactive_badge.dart';
import 'package:labuda/shared/utils/commerce_seller_identity.dart';

/// Canonical buyer-facing auction card.
///
/// Used by every discovery / browsing surface (Marketplace, ProfileStore).
/// Seller identity is redacted when [Auction.sellerUserLifecycle] is degraded,
/// providing parity with SearchResultItem (E8.4).
///
/// NOT for seller management surfaces.
class AuctionCard extends StatelessWidget {
  final Auction auction;
  final VoidCallback onTap;

  const AuctionCard({super.key, required this.auction, required this.onTap});

  CommerceSellerIdentity? get _sellerIdentity => buildCommerceSellerIdentity(
    username: auction.sellerUsername,
    storeName: auction.sellerFarmName,
  );

  bool get _isSellerDegraded => auction.sellerUserLifecycle.isDegraded;

  String? get _sellerLine1 => _isSellerDegraded
      ? auction.sellerUserLifecycle.publicRedactionLabel
      : _sellerIdentity?.line1;

  String? get _sellerLine2 => _isSellerDegraded ? null : _sellerIdentity?.line2;

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Card(
      margin: const EdgeInsets.only(bottom: 16),
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: onTap,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisSize: MainAxisSize.min,
          children: [
            // Image with urgency overlay
            _buildImageWithUrgency(isDark),

            // Content
            Padding(
              padding: const EdgeInsets.all(12),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisSize: MainAxisSize.min,
                children: [
                  // Title
                  Text(
                    auction.title,
                    style: AppTypography.bodyLarge.copyWith(
                      fontWeight: FontWeight.w600,
                    ),
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                  ),
                  const SizedBox(height: 8),

                  // Price row
                  //
                  // PURGED: "X bid" badge — the auction wire never emitted
                  // total_bids, so the counter was always-zero fake truth.
                  // Owner decision (2026-09-25): remove until the backend
                  // emits a canonical bid-count field.
                  Row(
                    children: [
                      Icon(Icons.gavel, size: 16, color: AppColors.primaryRed),
                      const SizedBox(width: 4),
                      Text(
                        auction.currentBid > 0
                            ? 'Rp ${auction.currentBid.toStringAsFixed(0)}'
                            : 'Mulai Rp ${auction.startingBid.toStringAsFixed(0)}',
                        style: AppTypography.bodyMedium.copyWith(
                          color: AppColors.primaryRed,
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                    ],
                  ),

                  // Seller identity with E8.2 lifecycle redaction
                  if (_sellerLine1 != null) ...[
                    const SizedBox(height: 4),
                    Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          _sellerLine1!,
                          style: AppTypography.bodySmall.copyWith(
                            color: isDark
                                ? AppColors.neutralGray500
                                : AppColors.neutralGray400,
                            fontStyle: _isSellerDegraded
                                ? FontStyle.italic
                                : FontStyle.normal,
                          ),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                        ),
                        if (_sellerLine2 != null) ...[
                          const SizedBox(height: 2),
                          Text(
                            _sellerLine2!,
                            style: AppTypography.bodySmall.copyWith(
                              color: isDark
                                  ? AppColors.neutralGray500
                                  : AppColors.neutralGray400,
                            ),
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                          ),
                        ],
                      ],
                    ),
                  ],

                  // Expired-seller visibility — seller-trust axis badge.
                  if (shouldShowSellerInactiveBadge(
                    sellerTrustLifecycle: auction.sellerTrustLifecycle,
                    sellerUserLifecycle: auction.sellerUserLifecycle,
                  )) ...[
                    const SizedBox(height: 4),
                    const SellerInactiveBadge(),
                  ],

                  const SizedBox(height: 4),
                  // PURGED: location row — AuctionLocation was never hydrated
                  // from any wire payload; the slot rendered nothing.
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildImageWithUrgency(bool isDark) {
    final hasMedia = auction.media.isNotEmpty;

    return Stack(
      children: [
        AspectRatio(
          aspectRatio: 16 / 9,
          child: Container(
            color: isDark ? AppColors.darkGray700 : AppColors.neutralGray200,
            child: hasMedia
                ? Image.network(
                    auction.media.first.originalUrl,
                    fit: BoxFit.cover,
                    errorBuilder: (context, error, stackTrace) =>
                        _buildPlaceholder(isDark),
                  )
                : _buildPlaceholder(isDark),
          ),
        ),

        // Urgency badge for active auctions
        if (auction.isActive)
          Positioned(top: 8, right: 8, child: _buildUrgencyBadge()),

        // Status badge for non-active auctions
        if (!auction.isActive)
          Positioned(top: 8, right: 8, child: _buildStatusBadge(isDark)),
      ],
    );
  }

  Widget _buildPlaceholder(bool isDark) {
    return Container(
      color: isDark ? AppColors.darkGray700 : AppColors.neutralGray200,
      child: const Center(
        child: Icon(
          Icons.image_outlined,
          size: 48,
          color: AppColors.neutralGray400,
        ),
      ),
    );
  }

  Widget _buildUrgencyBadge() {
    final timeRemaining = auction.getTimeRemaining();
    final bgColor = _getUrgencyColor(timeRemaining.urgencyLevel);

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
      decoration: BoxDecoration(
        color: bgColor,
        borderRadius: BorderRadius.circular(12),
      ),
      child: Text(
        timeRemaining.displayText,
        style: const TextStyle(
          fontSize: 11,
          fontWeight: FontWeight.w600,
          color: Colors.white,
        ),
      ),
    );
  }

  Color _getUrgencyColor(AuctionUrgencyLevel level) {
    switch (level) {
      case AuctionUrgencyLevel.critical:
        return Colors.red.withValues(alpha: 0.9);
      case AuctionUrgencyLevel.warning:
        return Colors.orange.withValues(alpha: 0.9);
      case AuctionUrgencyLevel.normal:
        return AppColors.primaryGreen.withValues(alpha: 0.9);
      case AuctionUrgencyLevel.ended:
        return AppColors.neutralGray600;
    }
  }

  Widget _buildStatusBadge(bool isDark) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray600 : AppColors.neutralGray300,
        borderRadius: BorderRadius.circular(12),
      ),
      child: Text(
        auction.status.displayName,
        style: TextStyle(
          fontSize: 11,
          fontWeight: FontWeight.w600,
          color: isDark ? AppColors.neutralGray300 : AppColors.neutralGray700,
        ),
      ),
    );
  }
}
