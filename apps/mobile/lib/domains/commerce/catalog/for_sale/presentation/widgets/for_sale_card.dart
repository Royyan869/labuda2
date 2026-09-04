import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/entities/for_sale.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/governance/seller_inactive_badge.dart';
import 'package:labuda/shared/utils/commerce_seller_identity.dart';

/// Canonical buyer-facing forSale card.
///
/// Used by every discovery / browsing surface (Explore, ForSaleList,
/// ProfileStore). Seller identity is redacted when [ForSale.sellerUserLifecycle]
/// is degraded, providing parity with SearchResultItem (E8.4).
///
/// NOT for seller management surfaces — use SellerForSaleManagementCard there.
class ForSaleCard extends StatelessWidget {
  final ForSale listing;
  final VoidCallback onTap;

  const ForSaleCard({super.key, required this.listing, required this.onTap});

  CommerceSellerIdentity? get _sellerIdentity => buildCommerceSellerIdentity(
    username: listing.sellerUsername,
    storeName: listing.sellerFarmName,
  );

  bool get _isSellerDegraded => listing.sellerUserLifecycle.isDegraded;

  String? get _sellerLine1 => _isSellerDegraded
      ? listing.sellerUserLifecycle.publicRedactionLabel
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
            // Image
            AspectRatio(
              aspectRatio: 16 / 9,
              child: Container(
                color: isDark
                    ? AppColors.darkGray700
                    : AppColors.neutralGray200,
                child: listing.media.isNotEmpty
                    ? Stack(
                        fit: StackFit.expand,
                        children: [
                          Image.network(
                            listing.media.first.originalUrl,
                            fit: BoxFit.cover,
                            errorBuilder: (context, error, stackTrace) {
                              return Icon(
                                Icons.image,
                                size: 48,
                                color: isDark
                                    ? AppColors.neutralGray600
                                    : AppColors.neutralGray400,
                              );
                            },
                          ),
                          // Video badge
                          if (listing.media.first.type == MediaType.video)
                            Positioned(
                              top: 8,
                              right: 8,
                              child: Container(
                                padding: const EdgeInsets.all(4),
                                decoration: BoxDecoration(
                                  color: Colors.black.withValues(alpha: 0.7),
                                  borderRadius: BorderRadius.circular(6),
                                ),
                                child: const Icon(
                                  Icons.play_circle_filled,
                                  color: Colors.white,
                                  size: 18,
                                ),
                              ),
                            ),
                        ],
                      )
                    : Icon(
                        Icons.image,
                        size: 48,
                        color: isDark
                            ? AppColors.neutralGray600
                            : AppColors.neutralGray400,
                      ),
              ),
            ),
            // Content
            Padding(
              padding: const EdgeInsets.all(12),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisSize: MainAxisSize.min,
                children: [
                  // Title
                  Text(
                    listing.title,
                    style: AppTypography.bodyLarge.copyWith(
                      fontWeight: FontWeight.w600,
                    ),
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                  ),
                  const SizedBox(height: 4),
                  // Description
                  Text(
                    listing.description,
                    style: AppTypography.bodySmall.copyWith(
                      color: isDark
                          ? AppColors.neutralGray400
                          : AppColors.neutralGray600,
                    ),
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
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
                  // Only when user-axis NOT already degraded (which fully
                  // redacts identity above).
                  if (shouldShowSellerInactiveBadge(
                    sellerTrustLifecycle: listing.sellerTrustLifecycle,
                    sellerUserLifecycle: listing.sellerUserLifecycle,
                  )) ...[
                    const SizedBox(height: 4),
                    const SellerInactiveBadge(),
                  ],
                  const SizedBox(height: 8),
                  // Price
                  Text(
                    listing.formattedPrice,
                    style: AppTypography.bodyMedium.copyWith(
                      fontWeight: FontWeight.w600,
                      color: AppColors.primaryRed,
                    ),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}
