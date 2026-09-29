/// ForSale Detail Screen
///
/// Product detail page for individual forSales.
/// This is the main product detail page for buyers.
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/entities/for_sale.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/for_sale_providers.dart';
import 'package:labuda/domains/chat/chat/presentation/utils/commerce_chat_navigation.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/domains/social/share/share.dart';
import 'package:labuda/domains/system/report/domain/entities/entities.dart';
import 'package:labuda/domains/system/report/presentation/dialogs/report_submission_dialog.dart';
import 'package:labuda/domains/commerce/catalog/shared/presentation/widgets/commerce_common_product_detail_section.dart';
import 'package:labuda/domains/commerce/catalog/shared/presentation/widgets/commerce_detail_primitives.dart';
import 'package:labuda/domains/commerce/catalog/shared/presentation/widgets/commerce_detail_seller_card.dart';
import 'package:labuda/domains/commerce/catalog/shared/presentation/widgets/commerce_detail_states.dart';
import 'package:labuda/domains/commerce/catalog/shared/presentation/widgets/commerce_saved_item_action_button.dart';
import 'package:labuda/shared/utils/media_extensions.dart';

/// ForSale Detail Screen
///
/// Displays detailed information about a single forSale.
/// This is the main product detail page for buyers.
class ForSaleDetailScreen extends ConsumerStatefulWidget {
  final String forSaleId;

  const ForSaleDetailScreen({super.key, required this.forSaleId});

  @override
  ConsumerState<ForSaleDetailScreen> createState() =>
      _ForSaleDetailScreenState();
}

class _ForSaleDetailScreenState extends ConsumerState<ForSaleDetailScreen> {
  static const String _title = 'Detail ForSale';

  @override
  Widget build(BuildContext context) {
    final forSaleAsync = ref.watch(forSaleDetailProvider(widget.forSaleId));

    // CANONICAL STATE SURFACE — the same loading / error / not-found
    // vocabulary the Auction detail renders. CommerceDetailStates is the
    // single authority for both sale channels; raw errors never reach the
    // screen, they stay in the provider/log.
    return forSaleAsync.when(
      loading: () => CommerceDetailStates.loading(title: _title),
      error: (_, _) => CommerceDetailStates.error(
        title: _title,
        headline: 'Gagal Memuat For Sale',
        message: 'Data belum bisa dimuat. Coba lagi nanti.',
        actionLabel: 'Coba Lagi',
        onAction: () => ref.invalidate(forSaleDetailProvider(widget.forSaleId)),
      ),
      data: (forSale) {
        if (forSale == null) {
          return CommerceDetailStates.notFound(
            title: _title,
            headline: 'For Sale Tidak Ditemukan',
            message: 'For sale ini mungkin telah dihapus atau ID tidak valid.',
            actionLabel: 'Kembali',
            onAction: () => Navigator.pop(context),
          );
        }
        return _buildDetail(context, forSale);
      },
    );
  }

