/// ForSale Detail Screen
///
/// Product detail page for individual forSales.
/// This is the main product detail page for buyers.
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/core/common/types/preparation_time.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/entities/for_sale.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/for_sale_providers.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/target_type.dart';
import 'package:labuda/domains/commerce/pricing/promotion/presentation/providers/promotion_providers.dart';
import 'package:labuda/domains/user/profile/profile.dart' show userDataProvider;
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/governance/seller_tier_badge.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/domains/social/share/share.dart';
import 'package:labuda/shared/utils/commerce_seller_identity.dart';
import 'package:labuda/domains/system/report/domain/entities/entities.dart';
import 'package:labuda/domains/system/report/presentation/dialogs/report_submission_dialog.dart';

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
  @override
  Widget build(BuildContext context) {
    final listingAsync = ref.watch(forSaleDetailProvider(widget.forSaleId));

    return Scaffold(
      appBar: AppBar(
        title: const Text('Detail Listing'),
        actions: [
          Builder(
            builder: (context) {
              final authState = ref.watch(authControllerProvider);
              final listing = listingAsync.value;

              if (listing == null || authState is! AuthStateAuthenticated) {
                return const SizedBox.shrink();
              }

              final isOwner = listing.sellerId == authState.user.id;
              return Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  // Share button — all authenticated users can share a listing to feed.
                  IconButton(
                    onPressed: () => _handleShareListing(context, listing),
                    icon: const Icon(Icons.share_outlined),
                    tooltip: 'Bagikan',
                  ),
                  // Report button — non-owners only.
                  if (!isOwner)
                    PopupMoreOptionsButton(
                      contentType: PopupMoreOptionsContentType.listing,
                      isCreator: false,
                      isDeleting: false,
                      onReport: () => _handleReportListing(context, listing),
                    ),
                ],
              );
            },
          ),
        ],
      ),
      body: listingAsync.when(
        data: (listing) {
          if (listing == null) {
            return const Center(child: Text('Listing not found'));
          }
          return _buildForSaleContent(context, listing);
        },
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (error, _) =>
            const Center(child: Text('Data belum bisa dimuat.')),
      ),
      bottomNavigationBar: _BuyNowBottomBar(forSaleId: widget.forSaleId),
    );
  }

  Future<void> _handleReportListing(
    BuildContext context,
    ForSale listing,
  ) async {
    final authState = ref.read(authControllerProvider);
    if (authState is! AuthStateAuthenticated) {
      if (mounted) {
        AppSnackBar.showError(
          context,
          'Silakan masuk untuk melaporkan listing',
        );
      }
      return;
    }

    if (listing.sellerId == authState.user.id) {
      if (mounted) {
        AppSnackBar.showError(
          context,
          'Tidak dapat melaporkan listing milik sendiri',
        );
      }
      return;
    }

    if (!mounted) return;
    await ReportSubmissionDialog.show(
      context,
      targetId: listing.forSaleId,
      targetType: ReportTargetType.forSale,
      targetTitle: listing.title,
    );
  }

  Future<void> _handleShareListing(
    BuildContext context,
    ForSale listing,
  ) async {
    final shareTarget = ShareTarget(
      id: listing.forSaleId,
      type: ExternalShareType.listing,
      title: listing.title,
      description: 'Rp ${listing.price.toStringAsFixed(0)}',
      imageUrl: listing.media.isNotEmpty
          ? listing.media.first.originalUrl
          : null,
    );

    await ShareBottomSheet.show(
      context: context,
      target: shareTarget,
      canSharePost: false,
    );
  }

  Widget _buildPreparationTimeSection(
    BuildContext context,
    PreparationTime preparationTime,
    String? preparationNote,
  ) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final note = preparationNote;

    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: preparationTime.isImmediate
            ? (isDark
                  ? AppColors.successGreen.withValues(alpha: 0.1)
                  : AppColors.successGreen.withValues(alpha: 0.08))
            : (isDark
                  ? AppColors.warning.withValues(alpha: 0.1)
                  : AppColors.warning.withValues(alpha: 0.08)),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(
          color: preparationTime.isImmediate
              ? AppColors.successGreen.withValues(alpha: 0.3)
              : AppColors.warning.withValues(alpha: 0.3),
          width: 1,
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(
                preparationTime.isImmediate
                    ? Icons.flash_on
                    : Icons.access_time,
                size: 16,
                color: preparationTime.isImmediate
                    ? AppColors.successGreen
                    : AppColors.warning,
              ),
              const SizedBox(width: 8),
              Text(
                'Waktu Persiapan',
                style: TextStyle(
                  fontSize: 12,
                  fontWeight: FontWeight.w600,
                  color: isDark
                      ? AppColors.neutralGray300
                      : AppColors.neutralGray700,
                ),
              ),
            ],
          ),
          const SizedBox(height: 8),
          Text(
            preparationTime.isImmediate
                ? 'Siap kirim langsung'
                : 'Estimasi siap kirim: ${preparationTime.displayName.toLowerCase()}',
            style: TextStyle(
              fontSize: 14,
              fontWeight: FontWeight.w600,
              color: preparationTime.isImmediate
                  ? AppColors.successGreen
                  : AppColors.warning,
            ),
          ),
          if (!preparationTime.isImmediate) ...[
            const SizedBox(height: 6),
            Text(
              preparationTime.description,
              style: TextStyle(
                fontSize: 12,
                color: isDark
                    ? AppColors.neutralGray400
                    : AppColors.neutralGray600,
                height: 1.4,
              ),
            ),
          ],
          if (note != null && note.isNotEmpty) ...[
            const SizedBox(height: 8),
            Container(
              padding: const EdgeInsets.all(8),
              decoration: BoxDecoration(
                color: isDark
                    ? AppColors.darkGray700.withValues(alpha: 0.5)
                    : AppColors.neutralGray100,
                borderRadius: BorderRadius.circular(6),
              ),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Icon(
                    Icons.info_outline,
                    size: 14,
                    color: preparationTime.isImmediate
                        ? AppColors.successGreen
                        : AppColors.warning,
                  ),
                  const SizedBox(width: 6),
                  Expanded(
                    child: Text(
                      note,
                      style: TextStyle(
                        fontSize: 11,
                        color: isDark
                            ? AppColors.neutralGray400
                            : AppColors.neutralGray600,
                        height: 1.4,
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ],
          const SizedBox(height: 6),
          Text(
            'Estimasi maksimal, penjual bisa kirim lebih cepat',
            style: TextStyle(
              fontSize: 11,
              color: isDark
                  ? AppColors.neutralGray500
                  : AppColors.neutralGray500,
              fontStyle: FontStyle.italic,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildForSaleContent(BuildContext context, ForSale listing) {
    return SingleChildScrollView(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Listing media images
          if (listing.media.isNotEmpty)
            SizedBox(
              height: 300,
              child: PageView.builder(
                itemCount: listing.media.length,
                itemBuilder: (context, index) {
                  return Image.network(
                    listing.media[index].originalUrl,
                    fit: BoxFit.cover,
                    errorBuilder: (context, error, stackTrace) => Container(
                      color: Theme.of(
                        context,
                      ).colorScheme.surfaceContainerHighest,
                      child: const Center(child: Icon(Icons.image, size: 48)),
                    ),
                  );
                },
              ),
            ),
          // Listing details
          Padding(
            padding: const EdgeInsets.all(16.0),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  listing.title,
                  style: Theme.of(context).textTheme.headlineSmall,
                ),
                const SizedBox(height: 8),
                Text(
                  'Rp ${listing.price.toStringAsFixed(0)}',
                  style: Theme.of(context).textTheme.headlineMedium?.copyWith(
                    color: Theme.of(context).colorScheme.primary,
                    fontWeight: FontWeight.bold,
                  ),
                ),
                const SizedBox(height: 16),

                // ═══════════════════════════════════════════════════════════════════════
                // WAKTU PERSIAPAN (PREPARATION TIME)
                // ═══════════════════════════════════════════════════════════════════════
                // Buyer expectation: Show this BEFORE purchase so buyers know what to expect
                // Uses shared preparation time mapping for consistency across all screens
                _buildPreparationTimeSection(
                  context,
                  listing.preparationTime,
                  listing.preparationNote,
                ),

                const SizedBox(height: 16),
                Text(
                  listing.description,
                  style: Theme.of(context).textTheme.bodyMedium,
                ),
                const SizedBox(height: 16),
                if (listing.location != null) ...[
                  Row(
                    children: [
                      const Icon(Icons.location_on),
                      const SizedBox(width: 8),
                      Text(listing.location!.displayName),
                    ],
                  ),
                  const SizedBox(height: 16),
                ],
                _ForSaleSellerCard(listing: listing),
                const SizedBox(height: 16),
                // Promote button (only for listing owner)
                _PromoteButton(forSaleId: widget.forSaleId),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

/// Promote Button Widget
class _PromoteButton extends ConsumerWidget {
  final String forSaleId;

  const _PromoteButton({required this.forSaleId});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final authState = ref.watch(authControllerProvider);
    final promotionsAsync = ref.watch(
      fixedPriceSaleActivePromotionsProvider(forSaleId),
    );
    final isDark = Theme.of(context).brightness == Brightness.dark;

    // Only show promote button to authenticated users
    if (authState is! AuthStateAuthenticated) {
      return const SizedBox.shrink();
    }

    return promotionsAsync.when(
      data: (result) {
        final activePromotions = result.data ?? [];
        final isPromoted = activePromotions.isNotEmpty;

        if (isPromoted) {
          // Show promoted badge with remaining time
          return _buildPromotedBadge(context, activePromotions.first);
        }

        // Show promote button
        return SizedBox(
          width: double.infinity,
          child: ElevatedButton.icon(
            onPressed: () => _openPromotionScreen(context, ref),
            icon: const Icon(Icons.campaign, size: 18),
            label: const Text('Promosikan Fixed-Price Sale'),
            style: ElevatedButton.styleFrom(
              backgroundColor: isDark
                  ? AppColors.darkGray700
                  : AppColors.neutralGray100,
              foregroundColor: isDark
                  ? AppColors.neutralWhite
                  : AppColors.neutralGray900,
              elevation: 0,
            ),
          ),
        );
      },
      loading: () => const SizedBox.shrink(),
      error: (_, _) => const SizedBox.shrink(),
    );
  }

  Widget _buildPromotedBadge(BuildContext context, promotion) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        gradient: LinearGradient(
          colors: [
            AppColors.primaryRed.withValues(alpha: 0.8),
            AppColors.primaryRed.withValues(alpha: 0.6),
          ],
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
        ),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Row(
        children: [
          const Icon(Icons.star, color: Colors.white, size: 20),
          const SizedBox(width: 8),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Sedang Dipromosikan',
                  style: TextStyle(
                    color: Colors.white,
                    fontSize: 14,
                    fontWeight: FontWeight.bold,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  'Listing Anda sedang muncul di prioritas',
                  style: TextStyle(
                    color: Colors.white.withValues(alpha: 0.9),
                    fontSize: 12,
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  void _openPromotionScreen(BuildContext context, WidgetRef ref) {
    // Get listing title from the forSale detail provider
    final listingAsync = ref.read(forSaleDetailProvider(forSaleId));
    listingAsync.when(
      data: (listing) {
        if (listing != null) {
          context.push(
            RoutePaths.sellerPromotionActivate,
            extra: {
              'preselectedTargetType': TargetType.forSale,
              'preselectedTargetId': forSaleId,
              'preselectedTargetTitle': listing.title,
            },
          );
        }
      },
      loading: () {},
      error: (_, _) {},
    );
  }
}

/// Listing Seller Card
///
/// Bounded commerce trust surface: shows seller identity to the buyer.
///
/// Owner Truth: farmName (Listing.sellerFarmName) is the public seller
/// identity; @username (Listing.sellerUsername) is the public user handle;
/// fullName is private/KYC and is NEVER read here.
///
/// Identity SOURCE PRIORITY:
///   1. Listing entity owner-truth fields (populated by mapper from backend
///      identity scalars: seller_farm_name / seller_username /
///      seller_avatar_url).
///   2. `userDataProvider(sellerId)` is consulted ONLY for username/avatar
///      when the entity carries neither. `user.fullName` is NEVER read.
///
/// MISSING-TRUTH POLICY:
///   - No fake fallback labels ('Penjual', 'Unknown', 'Seller', etc.).
///   - When neither farmName nor any username is known, the card renders
///     `SizedBox.shrink()` — hide rather than fabricate.
///   - Loading: a non-identity placeholder ("Memuat...") is shown.
///   - Error: hidden.
class _ForSaleSellerCard extends ConsumerWidget {
  final ForSale listing;

  const _ForSaleSellerCard({required this.listing});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    // Stage 2 — Seller tier badge. Visible only when:
    //   1. User-identity axis is active (not degraded — checked by outer guard).
    //   2. Seller-trust axis is active (subscription not expired).
    //   3. Tier is "pro" or "elite" (SellerTierBadge hides null/basic/unknown).
    // The badge is computed once here so both the data and loading branches
    // can reference it; only the data branch renders it.
    final tierBadgeVisible =
        !listing.sellerUserLifecycle.isDegraded &&
        listing.sellerTrustLifecycle == ContentLifecycle.active &&
        listing.sellerTier != null;

    // E8.2 — Seller user-identity lifecycle redaction. When the seller's
    // user identity is degraded (banned/deleted), render an italic
    // placeholder + neutral avatar + tap disabled. The listing itself is
    // controlled by `listing.status` and stays visible — seller-user
    // lifecycle MUST NOT hide the item.
    //
    // AXIS BOUNDARY: user-axis gate fires here. Seller-trust axis
    // (sellerTrustLifecycle) is now also consumed for the tier badge
    // gate only — no other UI behaviour changes on trust-axis state here.
    if (listing.sellerUserLifecycle.isDegraded) {
      return _buildDegradedRow(
        context,
        placeholder: listing.sellerUserLifecycle.publicRedactionLabel,
        isDark: isDark,
      );
    }

    final userAsync = ref.watch(userDataProvider(listing.sellerId));

    return userAsync.when(
      data: (user) {
        // Owner-truth identity from the listing entity.
        final farmName = listing.sellerFarmName;
        final hasFarm = farmName != null && farmName.isNotEmpty;
        final entityUsername = listing.sellerUsername;
        final hasEntityUsername =
            entityUsername != null && entityUsername.isNotEmpty;
        final entityAvatar = listing.sellerAvatar;
        final hasEntityAvatar = entityAvatar != null && entityAvatar.isNotEmpty;

        // user-lookup fills username/avatar when the entity lacks them
        // (never user.fullName which is KYC).
        final fallbackUsername = user?.username;
        final fallbackAvatarUrl = user?.avatarUrl;

        final username = hasEntityUsername
            ? entityUsername
            : ((fallbackUsername != null && fallbackUsername.isNotEmpty)
                  ? fallbackUsername
                  : null);
        final hasUsername = username != null;

        final avatarUrl = hasEntityAvatar
            ? entityAvatar
            : ((fallbackAvatarUrl != null && fallbackAvatarUrl.isNotEmpty)
                  ? fallbackAvatarUrl
                  : null);

        // Hide rather than fabricate when no truth is available.
        if (!hasFarm && !hasUsername) {
          return const SizedBox.shrink();
        }

        final identity = buildCommerceSellerIdentity(
          username: username,
          storeName: farmName,
        );
        if (identity == null) {
          return const SizedBox.shrink();
        }

        final row = _buildRow(
          context,
          avatar: CircleAvatar(
            radius: 24,
            backgroundImage: avatarUrl != null ? NetworkImage(avatarUrl) : null,
            child: avatarUrl == null ? const Icon(Icons.person) : null,
          ),
          displayName: identity.line1,
          username: identity.line2,
          onTap: () => ref
              .read(navigationHandlerProvider)
              .navigateToUserProfile(listing.sellerId),
          isDark: isDark,
        );

        if (!tierBadgeVisible) return row;

        // Stage 2 — tier badge placed below identity row as a subtle
        // secondary trust signal. Lifecycle dominates visually (row shows
        // first); badge is a soft reputation hint, NOT a primary trust seal.
        return Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            row,
            const SizedBox(height: 8),
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 12),
              child: SellerTierBadge(tier: listing.sellerTier),
            ),
          ],
        );
      },
      loading: () => _buildRow(
        context,
        avatar: const CircleAvatar(radius: 24, child: Icon(Icons.person)),
        // Non-identity loading hint — does not assert any seller identity.
        displayName: 'Memuat...',
        username: null,
        onTap: null,
        isDark: isDark,
      ),
      error: (_, _) => const SizedBox.shrink(),
    );
  }

  Widget _buildRow(
    BuildContext context, {
    required Widget avatar,
    required String displayName,
    required String? username,
    required VoidCallback? onTap,
    required bool isDark,
  }) {
    final content = Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray700 : AppColors.neutralGray100,
        borderRadius: BorderRadius.circular(8),
      ),
      child: Row(
        children: [
          avatar,
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  displayName,
                  style: const TextStyle(
                    fontSize: 16,
                    fontWeight: FontWeight.bold,
                  ),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
                if (username != null)
                  Text(
                    '@$username',
                    style: TextStyle(
                      fontSize: 12,
                      color: isDark
                          ? AppColors.neutralGray400
                          : AppColors.neutralGray600,
                    ),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
              ],
            ),
          ),
          if (onTap != null)
            Icon(
              Icons.chevron_right,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray600,
            ),
        ],
      ),
    );

    if (onTap == null) return content;
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(8),
      child: content,
    );
  }

  /// E8.2 — Render a degraded seller identity row.
  ///
  /// Italic placeholder label + neutral avatar + tap disabled +
  /// chevron suppressed + username subtitle suppressed. Matches the
  /// redaction vocabulary used by feed (E2.1), comments (E3.1), chat
  /// (E4.3), profile (E5.2/E5.3), and content detail (E6).
  Widget _buildDegradedRow(
    BuildContext context, {
    required String placeholder,
    required bool isDark,
  }) {
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray700 : AppColors.neutralGray100,
        borderRadius: BorderRadius.circular(8),
      ),
      child: Row(
        children: [
          CircleAvatar(
            radius: 24,
            backgroundColor: isDark
                ? AppColors.darkGray600
                : AppColors.neutralGray200,
            child: Icon(
              Icons.person,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray500,
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Text(
              placeholder,
              style: TextStyle(
                fontSize: 16,
                fontWeight: FontWeight.bold,
                fontStyle: FontStyle.italic,
                color: isDark
                    ? AppColors.neutralGray400
                    : AppColors.neutralGray500,
              ),
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
            ),
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
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
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

/// Buy Now Bottom Bar — shown for non-owner authenticated users when listing is available
class _BuyNowBottomBar extends ConsumerWidget {
  final String forSaleId;

  const _BuyNowBottomBar({required this.forSaleId});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final authState = ref.watch(authControllerProvider);
    final listingAsync = ref.watch(forSaleDetailProvider(forSaleId));
    final listing = listingAsync.value;

    // Hide for unauthenticated, owner, or unavailable listing
    if (listing == null || authState is! AuthStateAuthenticated) {
      return const SizedBox.shrink();
    }
    final isOwner = listing.sellerId == authState.user.id;
    if (isOwner || !listing.isAvailable) {
      return const SizedBox.shrink();
    }

    // Seller trust gate — show explanatory banner instead of hiding silently.
    if (listing.sellerTrustLifecycle != ContentLifecycle.active) {
      return const _SellerInactiveBanner();
    }

    final productId = listing.productId;
    if (productId == null || productId.isEmpty) {
      return Container(
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
        decoration: BoxDecoration(
          color: Theme.of(context).scaffoldBackgroundColor,
          border: Border(top: BorderSide(color: AppColors.neutralGray200)),
        ),
        child: SafeArea(
          child: Row(
            children: [
              Icon(
                Icons.info_outline,
                size: 20,
                color: Theme.of(context).colorScheme.onSurfaceVariant,
              ),
              const SizedBox(width: 10),
              Expanded(
                child: Text(
                  'ID produk belum tersedia dari backend. Checkout belum bisa dibuka.',
                  style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                    color: Theme.of(context).colorScheme.onSurfaceVariant,
                  ),
                ),
              ),
            ],
          ),
        ),
      );
    }

    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: Theme.of(context).scaffoldBackgroundColor,
        border: Border(top: BorderSide(color: AppColors.neutralGray200)),
      ),
      child: SafeArea(
        child: SizedBox(
          width: double.infinity,
          height: 48,
          child: ElevatedButton(
            onPressed: () {
              final uri = Uri(
                path: '/checkout/$forSaleId',
                queryParameters: {'product_id': productId},
              );
              context.push(uri.toString());
            },
            style: ElevatedButton.styleFrom(
              backgroundColor: AppColors.primaryRed,
              foregroundColor: Colors.white,
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(12),
              ),
            ),
            child: const Text(
              'Beli Sekarang',
              style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold),
            ),
          ),
        ),
      ),
    );
  }
}
