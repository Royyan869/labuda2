/// Auction Detail Info
///
/// Shows auction detailed information
library;

import 'package:flutter/material.dart';
import 'package:labuda/shared/domain/entities/resource_projection.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:labuda/domains/commerce/catalog/shared/presentation/widgets/commerce_common_product_detail_section.dart';
import 'package:labuda/domains/commerce/catalog/shared/presentation/widgets/commerce_detail_primitives.dart';
import 'package:labuda/core/src/theme/app_theme.dart';

/// Detail info widget for auction — TWO cards:
///   1. auction-specific card (channel explanation + bid increment)
///   2. the shared Product content card, consumed through
///      [CommerceCommonProductDetailSection] — the SAME section the ForSale
///      sibling uses, so no canonical value preserved in the Auction read
///      model is left dead.
///
/// Both use the canonical 16-margin [CommerceDetailSectionCard] frame.
class AuctionDetailInfo extends StatelessWidget {
  final Auction auction;

  const AuctionDetailInfo({super.key, required this.auction});

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        // 1) Channel-specific card.
        CommerceDetailSectionCard(
          margin: const EdgeInsets.fromLTRB(
            AppMetrics.p16,
            AppMetrics.p0,
            AppMetrics.p16,
            AppMetrics.p16,
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                'Detail Lelang',
                style: Theme.of(
                  context,
                ).textTheme.titleMedium?.copyWith(fontWeight: FontWeight.w700),
              ),
              const SizedBox(height: 8),
              // AUCTION EXPLANATION - Minimal 1-line explanation
              Container(
                padding: const EdgeInsets.symmetric(
                  horizontal: AppMetrics.p12,
                  vertical: AppMetrics.p8,
                ),
                decoration: BoxDecoration(
                  color: colorScheme.surfaceContainerHighest,
                  borderRadius: BorderRadius.circular(AppShape.r8),
                ),
                child: Row(
                  children: [
                    Icon(
                      Icons.info_outline,
                      size: AppIconSize.inlineGlyph,
                      color: colorScheme.onSurfaceVariant,
                    ),
                    const SizedBox(width: 8),
                    Expanded(
                      child: Text(
                        'Lelang — harga naik, penawar tertinggi menang',
                        style: context.typeRoles.bodyDense.copyWith(
                          color: colorScheme.onSurfaceVariant,
                          fontStyle: FontStyle.italic,
                        ),
                      ),
                    ),
                  ],
                ),
              ),
              const SizedBox(height: 12),
              _buildInfoRow(
                context,
                'Bid Increment',
                'Rp ${formatGroupedAmount(auction.bidIncrement.round())}',
              ),
            ],
          ),
        ),
        // 2) Shared Product content card (identical to the ForSale surface).
        //    Renders only the rows whose canonical value is present; when the
        //    payload carries none of them it collapses to nothing.
        Padding(
          padding: const EdgeInsets.fromLTRB(
            AppMetrics.p16,
            AppMetrics.p0,
            AppMetrics.p16,
            AppMetrics.p16,
          ),
          child: CommerceCommonProductDetailSection(
            title: 'Detail Produk',
            data: CommerceCommonProductDetailsData.fromAuction(auction),
          ),
        ),
      ],
    );
  }

  Widget _buildInfoRow(BuildContext context, String label, String value) {
    final colorScheme = Theme.of(context).colorScheme;
    return Padding(
      padding: const EdgeInsets.only(bottom: AppMetrics.p8),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Text(label, style: TextStyle(color: colorScheme.onSurfaceVariant)),
          Text(value, style: const TextStyle(fontWeight: FontWeight.w500)),
        ],
      ),
    );
  }
}
