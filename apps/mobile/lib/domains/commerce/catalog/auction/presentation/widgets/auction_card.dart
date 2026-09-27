import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/domain/entities/resource_projection.dart';
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
    final scheme = Theme.of(context).colorScheme;

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
            _buildImageWithUrgency(scheme),

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
                      Icon(Icons.gavel, size: 16, color: scheme.primary),
                      const SizedBox(width: 4),
                      Text(
                        auction.currentBid > 0
                            ? 'Rp ${formatGroupedAmount(auction.currentBid.round())}'
                            : 'Mulai Rp ${formatGroupedAmount(auction.startingBid.round())}',
                        style: AppTypography.bodyMedium.copyWith(
                          color: scheme.primary,
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
                            color: scheme.onSurfaceVariant,
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
                              color: scheme.onSurfaceVariant,
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

  Widget _buildImageWithUrgency(ColorScheme scheme) {
    final hasMedia = auction.media.isNotEmpty;

    return Stack(
      children: [
        AspectRatio(
          aspectRatio: 16 / 9,
          child: Container(
            color: scheme.surfaceContainerHighest,
            child: hasMedia
                ? Image.network(
                    auction.media.first.originalUrl,
                    fit: BoxFit.cover,
                    errorBuilder: (context, error, stackTrace) =>
                        _buildPlaceholder(scheme),
                  )
                : _buildPlaceholder(scheme),
          ),
        ),

        // Urgency badge for active auctions
        if (auction.isActive)
          Positioned(
            top: 8,
            right: 8,
            child: _buildUrgencyBadge(scheme),
          ),

        // Status badge for non-active auctions
        if (!auction.isActive)
          Positioned(top: 8, right: 8, child: _buildStatusBadge(scheme)),
      ],
    );
  }

  Widget _buildPlaceholder(ColorScheme scheme) {
    return Container(
      color: scheme.surfaceContainerHighest,
      child: Center(
        child: Icon(
          Icons.image_outlined,
          size: 48,
          color: scheme.onSurfaceVariant,
        ),
      ),
    );
  }

  Widget _buildUrgencyBadge(ColorScheme scheme) {
    final timeRemaining = auction.getTimeRemaining();
    final bgColor = _getUrgencyColor(scheme, timeRemaining.urgencyLevel);
    // Solid semantic badges pair with onPrimary; the neutral ended badge
    // (light gray in dark mode) pairs with surface ink instead.
    final fgColor = timeRemaining.urgencyLevel == AuctionUrgencyLevel.ended
        ? scheme.surface
        : scheme.onPrimary;

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
      decoration: BoxDecoration(
        color: bgColor,
        borderRadius: BorderRadius.circular(12),
      ),
      child: Text(
        timeRemaining.displayText,
        style: TextStyle(
          fontSize: 11,
          fontWeight: FontWeight.w600,
          color: fgColor,
        ),
      ),
    );
  }

  Color _getUrgencyColor(ColorScheme scheme, AuctionUrgencyLevel level) {
    switch (level) {
      case AuctionUrgencyLevel.critical:
        return scheme.error.withValues(alpha: 0.9);
      case AuctionUrgencyLevel.warning:
        return AppColors.statusWarning.withValues(alpha: 0.9);
      case AuctionUrgencyLevel.normal:
        return AppColors.statusSuccess.withValues(alpha: 0.9);
      case AuctionUrgencyLevel.ended:
        return scheme.onSurfaceVariant;
    }
  }

  Widget _buildStatusBadge(ColorScheme scheme) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
      decoration: BoxDecoration(
        color: scheme.surfaceContainerHighest,
        borderRadius: BorderRadius.circular(12),
      ),
      child: Text(
        auction.status.displayName,
        style: TextStyle(
          fontSize: 11,
          fontWeight: FontWeight.w600,
          color: scheme.onSurfaceVariant,
        ),
      ),
    );
  }
}
