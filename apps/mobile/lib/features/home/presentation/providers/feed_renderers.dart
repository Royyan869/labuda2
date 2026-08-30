/// Feed Renderers & Navigation State for Home Module
///
/// CONTRACT HYGIENE PASS V1:
/// - This file contains PRODUCTION CODE for feed rendering
/// - Previously named "_stubs.dart" which was misleading
/// - Now honestly named "feed_renderers.dart"
///
/// CONTAINS:
/// - PendingTabSwitch: Navigation state for tab switching
/// - FeedCard: Canonical social-first feed card renderer
/// - FeedCardFactory: Factory for building feed cards
///
/// SEMANTIC RULES:
/// - Home feed shows ONLY universal social content and reposts
/// - Commerce objects (auction, collection) belong in Explore, NOT here
/// - Reposts MUST be clearly distinguished from original content
/// - "Ditutup" (closed) for social closure, NOT transaction completion
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/features/home/domain/domain.dart'; // R3.1: Import FeedItem from home domain
import 'package:go_router/go_router.dart';
import 'package:labuda/domains/social/content/domain/entities/content_resource_projection.dart';
import 'package:labuda/domains/social/content/presentation/widgets/content_resource_projection_card.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/domains/social/like/domain/entities/like.dart';
import 'package:labuda/domains/social/like/presentation/providers/like_notifier.dart';
import 'package:labuda/domains/social/share/share.dart';
import 'package:labuda/domains/user/identity/authentication/presentation/widgets/blocked_action_gate.dart';
import 'package:visibility_detector/visibility_detector.dart';

// Navigation state for pending tab switch (e.g., Home -> Explore with specific sub-tab)
class PendingTabSwitch {
  final String? target;
  final int? exploreSubTab;
  final bool hasSwitch;

  const PendingTabSwitch({
    this.target,
    this.exploreSubTab,
    this.hasSwitch = false,
  });
}

/// Notifier for pending tab switch state (Riverpod 2.x Notifier pattern)
/// Used for navigation between Home/Explore tabs with context preservation
class PendingTabSwitchNotifier extends Notifier<PendingTabSwitch> {
  @override
  PendingTabSwitch build() => const PendingTabSwitch();

  void setSwitch(String target, {int? subTab}) {
    state = PendingTabSwitch(
      target: target,
      exploreSubTab: subTab,
      hasSwitch: true,
    );
  }

  void clear() {
    state = const PendingTabSwitch();
  }
}

// Export tab switch provider as pendingTabSwitchProvider for explore module
final pendingTabSwitchProvider =
    NotifierProvider<PendingTabSwitchNotifier, PendingTabSwitch>(
      PendingTabSwitchNotifier.new,
    );

// FeedCardFactory - Build cards for feed items
class FeedCardFactory {
  static Widget buildCardForFeedItem(dynamic item, WidgetRef ref) {
    if (item is! FeedItem) {
      return const SizedBox.shrink();
    }

    // P3A — Dispatch promoted item types to their dedicated renderers.
    switch (item.type) {
      case FeedItemType.promotedListing:
        return PromotedListingCard(item: item);
      case FeedItemType.promotedAuction:
        return PromotedAuctionCard(item: item);
      case FeedItemType.promotedExternal:
        return PromotedExternalCard(item: item);
      case FeedItemType.content:
        return FeedCard(item: item);
    }
  }
}

/// FEED / DISCOVERY QUALITY PASS V1
///
/// Home Feed Card - Social-first, honest rendering
///
/// SEMANTIC RULES:
/// - Home feed shows ONLY universal social content and reposts
/// - Commerce objects (auction, collection) belong in Explore, NOT here
/// - Reposts MUST be clearly distinguished from original content
/// - No fake engagement counts (hide if not available)
///
/// NO FAKE LIVELINESS:
/// - Engagement counts hidden (backend doesn't provide them yet)
/// - No misleading "0 likes" when data simply isn't available
/// - Better to be simple but honest than rich but fake
class FeedCard extends ConsumerWidget {
  final FeedItem item;

