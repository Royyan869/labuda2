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
import 'package:labuda/domains/chat/chat/presentation/utils/commerce_chat_navigation.dart';
import 'package:labuda/domains/commerce/negotiation/negotiation/presentation/providers/negotiation_providers.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/domains/social/share/share.dart';
import 'package:labuda/domains/system/report/domain/entities/entities.dart';
import 'package:labuda/domains/system/report/presentation/dialogs/report_submission_dialog.dart';
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
            forSale.formattedPrice,
            style: theme.textTheme.headlineMedium?.copyWith(
              color: colorScheme.primary,
              fontWeight: FontWeight.bold,
            ),
          ),
          const SizedBox(height: 8),
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
                    forSale.isNegotiable
                        ? 'Beli langsung — penawaran bisa diajukan lewat chat'
                        : 'Beli langsung — harga pas tanpa tawar',
                    style: TextStyle(
                      fontSize: AppType.s14,
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
      padding: const EdgeInsets.symmetric(
        horizontal: AppMetrics.p16,
        vertical: AppMetrics.p12,
      ),
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
              size: AppIconSize.action,
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
/// - Buyer: Chat/Nego/Buy Now per capability; the seller-trust axis alone
///   renders the explanatory inactive banner (never the capability set — an
///   all-false set also means the viewer identity never reached backend).
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

    // SELLER-TRUST AXIS is the ONLY source of the "Penjual tidak aktif"
    // banner. An all-false capability set is NOT: it also occurs when the
    // viewer identity never reached the backend, and that must never be
    // presented to the user as a seller problem.
    if (forSale.sellerTrustLifecycle != ContentLifecycle.active) {
      return const _SellerInactiveBanner();
    }

    // Caps carry at least one affordance → canonical capability-driven bar.
    if (caps != null && (caps.canChat || caps.canNegotiate || caps.canBuy)) {
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

    // CAPS UNUSABLE — slot absent (non-detail payload) or all-false (viewer
    // identity never reached the backend). NEVER render a blank bottom bar:
    // fall back to the same presentation-only facts the guest branch uses.
    // Permission stays server-side; the bar only promises an affordance.
    return _ForSaleActionBar(
      forSale: forSale,
      guest: false,
      canChat: true,
      canNegotiate: forSale.isNegotiable,
      canBuy: forSale.productId != null && forSale.stock > 0,
      unavailable: false,
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

  Future<void> _openChat(BuildContext context, WidgetRef ref) async {
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
    if (guest) {
      _requireLogin(context);
      return;
    }
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
    if (guest) {
      _requireLogin(context);
      return;
    }
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

    return Container(
      padding: const EdgeInsets.fromLTRB(
        AppMetrics.p16,
        AppMetrics.p8,
        AppMetrics.p16,
        AppMetrics.p12,
      ),
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
            // CANONICAL ONE-ROW CTA BAR — byte-shape parity with
            // AuctionDetailBottomBar: [Chat icon+label] [Nego icon+label]
            // [Beli Sekarang] all in the SAME row. A second CTA row is a
            // forbidden design.
            Row(
              children: [
                if (canChat)
                  _iconActionButton(
                    context,
                    icon: Icons.chat_bubble_outline,
                    label: 'Chat',
                    onTap: () => _openChat(context, ref),
                    expand: !canBuy,
                  ),
                if (canChat && (canNegotiate || canBuy))
                  const SizedBox(width: 12),
                if (canNegotiate)
                  _iconActionButton(
                    context,
                    icon: Icons.handshake_outlined,
                    label: 'Nego',
                    onTap: () => _openNegotiationOffer(context, ref),
                    expand: !canBuy,
                  ),
                if ((canChat || canNegotiate) && canBuy)
                  const SizedBox(width: 12),
                if (canBuy)
                  Expanded(
                    child: SizedBox(
                      height: AppContentSize.control,
                      child: ElevatedButton(
                        onPressed: () => _buyNow(context, ref),
                        child: const Text(
                          'Beli Sekarang',
                          style: TextStyle(
                            fontSize: AppType.s16,
                            fontWeight: FontWeight.bold,
                          ),
                        ),
                      ),
                    ),
                  ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  /// Icon + label-under affordance — the SAME shape as
  /// AuctionDetailBottomBar._buildActionButton (icon 20 → 2px → label s10).
  /// [expand] stretches it only when the primary Buy CTA is absent, so the
  /// row still never wraps into a second line.
  Widget _iconActionButton(
    BuildContext context, {
    required IconData icon,
    required String label,
    required VoidCallback onTap,
    required bool expand,
  }) {
    final theme = Theme.of(context);
    final button = InkWell(
      onTap: onTap,
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, color: theme.colorScheme.onSurfaceVariant, size: AppIconSize.action),
          const SizedBox(height: 2),
          Text(label, style: const TextStyle(fontSize: AppType.s12)),
        ],
      ),
    );
    return expand ? Expanded(child: button) : button;
  }
}
