/// Negotiation Accepted Action Widget
///
/// **STRICT MODE - READ ONLY DISPLAY**
///
/// Displays "Buy Now" button when negotiation is accepted.
/// Navigates to OrderPreviewScreen with pricing from backend.
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/for_sale_providers.dart';
import 'package:labuda/domains/commerce/negotiation/negotiation/domain/entities/negotiation.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/governance/seller_inactive_badge.dart';
import 'package:labuda/shared/shared.dart';

/// Negotiation Accepted Action Widget
///
/// Shows when a negotiation is accepted by the seller.
/// Displays the agreed price and a "Buy Now" button that navigates to order preview.
///
/// **PRICING TRANSPARENCY:**
/// - Shows the agreed price from backend negotiation
/// - "Buy Now" button navigates to OrderPreviewScreen
/// - Order preview shows complete pricing breakdown from backend
/// - No frontend calculations - all pricing from backend
class NegotiationAcceptedAction extends ConsumerWidget {
  final Negotiation negotiation;
  final String? chatId;

  const NegotiationAcceptedAction({
    super.key,
    required this.negotiation,
    this.chatId,
  });

  void _handleBuyNow(BuildContext context, String? productId) {
    if (productId == null || productId.isEmpty) {
      AppSnackBar.showError(
        context,
        'ID produk belum tersedia untuk checkout ini',
      );
      return;
    }

    // Navigate to checkout screen with negotiation context
    final uri = Uri(
      path: '/checkout/${negotiation.fixedPriceSaleId}',
      queryParameters: {
        'product_id': productId,
        if (negotiation.id.isNotEmpty) 'negotiation_id': negotiation.id,
        if (chatId != null && chatId!.isNotEmpty) 'return_to_chat': chatId,
      },
    );
    context.push(uri.toString());
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);

    // Expired-seller visibility — disable "Beli Sekarang" up-front when
    // the listing's seller has lapsed subscription. Backend order Guard 6
    // rejects regardless, but the UI short-circuits so the buyer is not
    // routed to /order-preview only to be bounced. Tolerant fallback:
    // when the listing detail provider is still loading or errored,
    // leave the CTA enabled (active default), since backend remains the
    // final authority.
    final listingAsync = ref.watch(
      forSaleDetailProvider(negotiation.fixedPriceSaleId),
    );
    final productId = listingAsync.maybeWhen(
      data: (listing) => listing?.productId,
      orElse: () => null,
    );
    final sellerInactive = listingAsync.maybeWhen(
      data: (listing) {
        if (listing == null) return false;
        return listing.sellerTrustLifecycle != ContentLifecycle.active;
      },
      orElse: () => false,
    );