  const FeedCard({super.key, required this.item});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    // QUALITY PASS: Extract repost data from additionalData
    final isRepost = item.additionalData['isRepost'] == true;
    final resourceProjection =
        item.additionalData['resourceProjection'] as ContentResourceProjection?;
    final originalAuthorId = item.additionalData['originalAuthorId'] as String?;

    // FIX-3: Use canonical enum-safe getter — no raw magic-string compare.

    // Governance lifecycle: TOMBSTONE drops from the list. UNAVAILABLE renders
    // muted with tap
    // disabled, signalled by `isUnavailable` through the existing renderers.
    if (item.lifecycle.isRemoved) {
      return const SizedBox.shrink();
    }
    final isUnavailable = item.lifecycle.isUnavailable;

    return Card(
      margin: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
      elevation: 0,
      color: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(12),
        side: BorderSide(
          color: isDark ? AppColors.darkGray700 : AppColors.neutralGray200,
        ),
      ),
      child: InkWell(
        onTap: isUnavailable ? null : () => _navigateToDetail(context),
        borderRadius: BorderRadius.circular(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // Governance unavailable banner sits above the content
            // affordances so the user sees "tidak tersedia" first.
            if (isUnavailable) _buildUnavailableBanner(context, isDark),
            // SHARE CONTRACT V1: Canonical RepostAttributionBar
            if (isRepost)
              RepostAttributionBar(
                originalAuthorId: originalAuthorId,
                originalAuthorName: _getOriginalAuthorName(),
                onTap: (!isUnavailable && resourceProjection != null)
                    ? () => _navigateToResource(context, resourceProjection)
                    : null,
              ),
            if (resourceProjection != null)
              Padding(
                padding: const EdgeInsets.fromLTRB(12, 0, 12, 8),
                child: ContentResourceProjectionCard(
                  resourceProjection: resourceProjection,
                  onTap: !isUnavailable
                      ? () => _navigateToResource(context, resourceProjection)
                      : null,
                ),
              ),
            // MEDIA INTEGRATION: Render media from MediaEntity
            if (item.media.isNotEmpty) _buildMediaImage(context),              // Content
              Padding(
                padding: const EdgeInsets.all(12),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    // Author info
                    _buildAuthorInfo(context, isDark),
                    const SizedBox(height: 8),
                    // Content text
                    _buildContentText(context, isDark),
                    const SizedBox(height: 8),
                    // Footer with Like, Comment, Share
                    _buildHonestFooter(context, ref, isDark),
                  ],
                ),
              ),
          ],
        ),
      ),
    );
  }

  /// QUALITY PASS: Get original author name from repost attribution.
  /// FIX-3 — Degrade gracefully when original author lifecycle is unavailable
  /// or removed. Returns a placeholder string so the RepostAttributionBar
  /// never shows a blank or stale username for a deleted/banned original author.
  String? _getOriginalAuthorName() {
    // FIX-3 — Lifecycle gate: degraded original author → placeholder.
    if (item.originalAuthorLifecycle.isDegraded) {
      return item.originalAuthorLifecycle.publicRedactionLabel;
    }
    return item.authorUsername;
  }

  /// QUALITY PASS: Navigate to canonical resource projection when tapped.
  void _navigateToResource(
    BuildContext context,
    ContentResourceProjection resourceProjection,
  ) {
    if (resourceProjection.isLive) {
      context.push(resourceProjection.canonicalPath);
    }
  }

  Widget _buildMediaImage(BuildContext context) {
    // MEDIA INTEGRATION: Use MediaEntity from canonical Content.media
    final imageUrl = item.media.first.originalUrl;
    return ClipRRect(
      borderRadius: const BorderRadius.vertical(top: Radius.circular(12)),
      child: Image.network(
        imageUrl,
        width: double.infinity,
        height: 200,
        fit: BoxFit.cover,
        errorBuilder: (context, error, stackTrace) => Container(
          width: double.infinity,
          height: 200,
          color: AppColors.neutralGray200,
          child: const Icon(
            Icons.image_not_supported,
            size: 48,
            color: AppColors.neutralGray400,
          ),
        ),
        loadingBuilder: (context, child, loadingProgress) {
          if (loadingProgress == null) return child;
          return Container(
            width: double.infinity,
            height: 200,
            color: AppColors.neutralGray100,
            child: const Center(
              child: CircularProgressIndicator(strokeWidth: 2),
            ),
          );
        },
      ),
    );
  }

  /// Governance UNAVAILABLE banner.
  /// Renders at the top of a card whose canonical lifecycle is `unavailable`.
  /// Tap is disabled at the InkWell, so the banner is the user-facing signal.
  Widget _buildUnavailableBanner(BuildContext context, bool isDark) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
      decoration: BoxDecoration(
        color: isDark
            ? AppColors.neutralGray800.withValues(alpha: 0.5)
            : AppColors.neutralGray100,
        border: Border(
          bottom: BorderSide(
            color: isDark ? AppColors.darkGray700 : AppColors.neutralGray200,
          ),
        ),
      ),
      child: Row(
        children: [
          Container(
            padding: const EdgeInsets.all(6),
            decoration: BoxDecoration(
              color: AppColors.neutralGray400.withValues(alpha: 0.2),
              shape: BoxShape.circle,
            ),
            child: const Icon(
              Icons.visibility_off_outlined,
              size: 16,
              color: AppColors.neutralGray500,
            ),
          ),
          const SizedBox(width: 8),
          Text(
            'Tidak tersedia',
            style: TextStyle(
              fontSize: 13,
              fontWeight: FontWeight.w600,
              color: isDark
                  ? AppColors.neutralGray300
                  : AppColors.neutralGray700,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildAuthorInfo(BuildContext context, bool isDark) {
    // FIX-3: Use canonical enum-safe getter — no raw magic-string compare.

    // E2.1 — Author lifecycle redaction. Independent from content
    // lifecycle: an active post by a suspended author still renders, with
    // the author block redacted and tap-to-profile disabled. Active /
    // null / unknown fall through to current behavior.
    final authorRedacted = item.authorLifecycle.isDegraded;
    final authorPlaceholder = _authorRedactionLabel(item.authorLifecycle);

    final authorMutedColor = isDark
        ? AppColors.neutralGray500
        : AppColors.neutralGray500;
    final authorNormalColor = isDark
        ? AppColors.neutralGray300
        : AppColors.neutralGray900;

    return InkWell(
      // Disable tap when the author identity is degraded — no profile
      // navigation off a tombstoned / suspended author block.
      onTap: authorRedacted ? null : () => _navigateToAuthorProfile(context),
      borderRadius: BorderRadius.circular(8),
      child: Row(
        children: [
          // Avatar — when the author is degraded, drop the network image and
          // fall back to the person icon. Never crash on null avatar; never
          // surface stale identity through a cached image.
          CircleAvatar(
            radius: 16,
            backgroundImage: (!authorRedacted && item.authorAvatarUrl != null)
                ? NetworkImage(item.authorAvatarUrl!)
                : null,
            child: (authorRedacted || item.authorAvatarUrl == null)
                ? Icon(
                    Icons.person,
                    size: 16,
                    color: authorRedacted ? AppColors.neutralGray400 : null,
                  )
                : null,
          ),
          const SizedBox(width: 8),
          // Public identity (OWNER TRUTH: username only).
          // E2.1 — When the author is redacted, emit a placeholder label
          // ("Pengguna tidak tersedia" / "Pengguna dihapus") instead of
          // the canonical handle. No fallback when both null and active.
          Expanded(
            child: Text(
              authorRedacted
                  ? authorPlaceholder
                  : (item.authorUsername != null
                        ? '@${item.authorUsername}'
                        : ''),
              style: TextStyle(
                fontWeight: FontWeight.w600,
                fontSize: 14,
                fontStyle: authorRedacted ? FontStyle.italic : FontStyle.normal,
                color: authorRedacted ? authorMutedColor : authorNormalColor,
              ),
            ),
          ),
          // Time
          Text(
            _formatTime(item.createdAt),
            style: TextStyle(
              fontSize: 12,
              color: AppColors.neutralGray500,
            ),
          ),
        ],
      ),
    );
  }

  /// Feed author redaction label.
  /// Delegates to the canonical [ContentLifecycleParse.publicRedactionLabel].
  String _authorRedactionLabel(ContentLifecycle authorLifecycle) =>
      authorLifecycle.publicRedactionLabel;

  /// Navigate to author's profile when author info is tapped
  void _navigateToAuthorProfile(BuildContext context) {
    if (item.authorId.isNotEmpty) {
      context.push('/user/${item.authorId}');
    }
  }

  Widget _buildContentText(BuildContext context, bool isDark) {
    // FIX-3: Use canonical enum-safe getter — no raw magic-string compare.

    return Text(
      item.content,
      maxLines: 3,
      overflow: TextOverflow.ellipsis,
      style: TextStyle(
        fontSize: 14,
        color: isDark ? AppColors.neutralGray300 : AppColors.neutralGray700,
      ),
    );
  }

  /// Footer with Like, Comment, and Share actions.
  /// Like uses live stats from the canonical Like domain.
  Widget _buildHonestFooter(BuildContext context, WidgetRef ref, bool isDark) {
    // Watch like stats for this content (only if authenticated)
    final authState = ref.watch(authControllerProvider);
    final currentUserId = authState is AuthStateAuthenticated
        ? authState.user.id
        : null;
    final currentUserName = authState is AuthStateAuthenticated
        ? authState.user.username
        : null;
    final isAuthenticated = currentUserId != null && currentUserId.isNotEmpty;

    final likeStatsAsync = isAuthenticated
        ? ref.watch(
            likeStatsProvider(
              LikeStatsParams(
                targetId: item.id,
                targetType: LikeTargetType.content,
                currentUserId: currentUserId,
              ),
            ),
          )
        : null;

    final stats = likeStatsAsync?.maybeWhen(
      data: (s) => s,
      orElse: () => null,
    );
    final likeCount = stats?.totalLikes ?? 0;
    final isLiked = stats?.isLikedByCurrentUser ?? false;

    return Row(
      children: [
        // Like action
        InkWell(
          onTap: isAuthenticated
              ? () => _handleLike(context, ref, currentUserId, currentUserName)
              : null,
          borderRadius: BorderRadius.circular(8),
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 4, vertical: 6),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(
                  isLiked ? Icons.favorite : Icons.favorite_border,
                  size: 16,
                  color: isLiked
                      ? AppColors.primaryRed
                      : (isAuthenticated
                            ? AppColors.primaryRed
                            : AppColors.neutralGray500),
                ),
                if (likeCount > 0) ...[
                  const SizedBox(width: 4),
                  Text(
                    '$likeCount',
                    style: TextStyle(
                      fontSize: 12,
                      color: isLiked
                          ? AppColors.primaryRed
                          : (isAuthenticated
                                ? AppColors.primaryRed
                                : AppColors.neutralGray500),
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                ],
              ],
            ),
          ),
        ),
        const SizedBox(width: 8),
        // Comment action
        InkWell(
          onTap: () => _navigateToComments(context),
          borderRadius: BorderRadius.circular(8),
          child: Row(
            children: [
              Icon(
                Icons.comment_outlined,
                size: 16,
                color: AppColors.primaryRed,
              ),
              const SizedBox(width: 4),
              Text(
                'Komentar',
                style: TextStyle(
                  fontSize: 12,
                  color: AppColors.primaryRed,
                  fontWeight: FontWeight.w500,
                ),
              ),
            ],
          ),
        ),
        const Spacer(),
        // Share action
        InkWell(
          onTap: () => _handleShareContent(context),
          borderRadius: BorderRadius.circular(8),
          child: Icon(
            Icons.share_outlined,
            size: 16,
            color: AppColors.neutralGray400,
          ),
        ),
      ],
    );
  }

  /// Handle like toggle for this content.
  void _handleLike(
    BuildContext context,
    WidgetRef ref,
    String currentUserId,
    String? currentUserName,
  ) async {
    final authState = ref.read(authControllerProvider);
    if (authState is AuthStateAuthenticated && !authState.emailVerified) {
      if (context.mounted) {
        await showBlockedActionGate(
          context,
          actionDescription: 'menyukai konten',
        );
      }
      return;
    }

    final notifier = ref.read(likeNotifierProvider.notifier);
    await notifier.toggleLike(
      targetId: item.id,
      targetType: LikeTargetType.content,
      userId: currentUserId,
      likerName: currentUserName ?? '',
      targetOwnerId: item.authorId,
    );
  }

  /// Handle share action - opens ShareBottomSheet for content
  void _handleShareContent(BuildContext context) {
    // MEDIA INTEGRATION: Use canonical media directly
    final imageUrl = item.media.isNotEmpty
        ? item.media.first.originalUrl
        : null;

    final shareTarget = ShareTarget(
      id: item.id,
      type: _mapFeedItemTypeToShareTarget(item.type),
      title: item.authorUsername != null ? '@${item.authorUsername}' : '',
      description: item.content,
      imageUrl: imageUrl,
      publicShareUrl: null, // Will use default public share URL generation
    );

    ShareBottomSheet.show(
      context: context,
      target: shareTarget,
      canSharePost: true,
    );
  }

  /// Map FeedItemType to ExternalShareType
  ///
  /// R3.1: Only social content types exist in FeedItemType
  /// Commerce types (auction, collection) are no longer in home feed
  ExternalShareType _mapFeedItemTypeToShareTarget(FeedItemType type) {
    switch (type) {
      case FeedItemType.content:
        return ExternalShareType.post;
      case FeedItemType.promotedListing:
      case FeedItemType.promotedAuction:
      case FeedItemType.promotedExternal:
        return ExternalShareType.post; // promoted items not shareable from feed
    }
  }

  /// Navigate to DiscussionScreen for this content
  void _navigateToComments(BuildContext context) {
    context.push('/comment/content/${item.id}');
  }

  String _formatTime(DateTime dateTime) {
    final now = DateTime.now();
    final difference = now.difference(dateTime);

    if (difference.inMinutes < 1) return 'baru saja';
    if (difference.inMinutes < 60) return '${difference.inMinutes}m';
    if (difference.inHours < 24) return '${difference.inHours}j';
    if (difference.inDays < 7) return '${difference.inDays}h';
    return '${dateTime.day}/${dateTime.month}/${dateTime.year}';
  }

  void _navigateToDetail(BuildContext context) {
    context.push('/content/${item.id}');
  }
}

// ============================================================================
// P3A — Promoted card widgets
// ============================================================================

// Fire-and-forget helper for promotion click analytics.
// Silently ignores errors so tracking never blocks navigation.
void _recordPromotionClick(WidgetRef ref, String instanceId, String surface) {
  if (instanceId.isEmpty) return;
  () async {
    try {
      await ref
          .read(apiClientProvider)
          .post(
            '/promotions/events',
            data: {
              'promotion_instance_id': instanceId,
              'event_type': 'click',
              'surface': surface,
            },
          );
    } catch (_) {}
  }();
}

// Session-level dedupe set for feed promotion impressions.
// Key: "instanceId:surface". Module-level — persists for the app session,
// resets on restart. No Riverpod state needed (no widget rebuilds required).
final _feedImpressionSeen = <String>{};

// Fire-and-forget impression helper — records at most once per instance per session.
// Fires when visibleFraction >= 0.5 (caller checks this).
// Errors are silently ignored so scroll performance is never affected.
void _recordPromotionImpression(
  WidgetRef ref,
  String instanceId,
  String surface,
) {
  if (instanceId.isEmpty) return;
  final key = '$instanceId:$surface';
  if (_feedImpressionSeen.contains(key)) return;
  _feedImpressionSeen.add(key);
  () async {
    try {
      await ref
          .read(apiClientProvider)
          .post(
            '/promotions/events',
            data: {
              'promotion_instance_id': instanceId,
              'event_type': 'impression',
              'surface': surface,
            },
          );
    } catch (_) {}
  }();
}

/// Badge shown on all promoted feed items.
class _PromotedBadge extends StatelessWidget {
  const _PromotedBadge();

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
      decoration: BoxDecoration(
        color: AppColors.primaryRed.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(4),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(Icons.campaign_outlined, size: 14, color: AppColors.primaryRed),
          const SizedBox(width: 4),
          Text(
            'Dipromosikan',
            style: TextStyle(
              color: AppColors.primaryRed,
              fontSize: 11,
              fontWeight: FontWeight.w600,
            ),
          ),
        ],
      ),
    );
  }
}

