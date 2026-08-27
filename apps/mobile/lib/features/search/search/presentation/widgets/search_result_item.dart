import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/features/search/search/domain/entities/search_result.dart';
import 'package:labuda/features/search/search/presentation/utils/search_result_type_helper.dart';
import 'package:labuda/features/search/search/presentation/widgets/search_result_extra_info.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/governance/seller_inactive_badge.dart';
import 'package:labuda/shared/widgets/promoted_badge.dart';
import 'package:labuda/shared/widgets/follow_button.dart';

/// Widget to display a single search result item
class SearchResultItem extends ConsumerWidget {
  final SearchResult result;
  final VoidCallback? onTap;

  const SearchResultItem({super.key, required this.result, this.onTap});

  /// Canonical governance lifecycle for content rows. Listing/auction/user
  /// rows always render `active` (their lifecycle adoption is a separate
  /// downstream batch — see B5.1 rollout plan).
  ContentLifecycle get _lifecycle => result.type == SearchResultType.content
      ? ContentLifecycleParse.fromWire(result.metadata['lifecycle'] as String?)
      : ContentLifecycle.active;

  /// E9.1 — Content author user-identity lifecycle. Read only from
  /// `metadata['authorLifecycle']` which the adapter sources ONLY from the
  /// `card.author.lifecycle` wire slot on content rows. Any other row type
  /// stays `active` so this branch never fires on listing/auction/user rows.
  /// AXIS BOUNDARY: independent from `metadata['lifecycle']` (item-axis).
  ContentLifecycle get _contentAuthorLifecycle {
    if (result.type != SearchResultType.content) return ContentLifecycle.active;
    return ContentLifecycleParse.fromWire(
      result.metadata['authorLifecycle'] as String?,
    );
  }

  /// Subtitle redaction placeholder when content author identity is degraded.
  String? get _contentAuthorRedactionSubtitle =>
      _contentAuthorLifecycle.isActive
      ? null
      : _contentAuthorLifecycle.publicRedactionLabel;

  /// E8.4 — Seller user-axis lifecycle for listing/auction rows. Read only
  /// from `metadata['sellerLifecycle']` which the adapter sources ONLY from
  /// the nested `seller.user.lifecycle` wire slot. Top-level
  /// `seller.lifecycle` (seller-trust axis) is never consumed. Any other
  /// row type stays `active` so the seller-axis branch never fires.
  ContentLifecycle get _sellerUserLifecycle {
    final isSellerSurface =
        result.type == SearchResultType.listing ||
        result.type == SearchResultType.auction;
    if (!isSellerSurface) return ContentLifecycle.active;
    return ContentLifecycleParse.fromWire(
      result.metadata['sellerLifecycle'] as String?,
    );
  }

  /// Subtitle redaction placeholder when seller user-identity is degraded.
  /// Vocabulary delegated to [ContentLifecycleParse.publicRedactionLabel].
  /// Item row remains visible and tappable — this swaps only the seller-
  /// identifying subtitle string.
  String? get _sellerRedactionSubtitle => _sellerUserLifecycle.isActive
      ? null
      : _sellerUserLifecycle.publicRedactionLabel;