  Widget _buildDetail(BuildContext context, ForSale forSale) {
    return Scaffold(
      appBar: AppBarCustom(
        title: _title,
        showBackButton: true,
        actions: [
          Builder(
            builder: (context) {
              final authState = ref.watch(authControllerProvider);

              if (authState is! AuthStateAuthenticated) {
                return const SizedBox.shrink();
              }

              final isOwner = forSale.sellerId == authState.user.id;
              return Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  // Promotion is contract-based; the seller entry point is the
                  // canonical promotion management list (owner-only — mirrors
                  // the Auction action bar).
                  if (isOwner && forSale.status == ForSaleStatus.active)
                    IconButton(
                      onPressed: () =>
                          context.push(RoutePaths.sellerCanonicalPromotions),
                      icon: const Icon(Icons.campaign_outlined),
                      tooltip: 'Promote',
                    ),
                  // Save button — non-owners only.
                  if (!isOwner)
                    CommerceSavedItemActionButton(
                      targetType: 'for_sale',
                      targetId: forSale.forSaleId,
                      label: 'Simpan',
                      activeLabel: 'Tersimpan',
                      icon: Icons.bookmark_border,
                      activeIcon: Icons.bookmark,
                    ),
                  // Share button — all authenticated users can share a forSale to feed.
                  IconButton(
                    onPressed: () => _handleShareForSale(context, forSale),
                    icon: const Icon(Icons.share_outlined),
                    tooltip: 'Bagikan',
                  ),
                  // Report button — non-owners only.
                  if (!isOwner)
                    PopupMoreOptionsButton(
                      contentType: PopupMoreOptionsContentType.forSale,
                      isCreator: false,
                      isDeleting: false,
                      onReport: () => _handleReportForSale(context, forSale),
                    ),
                ],
              );
            },
          ),
        ],
      ),
      body: SafeArea(
        bottom: false,
        child: RefreshIndicator(
          onRefresh: () async =>
              ref.invalidate(forSaleDetailProvider(widget.forSaleId)),
          child: _buildForSaleContent(context, forSale),
        ),
      ),
      bottomNavigationBar: _ForSaleDetailActionBar(forSaleId: widget.forSaleId),
    );
  }

  Future<void> _handleReportForSale(
    BuildContext context,
    ForSale forSale,
  ) async {
    final authState = ref.read(authControllerProvider);
    if (authState is! AuthStateAuthenticated) {
      if (mounted) {
        AppSnackBar.showError(
          context,
          'Silakan masuk untuk melaporkan forSale',
        );
      }
      return;
    }

    if (forSale.sellerId == authState.user.id) {
      if (mounted) {
        AppSnackBar.showError(
          context,
          'Tidak dapat melaporkan forSale milik sendiri',
        );
      }
      return;
    }

    if (!mounted) return;
    await ReportSubmissionDialog.show(
      context,
      targetId: forSale.forSaleId,
      targetType: ReportTargetType.forSale,
      targetTitle: forSale.title,
    );
  }

  Future<void> _handleShareForSale(
    BuildContext context,
    ForSale forSale,
  ) async {
    final shareTarget = ShareTarget(
      id: forSale.forSaleId,
      type: ExternalShareType.forSale,
      title: forSale.title,
      description: forSale.formattedPrice,
      imageUrl: forSale.media.isNotEmpty
          ? forSale.media.first.originalUrl
          : null,
    );

    await ShareBottomSheet.show(
      context: context,
      target: shareTarget,
      canSharePost: false,
    );
  }

  Widget _buildForSaleContent(BuildContext context, ForSale forSale) {
    // CANONICAL DETAIL SKELETON (identical to Auction): media block →
    // title → section cards in the shared 16-margin frame → seller card.
    return CustomScrollView(
      slivers: [
        SliverToBoxAdapter(child: _ForSaleDetailMedia(forSale: forSale)),
        SliverToBoxAdapter(child: _ForSaleDetailTitle(forSale: forSale)),
        SliverToBoxAdapter(child: _ForSalePriceSection(forSale: forSale)),
        // Shared Product content — the SAME card the Auction sibling
        // renders, fed through the canonical fromForSale mapping
        // (attributes, shipping readiness and description live here).
        SliverToBoxAdapter(
          child: Padding(
            padding: const EdgeInsets.fromLTRB(AppMetrics.p16, AppMetrics.p0, AppMetrics.p16, AppMetrics.p16),
            child: CommerceCommonProductDetailSection(
              title: 'Detail Produk',
              data: CommerceCommonProductDetailsData.fromForSale(forSale),
            ),
          ),
        ),
        // Seller block — single render authority for both sale channels.
        SliverToBoxAdapter(
          child: CommerceDetailSellerCard(
            sellerId: forSale.sellerId,
            username: forSale.sellerUsername,
            storeName: forSale.sellerFarmName,
            avatarUrl: forSale.sellerAvatar,
            sellerUserLifecycle: forSale.sellerUserLifecycle,
            sellerTrustLifecycle: forSale.sellerTrustLifecycle,
            tier: forSale.sellerTier,
          ),
        ),
      ],
    );
  }
}

/// Canonical DETAIL MEDIA BLOCK — identical to the Auction header: the
/// shared `MediaCarouselWidget` at 4/3, edge to edge, no raw
/// `Image.network`, no local `PageView` controller. When the payload
/// carries no usable URL the same neutral placeholder renders instead.
class _ForSaleDetailMedia extends StatelessWidget {
  final ForSale forSale;

  const _ForSaleDetailMedia({required this.forSale});

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;

    if (forSale.media.isNotEmptyUrls) {
      return MediaCarouselWidget(
        media: forSale.media,
        aspectRatio: 4 / 3,
        borderRadius: BorderRadius.zero,
      );
    }

    return Container(
      height: 225,
      color: colorScheme.surfaceContainerHighest,
      child: Center(
        child: Icon(
          Icons.image_outlined,
          size: 64,
          color: colorScheme.onSurfaceVariant,
        ),
      ),
    );
  }
}

/// Canonical detail title block — the same slot, style and spacing the
/// Auction detail uses right under the media gallery.
class _ForSaleDetailTitle extends StatelessWidget {
  final ForSale forSale;

  const _ForSaleDetailTitle({required this.forSale});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(AppMetrics.p16, AppMetrics.p16, AppMetrics.p16, AppMetrics.p16),
      child: Text(
        forSale.title,
        style: Theme.of(context).textTheme.headlineSmall,
      ),
    );
  }
}

