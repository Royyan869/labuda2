import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/rating/domain/entities/rating_entity.dart';
import 'package:labuda/domains/system/shared/domain/services/time_format_service.dart';

/// CANONICAL Rating Card Widget
///
/// Displays a rating using only canonical fields:
/// - orderId, buyerId, sellerId, ratingValue, comment, createdAt
///
/// Business Truth (LOCKED):
/// - Rating is IMMUTABLE (no edit/delete)
/// - Rating direction is BUYER → SELLER ONLY
/// - No helpful voting, media, or criteria scores
class RatingCard extends StatelessWidget {
  final Rating rating;
  final VoidCallback? onTap;
  final String? buyerName;
  final String? buyerAvatar;

  const RatingCard({
    super.key,
    required this.rating,
    this.onTap,
    this.buyerName,
    this.buyerAvatar,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;

    return Card(
      margin: const EdgeInsets.symmetric(
        horizontal: AppMetrics.p16,
        vertical: AppMetrics.p8,
      ),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(AppShape.r12),
        child: Padding(
          padding: const EdgeInsets.all(AppMetrics.p16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              _buildHeader(theme, colorScheme),
              const SizedBox(height: 12),
              _buildRatingStars(context),
              if (rating.comment != null && rating.comment!.isNotEmpty) ...[
                const SizedBox(height: 8),
                _buildCommentText(theme),
              ],
              const SizedBox(height: 8),
              _buildFooter(theme, colorScheme),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildHeader(ThemeData theme, ColorScheme colorScheme) {
    return Row(
      children: [
        CircleAvatar(
          radius: 20,
          backgroundColor: colorScheme.primaryContainer,
          child: Icon(Icons.person, color: colorScheme.onPrimaryContainer),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                buyerName ?? 'Buyer ${rating.buyerId.substring(0, 8)}',
                style: theme.textTheme.titleSmall?.copyWith(
                  fontWeight: FontWeight.w600,
                ),
              ),
              Text(
                'Verified Purchase',
                style: theme.textTheme.bodySmall?.copyWith(
                  color: colorScheme.onSurfaceVariant,
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildRatingStars(BuildContext context) {
    return Row(
      children: [
        ...List.generate(5, (index) {
          return Icon(
            index < rating.ratingValue ? Icons.star : Icons.star_border,
            color: AppColors.koiGold,
            size: AppIconSize.action,
          );
        }),
        const SizedBox(width: 8),
        Text(
          '${rating.ratingValue}/5',
          style: context.typeRoles.bodyDense.copyWith(
            fontWeight: FontWeight.w600,
          ),
        ),
      ],
    );
  }

  Widget _buildCommentText(ThemeData theme) {
    return Text(
      rating.comment!,
      style: theme.textTheme.bodyMedium,
      maxLines: 3,
      overflow: TextOverflow.ellipsis,
    );
  }

  Widget _buildFooter(ThemeData theme, ColorScheme colorScheme) {
    return Row(
      children: [
        Flexible(
          child: Text(
            const TimeFormatService().formatTimeAgo(rating.createdAt),
            style: theme.textTheme.bodySmall?.copyWith(
              color: colorScheme.onSurfaceVariant,
            ),
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
          ),
        ),
        const Spacer(),
        IconButton(
          onPressed: null, // Ratings are immutable
          icon: Icon(
            Icons.more_vert,
            size: AppIconSize.inlineGlyph,
            color: colorScheme.onSurfaceVariant,
           semanticLabel: 'Opsi',
           ),
        ),
      ],
    );
  }
}