  /// Seller-trust axis lifecycle for listing/auction rows (subscription
  /// expired/lapsed). Read from `metadata['sellerTrustLifecycle']`.
  /// Any other row type stays active so this branch never fires.
  ContentLifecycle get _sellerTrustLifecycle {
    final isSellerSurface =
        result.type == SearchResultType.listing ||
        result.type == SearchResultType.auction;
    if (!isSellerSurface) return ContentLifecycle.active;
    return ContentLifecycleParse.fromWire(
      result.metadata['sellerTrustLifecycle'] as String?,
    );
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final isUnavailable = _lifecycle.isUnavailable;
    return Opacity(
      opacity: isUnavailable ? 0.55 : 1.0,
      child: InkWell(
        onTap: isUnavailable ? null : onTap,
        borderRadius: BorderRadius.circular(12),
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
          child: Row(
            children: [
              _buildImage(context),
              const SizedBox(width: 12),
              Expanded(
                child: _buildContent(context, isUnavailable: isUnavailable),
              ),
              // Show FollowButton for user results, otherwise show type indicator
              if (result.type == SearchResultType.user)
                _buildFollowButton(context, ref)
              else
                _buildTypeIndicator(context),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildImage(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return ClipRRect(
      borderRadius: BorderRadius.circular(
        result.type == SearchResultType.user ? 24 : 8,
      ),
      child: Container(
        width: 48,
        height: 48,
        color: isDark ? AppColors.darkGray700 : AppColors.neutralGray200,
        child: result.imageUrl != null
            ? Image.network(
                result.imageUrl!,
                fit: BoxFit.cover,
                errorBuilder: (_, _, _) => _buildPlaceholder(),
              )
            : _buildPlaceholder(),
      ),
    );
  }

  Widget _buildPlaceholder() {
    return Center(
      child: Icon(
        SearchResultTypeHelper.getIcon(result.type),
        color: AppColors.neutralGray400,
        size: 24,
      ),
    );
  }

  Widget _buildContent(BuildContext context, {required bool isUnavailable}) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final extraInfo = SearchResultExtraInfo(result: result);
    final sellerSurface =
        result.type == SearchResultType.listing ||
        result.type == SearchResultType.auction;
    final subtitleMaxLines = sellerSurface ? 2 : 1;
    // E8.4 — seller user-axis subtitle redaction (listing/auction only).
    // E9.1 — content author user-axis subtitle redaction (content only).
    // Both lose to item-axis `isUnavailable` (content tombstone styling).
    // sellerRedactionSubtitle and contentAuthorRedactionSubtitle are
    // mutually exclusive — different row types can never both fire.
    final sellerRedactionSubtitle = _sellerRedactionSubtitle;
    final contentAuthorRedactionSubtitle = _contentAuthorRedactionSubtitle;
    final showSellerInactiveBadge = shouldShowSellerInactiveBadge(
      sellerTrustLifecycle: _sellerTrustLifecycle,
      sellerUserLifecycle: _sellerUserLifecycle,
    );

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        Row(
          children: [
            Expanded(
              child: Text(
                result.title,
                style: TextStyle(
                  fontWeight: FontWeight.w600,
                  color: isDark
                      ? AppColors.neutralGray100
                      : AppColors.neutralGray900,
                ),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
              ),
            ),
            // PROMOTION PHASE 4: Show "Promoted" badge for promoted items
            if (result.isPromoted) ...[
              const SizedBox(width: 6),
              PromotedBadge.chip(),
            ],
          ],
        ),
        if (isUnavailable) ...[
          const SizedBox(height: 2),
          Text(
            'Tidak tersedia',
            style: TextStyle(
              fontSize: 12,
              fontWeight: FontWeight.w600,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray500,
            ),
            maxLines: subtitleMaxLines,
            overflow: TextOverflow.ellipsis,
          ),
        ] else if (sellerRedactionSubtitle != null) ...[
          const SizedBox(height: 2),
          Text(
            sellerRedactionSubtitle,
            style: TextStyle(
              fontSize: 13,
              fontStyle: FontStyle.italic,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray500,
            ),
            maxLines: subtitleMaxLines,
            overflow: TextOverflow.ellipsis,
          ),
        ] else if (contentAuthorRedactionSubtitle != null) ...[
          const SizedBox(height: 2),
          Text(
            contentAuthorRedactionSubtitle,
            style: TextStyle(
              fontSize: 13,
              fontStyle: FontStyle.italic,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray500,
            ),
            maxLines: subtitleMaxLines,
            overflow: TextOverflow.ellipsis,
          ),
        ] else if (result.subtitle != null) ...[
          const SizedBox(height: 2),
          Text(
            result.subtitle!,
            style: TextStyle(
              fontSize: 13,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray500,
            ),
            maxLines: subtitleMaxLines,
            overflow: TextOverflow.ellipsis,
          ),
        ],
        if (extraInfo.hasExtraInfo) ...[const SizedBox(height: 4), extraInfo],
        if (showSellerInactiveBadge) ...[
          const SizedBox(height: 4),
          const SellerInactiveBadge(),
        ],
      ],
    );
  }

  Widget _buildTypeIndicator(BuildContext context) {
    final color = SearchResultTypeHelper.getColor(result.type);
    final label = SearchResultTypeHelper.getLabel(result.type);

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.15),
        borderRadius: BorderRadius.circular(12),
      ),
      child: Text(
        label,
        style: TextStyle(
          fontSize: 11,
          fontWeight: FontWeight.w600,
          color: color,
        ),
      ),
    );
  }

  /// Build FollowButton for user search results
  ///
  /// Only shows for user type results. Uses the result.id as userId.
  Widget _buildFollowButton(BuildContext context, WidgetRef ref) {
    // For user results, result.id is the userId
    return FollowButton(
      userId: result.id,
      buttonSize: 32,
      iconSize: 14,
      fontSize: 12,
    );
  }
}