/// ForSale channel value block — the price/stock card that mirrors the
/// Auction countdown block: the headline transaction value first, then the
/// channel facts, in the canonical 16-margin [CommerceDetailSectionCard]
/// frame.
class _ForSalePriceSection extends StatelessWidget {
  final ForSale forSale;

  const _ForSalePriceSection({required this.forSale});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;

    return CommerceDetailSectionCard(
      margin: const EdgeInsets.fromLTRB(AppMetrics.p16, AppMetrics.p0, AppMetrics.p16, AppMetrics.p16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            forSale.formattedPrice,
            style: theme.textTheme.headlineMedium?.copyWith(
              color: colorScheme.primary,
              fontWeight: FontWeight.bold,
            ),
          ),
          const SizedBox(height: 8),
          Container(
            padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p12, vertical: AppMetrics.p8),
            decoration: BoxDecoration(
              color: colorScheme.surfaceContainerHighest,
              borderRadius: BorderRadius.circular(AppShape.r8),
            ),
            child: Row(
              children: [
                Icon(
                  Icons.info_outline,
                  size: 16,
                  color: colorScheme.onSurfaceVariant,
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    forSale.isNegotiable
                        ? 'Beli langsung — penawaran bisa diajukan lewat chat'
                        : 'Beli langsung — harga pas tanpa tawar',
                    style: TextStyle(
                      fontSize: AppType.s13,
                      color: colorScheme.onSurfaceVariant,
                      fontStyle: FontStyle.italic,
                    ),
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: 12),
          CommerceDetailLabelValue(
            label: 'Stok',
            value: forSale.stock > 0 ? '${forSale.stock} tersedia' : 'Habis',
          ),
        ],
      ),
    );
  }
}