/// Formats a price in IDR minor units to a display string.
String _formatPrice(int? priceMinor) {
  if (priceMinor == null) return '-';
  final rupiah = priceMinor ~/ 100;
  if (rupiah >= 1000000) {
    return 'Rp${(rupiah / 1000000).toStringAsFixed(1)}jt';
  }
  if (rupiah >= 1000) {
    return 'Rp${(rupiah / 1000).toStringAsFixed(0)}rb';
  }
  return 'Rp$rupiah';
}

/// Promoted listing card — shows listing image, title, price, seller.
class PromotedListingCard extends ConsumerWidget {
  final FeedItem item;
  const PromotedListingCard({super.key, required this.item});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final data = item.additionalData;
    final imageUrl = data['imageUrl'] as String?;
    final title = data['title'] as String? ?? '';
    final pricePerUnit = data['pricePerUnit'] as int?;
    final sellerUsername = data['sellerUsername'] as String?;
    final sellerFarmName = data['sellerFarmName'] as String?;
    final sellerLabel = _formatSellerLabel(sellerUsername, sellerFarmName);
    final forSaleId = data['forSaleId'] as String?;
    final promotionInstanceId = data['promotionInstanceId'] as String? ?? '';

    return VisibilityDetector(
      key: Key('promo_imp_${promotionInstanceId}_feed_listing'),
      onVisibilityChanged: (info) {
        if (info.visibleFraction >= 0.5) {
          _recordPromotionImpression(ref, promotionInstanceId, 'feed');
        }
      },
      child: Card(
        margin: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
        elevation: 0,
        color: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(12),
          side: BorderSide(
            color: isDark ? AppColors.darkGray700 : AppColors.neutralGray200,
          ),
        ),
        child: InkWell(
          onTap: forSaleId != null
              ? () {
                  _recordPromotionClick(ref, promotionInstanceId, 'feed');
                  context.push(
                    RoutePaths.forSaleDetail.replaceFirst(
                      ':forSaleId',
                      forSaleId,
                    ),
                  );
                }
              : null,
          borderRadius: BorderRadius.circular(12),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              if (imageUrl != null && imageUrl.isNotEmpty)
                ClipRRect(
                  borderRadius: const BorderRadius.vertical(
                    top: Radius.circular(12),
                  ),
                  child: Image.network(
                    imageUrl,
                    width: double.infinity,
                    height: 180,
                    fit: BoxFit.cover,
                    errorBuilder: (_, _, _) => Container(
                      width: double.infinity,
                      height: 180,
                      color: AppColors.neutralGray200,
                      child: const Icon(
                        Icons.image_not_supported,
                        size: 48,
                        color: AppColors.neutralGray400,
                      ),
                    ),
                  ),
                ),
              Padding(
                padding: const EdgeInsets.all(12),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    const _PromotedBadge(),
                    const SizedBox(height: 8),
                    Text(
                      title,
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(
                        fontSize: 15,
                        fontWeight: FontWeight.w600,
                        color: isDark
                            ? AppColors.neutralWhite
                            : AppColors.neutralGray900,
                      ),
                    ),
                    const SizedBox(height: 4),
                    Text(
                      _formatPrice(pricePerUnit),
                      style: TextStyle(
                        fontSize: 16,
                        fontWeight: FontWeight.w700,
                        color: AppColors.primaryRed,
                      ),
                    ),
                    const SizedBox(height: 6),
                    Row(
                      children: [
                        Icon(
                          Icons.storefront_outlined,
                          size: 14,
                          color: AppColors.neutralGray500,
                        ),
                        const SizedBox(width: 4),
                        Text(
                          sellerLabel,
                          style: TextStyle(
                            fontSize: 13,
                            color: AppColors.neutralGray500,
                          ),
                        ),
                      ],
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// Promoted auction card — shows auction image, title, bid info, timer, seller.
class PromotedAuctionCard extends ConsumerWidget {
  final FeedItem item;
  const PromotedAuctionCard({super.key, required this.item});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final data = item.additionalData;
    final imageUrl = data['imageUrl'] as String?;
    final title = data['title'] as String? ?? '';
    final startPrice = data['startPrice'] as int?;
    final currentBid = data['currentBid'] as int?;
    final bidCount = data['bidCount'] as int? ?? 0;
    final endAtStr = data['endAt'] as String?;
    final sellerUsername = data['sellerUsername'] as String?;
    final sellerFarmName = data['sellerFarmName'] as String?;
    final sellerLabel = _formatSellerLabel(sellerUsername, sellerFarmName);
    final auctionId = data['auctionId'] as String?;
    final promotionInstanceId = data['promotionInstanceId'] as String? ?? '';

    final displayPrice = currentBid ?? startPrice;
    final priceLabel = currentBid != null ? 'Bid saat ini' : 'Mulai dari';

    String timeRemaining = '';
    if (endAtStr != null) {
      final endAt = DateTime.tryParse(endAtStr);
      if (endAt != null) {
        final diff = endAt.difference(DateTime.now());
        if (diff.isNegative) {
          timeRemaining = 'Berakhir';
        } else if (diff.inDays > 0) {
          timeRemaining = '${diff.inDays}h tersisa';
        } else if (diff.inHours > 0) {
          timeRemaining = '${diff.inHours}j tersisa';
        } else {
          timeRemaining = '${diff.inMinutes}m tersisa';
        }
      }
    }

    return VisibilityDetector(
      key: Key('promo_imp_${promotionInstanceId}_feed_auction'),
      onVisibilityChanged: (info) {
        if (info.visibleFraction >= 0.5) {
          _recordPromotionImpression(ref, promotionInstanceId, 'feed');
        }
      },
      child: Card(
        margin: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
        elevation: 0,
        color: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(12),
          side: BorderSide(
            color: isDark ? AppColors.darkGray700 : AppColors.neutralGray200,
          ),
        ),
        child: InkWell(
          onTap: auctionId != null
              ? () {
                  _recordPromotionClick(ref, promotionInstanceId, 'feed');
                  context.push('/auction/$auctionId');
                }
              : null,
          borderRadius: BorderRadius.circular(12),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              if (imageUrl != null && imageUrl.isNotEmpty)
                ClipRRect(
                  borderRadius: const BorderRadius.vertical(
                    top: Radius.circular(12),
                  ),
                  child: Image.network(
                    imageUrl,
                    width: double.infinity,
                    height: 180,
                    fit: BoxFit.cover,
                    errorBuilder: (_, _, _) => Container(
                      width: double.infinity,
                      height: 180,
                      color: AppColors.neutralGray200,
                      child: const Icon(
                        Icons.image_not_supported,
                        size: 48,
                        color: AppColors.neutralGray400,
                      ),
                    ),
                  ),
                ),
              Padding(
                padding: const EdgeInsets.all(12),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        const _PromotedBadge(),
                        const Spacer(),
                        if (timeRemaining.isNotEmpty)
                          Container(
                            padding: const EdgeInsets.symmetric(
                              horizontal: 8,
                              vertical: 3,
                            ),
                            decoration: BoxDecoration(
                              color: Colors.orange.withValues(alpha: 0.1),
                              borderRadius: BorderRadius.circular(4),
                            ),
                            child: Text(
                              timeRemaining,
                              style: const TextStyle(
                                color: Colors.orange,
                                fontSize: 11,
                                fontWeight: FontWeight.w600,
                              ),
                            ),
                          ),
                      ],
                    ),
                    const SizedBox(height: 8),
                    Text(
                      title,
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(
                        fontSize: 15,
                        fontWeight: FontWeight.w600,
                        color: isDark
                            ? AppColors.neutralWhite
                            : AppColors.neutralGray900,
                      ),
                    ),
                    const SizedBox(height: 4),
                    Text(
                      priceLabel,
                      style: TextStyle(
                        fontSize: 12,
                        color: AppColors.neutralGray500,
                      ),
                    ),
                    Text(
                      _formatPrice(displayPrice),
                      style: TextStyle(
                        fontSize: 16,
                        fontWeight: FontWeight.w700,
                        color: AppColors.primaryRed,
                      ),
                    ),
                    const SizedBox(height: 6),
                    Row(
                      children: [
                        Icon(
                          Icons.storefront_outlined,
                          size: 14,
                          color: AppColors.neutralGray500,
                        ),
                        const SizedBox(width: 4),
                        Expanded(
                          child: Text(
                            sellerLabel,
                            style: TextStyle(
                              fontSize: 13,
                              color: AppColors.neutralGray500,
                            ),
                          ),
                        ),
                        if (bidCount > 0)
                          Text(
                            '$bidCount bid',
                            style: TextStyle(
                              fontSize: 12,
                              color: AppColors.neutralGray400,
                            ),
                          ),
                      ],
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

String _formatSellerLabel(String? sellerUsername, String? sellerFarmName) {
  final username = (sellerUsername ?? '').trim();
  final farmName = (sellerFarmName ?? '').trim();
  if (username.isNotEmpty && farmName.isNotEmpty) {
    return '@$username • $farmName';
  }
  if (username.isNotEmpty) {
    return '@$username';
  }
  return '';
}

/// Promoted external product card — shows title, media, and opens external URL.
class PromotedExternalCard extends ConsumerWidget {
  final FeedItem item;
  const PromotedExternalCard({super.key, required this.item});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final data = item.additionalData;
    final title = data['title'] as String? ?? '';
    final externalUrl = data['externalUrl'] as String?;
    final externalMediaUrl = data['externalMediaUrl'] as String?;
    final promotionInstanceId = data['promotionInstanceId'] as String? ?? '';

    return VisibilityDetector(
      key: Key('promo_imp_${promotionInstanceId}_feed_external'),
      onVisibilityChanged: (info) {
        if (info.visibleFraction >= 0.5) {
          _recordPromotionImpression(ref, promotionInstanceId, 'feed');
        }
      },
      child: Card(
        margin: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
        elevation: 0,
        color: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(12),
          side: BorderSide(
            color: isDark ? AppColors.darkGray700 : AppColors.neutralGray200,
          ),
        ),
        child: InkWell(
          onTap: externalUrl != null
              ? () {
                  _recordPromotionClick(ref, promotionInstanceId, 'feed');
                  showExternalLinkInterstitial(context, url: externalUrl);
                }
              : null,
          borderRadius: BorderRadius.circular(12),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              if (externalMediaUrl != null && externalMediaUrl.isNotEmpty)
                ClipRRect(
                  borderRadius: const BorderRadius.vertical(
                    top: Radius.circular(12),
                  ),
                  child: Image.network(
                    externalMediaUrl,
                    width: double.infinity,
                    height: 180,
                    fit: BoxFit.cover,
                    errorBuilder: (_, _, _) => const SizedBox.shrink(),
                  ),
                ),
              Padding(
                padding: const EdgeInsets.all(12),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    const _PromotedBadge(),
                    const SizedBox(height: 8),
                    Text(
                      title,
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(
                        fontSize: 15,
                        fontWeight: FontWeight.w600,
                        color: isDark
                            ? AppColors.neutralWhite
                            : AppColors.neutralGray900,
                      ),
                    ),
                    if (externalUrl != null) ...[
                      const SizedBox(height: 6),
                      Row(
                        children: [
                          Icon(
                            Icons.open_in_new,
                            size: 14,
                            color: AppColors.neutralGray400,
                          ),
                          const SizedBox(width: 4),
                          Expanded(
                            child: Text(
                              Uri.tryParse(externalUrl)?.host ?? externalUrl,
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                              style: TextStyle(
                                fontSize: 12,
                                color: AppColors.neutralGray400,
                              ),
                            ),
                          ),
                        ],
                      ),
                    ],
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

// Global feed refresh callback stub
VoidCallback? _globalFeedRefreshCallback;

void setGlobalFeedRefreshCallback(VoidCallback callback) {
  _globalFeedRefreshCallback = callback;
}

void refreshFeedGlobally() {
  _globalFeedRefreshCallback?.call();
}
