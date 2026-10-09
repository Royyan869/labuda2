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
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/checkout_intent.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/for_sale_providers.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_providers.dart';
import 'package:labuda/domains/chat/chat/presentation/models/pending_commerce_attachment.dart';
import 'package:labuda/domains/chat/chat/presentation/utils/commerce_chat_navigation.dart';
import 'package:labuda/domains/commerce/negotiation/negotiation/presentation/providers/negotiation_providers.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/domains/social/share/share.dart';
import 'package:labuda/domains/system/report/domain/entities/entities.dart';
import 'package:labuda/domains/commerce/negotiation/negotiation/presentation/widgets/negotiation_offer_sheet.dart';
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
    // BUSINESS TRUTH (Owner): the author views their own listing WITHOUT a
    // viewer-directed bottom surface. The bottom slot mirrors the surface
    // exactly — NULL for the author, never a zero-height stand-in (a
    // non-null slot makes the Scaffold strip the body's bottom system
    // padding while reserving no region).
    //
    // ONE body law for both states: the body SafeArea below is
    // unconditional. With a bar the framework strips the body's bottom
    // padding (`removeBottomPadding: widget.bottomNavigationBar != null`
    // in Scaffold) so the SafeArea contributes NOTHING and BottomActionBar
    // owns the live system inset; with the slot null the SafeArea IS the
    // sole bottom-inset authority. No `if author` layout branch, no fixed
    // clearance, no manual inset arithmetic.
    final authState = ref.watch(authControllerProvider);
    final isOwner =
        authState is AuthStateAuthenticated &&
        forSale.sellerId == authState.user.id;

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
                  // Save button — non-owners only.
                   if (!isOwner && forSale.status == ForSaleStatus.active)
                     CommerceSavedItemActionButton(
                      targetType: 'for_sale',
                      targetId: forSale.forSaleId,
                      label: 'Simpan',
                      activeLabel: 'Tersimpan',
                      icon: Icons.bookmark_border,
                      activeIcon: Icons.bookmark,
                    ),
                   if (forSale.status == ForSaleStatus.active)
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
        child: RefreshIndicator(
          onRefresh: () async =>
              ref.invalidate(forSaleDetailProvider(widget.forSaleId)),
          child: _buildForSaleContent(context, forSale),
        ),
      ),
      bottomNavigationBar: isOwner
          ? null
          : _ForSaleDetailActionBar(forSale: forSale),
    );
  }

  Future<void> _handleReportForSale(
    BuildContext context,
    ForSale forSale,
  ) async {
    final authState = ref.read(authControllerProvider);
    if (authState is! AuthStateAuthenticated) {
      if (mounted) {
        ref.read(navigationHandlerProvider).navigateToSignIn();
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
    await context.push<bool>(
      RoutePaths.reportLocation(
        targetType: ReportTargetType.forSale.name,
        targetId: forSale.forSaleId,
        targetTitle: forSale.title,
      ),
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
            padding: const EdgeInsets.fromLTRB(
              AppMetrics.p16,
              AppMetrics.p0,
              AppMetrics.p16,
              AppMetrics.p16,
            ),
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
            originLine: forSale.publicOriginLine,
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
/// shared `MediaCarouselWidget` at 4:5 contain (same as the card — koi never
/// cropped), edge to edge, tap opens the fullscreen viewer. When the payload
/// carries no usable URL the same neutral placeholder renders instead.
class _ForSaleDetailMedia extends StatelessWidget {
  final ForSale forSale;

  const _ForSaleDetailMedia({required this.forSale});

  void _openViewer(BuildContext context, int index) {
    if (forSale.media.isEmpty) return;
    showDialog(
      context: context,
      barrierColor: Theme.of(context).colorScheme.scrim.withValues(alpha: 0.87),
      builder: (_) => MediaViewerWidget(
        media: forSale.media,
        initialIndex: index.clamp(0, forSale.media.length - 1),
        title: forSale.title,
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;

    if (forSale.media.isNotEmptyUrls) {
      return MediaCarouselWidget(
        media: forSale.media,
        aspectRatio: 4 / 5,
        fit: BoxFit.contain,
        borderRadius: BorderRadius.zero,
        onImageTapWithIndex: (index) => _openViewer(context, index),
      );
    }

    return AspectRatio(
      aspectRatio: 4 / 5,
      child: Container(
        color: colorScheme.surfaceContainerHighest,
        child: Center(
          child: Icon(
            Icons.image_outlined,
            size: AppIconSize.display,
            color: colorScheme.onSurfaceVariant,
          ),
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
      padding: const EdgeInsets.fromLTRB(
        AppMetrics.p16,
        AppMetrics.p16,
        AppMetrics.p16,
        AppMetrics.p16,
      ),
      child: Text(
        forSale.title,
        style: Theme.of(context).textTheme.headlineSmall,
      ),
    );
  }
}

/// ForSale channel value block with canonical price, negotiation, and stock
/// presentation.
class _ForSalePriceSection extends StatelessWidget {
  final ForSale forSale;

  const _ForSalePriceSection({required this.forSale});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;

    return CommerceDetailSectionCard(
      margin: const EdgeInsets.fromLTRB(
        AppMetrics.p16,
        AppMetrics.p0,
        AppMetrics.p16,
        AppMetrics.p16,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.center,
            children: [
              Expanded(
                child: Text(
                  forSale.formattedPrice,
                  style: theme.textTheme.headlineMedium?.copyWith(
                    color: colorScheme.primary,
                    fontWeight: FontWeight.bold,
                  ),
                ),
              ),
              if (forSale.isNegotiable)
                Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: AppMetrics.p8,
                    vertical: AppMetrics.p4,
                  ),
                  decoration: BoxDecoration(
                    color: colorScheme.secondaryContainer,
                    borderRadius: BorderRadius.circular(AppShape.r8),
                  ),
                  child: Text(
                    'Nego',
                    style: theme.textTheme.labelMedium?.copyWith(
                      color: colorScheme.onSecondaryContainer,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                ),
            ],
          ),
          if (forSale.stock == 0) ...[
            const SizedBox(height: 12),
            const CommerceDetailLabelValue(label: 'Status', value: 'Habis'),
          ] else if (forSale.stock > 1) ...[
            const SizedBox(height: 12),
            CommerceDetailLabelValue(
              label: 'Stok',
              value: '${forSale.stock} tersedia',
            ),
          ],
        ],
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
/// - Guest or missing capability: no commerce action bar.
/// - Owner: never mounted — the screen's `bottomNavigationBar` slot is NULL
///   for the author.
/// - Buyer: Chat/Nego/Buy Now per canonical capability.
class _ForSaleDetailActionBar extends ConsumerWidget {
  final ForSale forSale;

  const _ForSaleDetailActionBar({required this.forSale});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final authState = ref.watch(authControllerProvider);

    final isAuthenticated = authState is AuthStateAuthenticated;

    final caps = forSale.viewerCapabilities;
    if (!isAuthenticated || caps == null) {
      return const SizedBox.shrink();
    }

    final unavailable = caps.canChat && !caps.canBuy && !caps.canNegotiate;
    return _ForSaleActionBar(
      forSale: forSale,
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
  final bool canChat;
  final bool canNegotiate;
  final bool canBuy;
  final bool unavailable;

  const _ForSaleActionBar({
    required this.forSale,
    required this.canChat,
    required this.canNegotiate,
    required this.canBuy,
    required this.unavailable,
  });

  Future<void> _openChat(BuildContext context, WidgetRef ref) async {
    await openCommerceChat(
      context: context,
      ref: ref,
      attachment: PendingCommerceAttachment.forSale(
        forSaleId: forSale.forSaleId,
        title: forSale.title,
        imageUrl: forSale.media.isNotEmpty
            ? (forSale.media.first.thumbnailUrl ??
                  forSale.media.first.originalUrl)
            : null,
        price: forSale.price.toInt(),
      ),
      sellerId: forSale.sellerId,
    );
  }

  /// CANONICAL NEGO PATH (owner decision): the offer nominal is entered in
  /// a bottom sheet ON the detail screen and posted straight to the
  /// chat-room-scoped negotiation endpoint. NO navigation to chat, NO
  /// silently auto-sent product card — the sheet reports terkirim/gagal
  /// and the detail stays on screen.
  Future<void> _openNegotiationOffer(
    BuildContext context,
    WidgetRef ref,
  ) async {
    final sent = await NegotiationOfferSheet.show(
      context: context,
      productTitle: forSale.title,
      onSubmit: (price) => _submitNegotiationOffer(ref, price),
    );
    if (!sent || !context.mounted) return;
    AppSnackBar.showSuccess(context, 'Penawaran terkirim ke penjual');
  }

  /// Returns `null` on success, or the failure copy to render inline in the
  /// sheet. Room resolution is required by the room-scoped contract but
  /// never navigates — it only materializes the canonical room id.
  Future<String?> _submitNegotiationOffer(WidgetRef ref, int price) async {
    final authState = ref.read(authControllerProvider);
    if (authState is! AuthStateAuthenticated) {
      return 'Silakan masuk untuk mengirim penawaran';
    }
    final chat = await ref
        .read(chatListProvider.notifier)
        .getOrCreateChat(
          userId: authState.user.id,
          otherUserId: forSale.sellerId,
        );
    if (chat == null) {
      return ref.read(chatListProvider).error ?? 'Gagal mengirim penawaran';
    }
    final result = await ref
        .read(negotiationNotifierProvider.notifier)
        .createNegotiation(
          chatRoomId: chat.id,
          fixedPriceSaleId: forSale.forSaleId,
          price: price,
        );
    if (result.isSuccess && result.data != null) return null;
    return 'Penawaran gagal. Coba lagi.';
  }

  Future<void> _buyNow(BuildContext context, WidgetRef ref) async {
    // ONE FUNNEL: the commerce intent resolves the LIVE listing, enforces the
    // seller trust gate, resolves the physical product id and carries the deal
    // binding (viewer_negotiation_id → negotiation_id). The detail screen
    // builds no route and plumbs no product id of its own.
    await openForSaleCheckout(
      context,
      ref,
      CheckoutIntent(
        forSaleId: forSale.forSaleId,
        // DEAL BINDING: detail wire's viewer_negotiation_id — checkout prices
        // at the agreed deal, never list (owner truth: valid 24h).
        negotiationId: forSale.viewerNegotiationId,
      ),
    );
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);

    // CANONICAL ONE-ROW CTA BAR: [Chat icon+label] [Nego icon+label]
    // [Beli Sekarang] all in the SAME row. Chrome, spacing, and the icon
    // affordance shape are owned by [BottomActionBar]; this widget only
    // decides which affordances the viewer capabilities allow.
    return BottomActionBar(
      header: unavailable
          ? Container(
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
            )
          : null,
      leading: [
        if (canChat)
          BottomBarIconAction(
            icon: Icons.chat_bubble_outline,
            label: 'Chat',
            onPressed: () => _openChat(context, ref),
          ),
        if (canNegotiate)
          BottomBarIconAction(
            icon: Icons.handshake_outlined,
            label: 'Tawar',
            onPressed: () => _openNegotiationOffer(context, ref),
          ),
      ],
      primary: canBuy
          ? BottomBarAction(
              label: 'Beli Sekarang',
              onPressed: () => _buyNow(context, ref),
            )
          : null,
    );
  }
}