/// Shown in the bottom bar position when the seller's subscription is expired.
/// Replaces the BuyNow button with a visible explanation so buyers understand
/// why no transaction action is available, rather than seeing a blank bottom.
class _SellerInactiveBanner extends StatelessWidget {
  const _SellerInactiveBanner();

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p16, vertical: AppMetrics.p12),
      decoration: BoxDecoration(
        color: theme.scaffoldBackgroundColor,
        border: Border(
          top: BorderSide(color: theme.colorScheme.outlineVariant),
        ),
      ),
      child: SafeArea(
        child: Row(
          children: [
            Icon(
              Icons.pause_circle_outline,
              size: 20,
              color: theme.colorScheme.onSurfaceVariant,
            ),
            const SizedBox(width: 10),
            Expanded(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    'Penjual tidak aktif',
                    style: theme.textTheme.bodyMedium?.copyWith(
                      fontWeight: FontWeight.w600,
                      color: theme.colorScheme.onSurface,
                    ),
                  ),
                  const SizedBox(height: 2),
                  Text(
                    'Transaksi baru tidak tersedia untuk seller ini.',
                    style: theme.textTheme.bodySmall?.copyWith(
                      color: theme.colorScheme.onSurfaceVariant,
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

/// For Sale Detail Action Bar — canonical viewer-capability driven.
///
/// Action authority comes exclusively from the detail wire's
/// `viewer_capabilities` slot (backend EvaluateForSaleViewerCapabilities):
/// can_chat / can_negotiate / can_buy. The bar no longer re-derives
/// transaction permission from raw status/stock/seller-lifecycle locally.
///
/// - Guest (Model B): affordances stay visible; any CTA routes to the
///   canonical sign-in flow.
/// - Owner: buyer action bar not applicable (owner actions live elsewhere).
/// - Buyer: Chat/Nego/Buy Now per capability; an all-false capability set
///   (seller-trust inactive) renders the explanatory inactive banner.
class _ForSaleDetailActionBar extends ConsumerWidget {
  final String forSaleId;

  const _ForSaleDetailActionBar({required this.forSaleId});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final authState = ref.watch(authControllerProvider);
    final forSaleAsync = ref.watch(forSaleDetailProvider(forSaleId));
    final forSale = forSaleAsync.value;
    if (forSale == null) return const SizedBox.shrink();

    final isAuthenticated = authState is AuthStateAuthenticated;

    // Owner: buyer action bar not applicable.
    if (isAuthenticated && forSale.sellerId == authState.user.id) {
      return const SizedBox.shrink();
    }

    // Guest (Model B): affordances visible; the auth boundary redirects to
    // the canonical login flow on tap. Nego/Buy affordance uses raw facts
    // for PRESENTATION only (permission is never granted locally).
    if (!isAuthenticated) {
      return _ForSaleActionBar(
        forSale: forSale,
        guest: true,
        canChat: true,
        canNegotiate: forSale.isNegotiable,
        canBuy: forSale.productId != null && forSale.stock > 0,
        unavailable: false,
      );
    }

    final caps = forSale.viewerCapabilities;
    // Non-detail payload safety: no viewer-scoped capability slot on
    // list/search payloads → no transaction CTA.
    if (caps == null) return const SizedBox.shrink();

    // Buyer with no available action (seller-trust inactive) → banner.
    if (!caps.canChat && !caps.canNegotiate && !caps.canBuy) {
      return const _SellerInactiveBanner();
    }

    final unavailable = caps.canChat && !caps.canBuy && !caps.canNegotiate;
    return _ForSaleActionBar(
      forSale: forSale,
      guest: false,
      canChat: caps.canChat,
      canNegotiate: caps.canNegotiate,
      canBuy: caps.canBuy,
      unavailable: unavailable,
    );
  }
}

/// Renders the buyer action row(s) from canonical capability facts.
class _ForSaleActionBar extends ConsumerWidget {
  final ForSale forSale;
  final bool guest;
  final bool canChat;
  final bool canNegotiate;
  final bool canBuy;
  final bool unavailable;

  const _ForSaleActionBar({
    required this.forSale,
    required this.guest,
    required this.canChat,
    required this.canNegotiate,
    required this.canBuy,
    required this.unavailable,
  });

  void _requireLogin(BuildContext context) {
    context.push(RoutePaths.signIn);
  }

  Future<void> _openChat(
    BuildContext context,
    WidgetRef ref, {
    required bool negotiate,
  }) async {
    await openCommerceChat(
      context: context,
      ref: ref,
      reference: ShareReference.forSale(
        forSaleId: forSale.forSaleId,
        title: forSale.title,
        imageUrl: forSale.media.isNotEmpty
            ? (forSale.media.first.thumbnailUrl ??
                  forSale.media.first.originalUrl)
            : null,
        isAvailable: forSale.isAvailable,
        isSold: forSale.stock == 0,
      ),
      sellerId: forSale.sellerId,
      autoOpenNegotiation: negotiate,
    );
  }

  void _buyNow(BuildContext context) {
    if (guest) {
      _requireLogin(context);
      return;
    }
    final productId = forSale.productId;
    if (productId == null || productId.isEmpty) return;
    final uri = Uri(
      path: '/checkout/${forSale.forSaleId}',
      queryParameters: {'product_id': productId},
    );
    context.push(uri.toString());
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);
    final showSecondary = canChat || canNegotiate;
    final showPrimary = canBuy;

    return Container(
      padding: const EdgeInsets.fromLTRB(AppMetrics.p16, AppMetrics.p8, AppMetrics.p16, AppMetrics.p12),
      decoration: BoxDecoration(
        color: theme.scaffoldBackgroundColor,
        border: Border(
          top: BorderSide(color: theme.colorScheme.outlineVariant),
        ),
      ),
      child: SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            if (unavailable)
              Container(
                margin: const EdgeInsets.only(bottom: AppMetrics.p8),
                padding: const EdgeInsets.symmetric(
                  horizontal: AppMetrics.p12,
                  vertical: AppMetrics.p8,
                ),
                decoration: BoxDecoration(
                  color: theme.colorScheme.surfaceContainerHighest,
                  borderRadius: BorderRadius.circular(AppShape.r8),
                ),
                child: Text(
                  'Item sudah tidak tersedia untuk dibeli',
                  style: theme.textTheme.bodySmall?.copyWith(
                    color: theme.colorScheme.onSurfaceVariant,
                  ),
                ),
              ),
            if (showSecondary)
              Row(
                children: [
                  if (canChat)
                    Expanded(
                      child: _secondaryActionButton(
                        context,
                        icon: Icons.chat_bubble_outline,
                        label: 'Chat',
                        onTap: () => _openChat(context, ref, negotiate: false),
                      ),
                    ),
                  if (canChat && canNegotiate) const SizedBox(width: 8),
                  if (canNegotiate)
                    Expanded(
                      child: _secondaryActionButton(
                        context,
                        icon: Icons.handshake_outlined,
                        label: 'Ajukan Penawaran',
                        onTap: () => _openChat(context, ref, negotiate: true),
                      ),
                    ),
                ],
              ),
            if (showSecondary && showPrimary) const SizedBox(height: 8),
            if (showPrimary)
              SizedBox(
                height: 48,
                child: ElevatedButton(
                  onPressed: () => _buyNow(context),
                  child: const Text(
                    'Beli Sekarang',
                    style: TextStyle(fontSize: AppType.s16, fontWeight: FontWeight.bold),
                  ),
                ),
              ),
          ],
        ),
      ),
    );
  }

  Widget _secondaryActionButton(
    BuildContext context, {
    required IconData icon,
    required String label,
    required VoidCallback onTap,
  }) {
    return OutlinedButton.icon(
      onPressed: onTap,
      icon: Icon(icon, size: 18),
      label: Text(label),
      style: OutlinedButton.styleFrom(
        minimumSize: const Size.fromHeight(44),
      ),
    );
  }
}