    return Container(
      margin: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        gradient: LinearGradient(
          colors: [
            theme.colorScheme.primaryContainer.withValues(alpha: 0.3),
            theme.colorScheme.primaryContainer.withValues(alpha: 0.1),
          ],
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
        ),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: theme.colorScheme.primary.withValues(alpha: 0.3),
          width: 1.5,
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Header with icon
          Row(
            children: [
              Container(
                padding: const EdgeInsets.all(8),
                decoration: BoxDecoration(
                  color: theme.colorScheme.primary.withValues(alpha: 0.2),
                  borderRadius: BorderRadius.circular(8),
                ),
                child: Icon(
                  Icons.check_circle_outline,
                  color: theme.colorScheme.primary,
                  size: 20,
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      'Harga Disetujui!',
                      style: theme.textTheme.titleSmall?.copyWith(
                        fontWeight: FontWeight.w700,
                        color: theme.colorScheme.primary,
                      ),
                    ),
                    const SizedBox(height: 2),
                    Text(
                      'Penjual menyetujui harga tawaran Anda',
                      style: theme.textTheme.bodySmall?.copyWith(
                        color: theme.colorScheme.onSurfaceVariant,
                      ),
                    ),
                  ],
                ),
              ),
            ],
          ),

          const Divider(height: 24),

          // Price breakdown
          _buildPriceBreakdown(theme),

          const SizedBox(height: 16),

          // Trust badges
          _buildTrustBadges(theme),

          const SizedBox(height: 16),

          // Expired-seller visibility — badge above CTA when seller is
          // inactive so the buyer sees why "Beli Sekarang" is disabled.
          if (sellerInactive) ...[
            const SellerInactiveBadge(
              label: 'Penjual tidak aktif — pembelian tidak tersedia',
            ),
            const SizedBox(height: 12),
          ],
          if (productId == null || productId.isEmpty) ...[
            Text(
              'ID produk belum tersedia untuk checkout ini',
              style: theme.textTheme.bodySmall?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
            ),
            const SizedBox(height: 12),
          ],
          // Buy Now button
          SizedBox(
            width: double.infinity,
            child: ElevatedButton.icon(
              onPressed:
                  (sellerInactive || productId == null || productId.isEmpty)
                  ? null
                  : () => _handleBuyNow(context, productId),
              icon: const Icon(Icons.shopping_cart_outlined),
              label: const Text('Beli Sekarang'),
              style: ElevatedButton.styleFrom(
                padding: const EdgeInsets.symmetric(vertical: 14),
                backgroundColor: theme.colorScheme.primary,
                foregroundColor: theme.colorScheme.onPrimary,
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(8),
                ),
              ),
            ),
          ),

          const SizedBox(height: 8),

          // Urgency text
          Center(
            child: Text(
              'Segera checkout untuk mengunci barang',
              style: theme.textTheme.labelSmall?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
                fontStyle: FontStyle.italic,
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildPriceBreakdown(ThemeData theme) {
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: theme.colorScheme.surface.withValues(alpha: 0.8),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Column(
        children: [
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text(
                'Harga Awal',
                style: theme.textTheme.bodyMedium?.copyWith(
                  color: theme.colorScheme.onSurfaceVariant,
                ),
              ),
              Text(
                CurrencyUtils.format(negotiation.originalPrice),
                style: theme.textTheme.bodyMedium?.copyWith(
                  decoration: TextDecoration.lineThrough,
                  color: theme.colorScheme.onSurfaceVariant,
                ),
              ),
            ],
          ),
          const SizedBox(height: 8),
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text(
                'Harga Disepakati',
                style: theme.textTheme.bodyMedium?.copyWith(
                  fontWeight: FontWeight.w600,
                ),
              ),
              Text(
                CurrencyUtils.format(
                  negotiation.agreedPrice ?? negotiation.currentOfferPrice,
                ),
                style: theme.textTheme.titleMedium?.copyWith(
                  fontWeight: FontWeight.w700,
                  color: theme.colorScheme.primary,
                  fontSize: 16,
                ),
              ),
            ],
          ),
          if (negotiation.originalPrice >
              (negotiation.agreedPrice ?? negotiation.currentOfferPrice))
            Padding(
              padding: const EdgeInsets.only(top: 8),
              child: Row(
                children: [
                  Icon(
                    Icons.local_offer_outlined,
                    size: 14,
                    color: theme.colorScheme.secondary,
                  ),
                  const SizedBox(width: 4),
                  Text(
                    'Hemat ${CurrencyUtils.format(negotiation.originalPrice - (negotiation.agreedPrice ?? negotiation.currentOfferPrice))}',
                    style: theme.textTheme.labelSmall?.copyWith(
                      color: theme.colorScheme.secondary,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                ],
              ),
            ),
        ],
      ),
    );
  }

  Widget _buildTrustBadges(ThemeData theme) {
    return Wrap(
      spacing: 8,
      runSpacing: 8,
      children: [
        _buildBadge(
          theme,
          icon: Icons.lock_outline,
          label: 'Harga Terkunci',
          color: theme.colorScheme.primary,
        ),
        _buildBadge(
          theme,
          icon: Icons.verified_outlined,
          label: 'Valid 10 Menit',
          color: theme.colorScheme.tertiary,
        ),
      ],
    );
  }

  Widget _buildBadge(
    ThemeData theme, {
    required IconData icon,
    required String label,
    required Color color,
  }) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: color.withValues(alpha: 0.3)),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 12, color: color),
          const SizedBox(width: 4),
          Text(
            label,
            style: theme.textTheme.labelSmall?.copyWith(
              color: color,
              fontWeight: FontWeight.w600,
            ),
          ),
        ],
      ),
    );
  }
}
