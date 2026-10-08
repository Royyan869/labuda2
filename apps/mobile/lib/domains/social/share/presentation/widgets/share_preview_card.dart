import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/widgets/address_location_view.dart';
import 'package:labuda/shared/widgets/app_image.dart';
import 'package:labuda/shared/domain/entities/resource_projection.dart';
import '../../domain/entities/share_target.dart';

/// Preview card showing what content will be shared
class SharePreviewCard extends StatelessWidget {
  final ShareTarget target;

  const SharePreviewCard({
    super.key,
    required this.target,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final cardColor = scheme.surfaceContainerHigh;
    final borderColor = scheme.outlineVariant;
    final textColor = scheme.onSurface;
    final secondaryTextColor = scheme.onSurfaceVariant;
    final placeholderColor = scheme.surfaceContainerHighest;

    return Container(
      margin: const EdgeInsets.all(AppMetrics.p16),
      padding: const EdgeInsets.all(AppMetrics.p12),
      decoration: BoxDecoration(
        color: cardColor,
        borderRadius: BorderRadius.circular(AppShape.r12),
        border: Border.all(color: borderColor, width: 1),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          // Image preview - SQUARE (if exists)
          if (target.imageUrl != null) ...[
            ClipRRect(
              borderRadius: BorderRadius.circular(AppShape.r8),
              child: AspectRatio(
                aspectRatio: 1.0, // Square image
                child: AppImage(
                  imageUrl: target.imageUrl,
                  width: double.infinity,
                  fit: BoxFit.cover,
                  backgroundColor: placeholderColor,
                  errorWidget: Container(
                    color: placeholderColor,
                    child: Icon(
                      Icons.broken_image,
                      size: AppIconSize.display,
                      color: secondaryTextColor,
                    ),
                  ),
                ),
              ),
            ),
            const SizedBox(height: 12),
          ],

          // Type badge
          Row(
            children: [
              Icon(
                _getTypeIcon(),
                size: AppIconSize.inlineGlyph,
                color: secondaryTextColor,
              ),
              const SizedBox(width: 4),
              Text(
                _getTypeLabel(),
                style: Theme.of(context).textTheme.bodySmall?.copyWith(
                  color: secondaryTextColor,
                  fontWeight: FontWeight.w500,
                ),
              ),
            ],
          ),
          const SizedBox(height: 8),

          // Title
          Text(
            target.title,
            style: Theme.of(context).textTheme.bodyLarge?.copyWith(
              fontWeight: FontWeight.w600,
              color: textColor,
            ),
            maxLines: 2,
            overflow: TextOverflow.ellipsis,
          ),
          const SizedBox(height: 8),

          // Metadata display berdasarkan content type
          _buildMetadataSection(context, textColor, secondaryTextColor),
        ],
      ),
    );
  }

  /// Build metadata section based on content type
  Widget _buildMetadataSection(
    BuildContext context,
    Color textColor,
    Color secondaryTextColor,
  ) {
    switch (target.type) {
      case ExternalShareType.forSale:
        return _buildForSaleMetadata(context, textColor, secondaryTextColor);
      case ExternalShareType.auction:
        return _buildAuctionMetadata(context, textColor, secondaryTextColor);
      case ExternalShareType.request:
        return _buildContentMetadata(context, textColor, secondaryTextColor);
      case ExternalShareType.post:
      case ExternalShareType.profile:
        return _buildDefaultMetadata(context, secondaryTextColor);
    }
  }

  /// For Sale metadata - compact dengan info variety & size
  Widget _buildForSaleMetadata(
    BuildContext context,
    Color textColor,
    Color secondaryTextColor,
  ) {
    final variety = target.metadata['variety'] as String?;
    final size = target.metadata['size'] as num?;
    final location = target.metadata['location'] as String?;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        // Row 1: Variety & Size
        if (variety != null || size != null)
          Row(
            children: [
              if (variety != null) ...[
                Icon(Icons.category, size: AppIconSize.inlineGlyph, color: secondaryTextColor),
                const SizedBox(width: 4),
                Flexible(
                  child: Text(
                    variety,
                    style: Theme.of(context).textTheme.bodySmall?.copyWith(
                      color: secondaryTextColor,
                    ),
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
              ],
              if (variety != null && size != null) const SizedBox(width: 12),
              if (size != null) ...[
                Icon(Icons.straighten, size: AppIconSize.inlineGlyph, color: secondaryTextColor),
                const SizedBox(width: 4),
                Text(
                  '${size.toStringAsFixed(0)} cm',
                  style: Theme.of(context).textTheme.bodySmall?.copyWith(
                    color: secondaryTextColor,
                  ),
                ),
              ],
            ],
          ),

        // Row 2: Location
        if (location != null) ...[
          const SizedBox(height: 4),
          AddressLocationView(
            location: location,
            mode: AddressLocationMode.compact,
            icon: Icons.location_on,
            iconSize: AppIconSize.inlineGlyph,
            spacing: 4,
            style: Theme.of(context).textTheme.bodySmall?.copyWith(
              color: secondaryTextColor,
            ),
          ),
        ],
      ],
    );
  }

  /// Auction metadata - show current bid & time remaining
  Widget _buildAuctionMetadata(
    BuildContext context,
    Color textColor,
    Color secondaryTextColor,
  ) {
    final scheme = Theme.of(context).colorScheme;
    final currentBid = target.metadata['currentBid'] as num?;
    final endTimeStr = target.metadata['endTime'] as String?;
    final variety = target.metadata['variety'] as String?;
    final size = target.metadata['size'] as num?;

    // Parse time remaining
    String? timeRemaining;
    bool isUrgent = false;
    if (endTimeStr != null) {
      final endTime = DateTime.parse(endTimeStr);
      final remaining = endTime.difference(DateTime.now());
      if (remaining.isNegative) {
        timeRemaining = 'Berakhir';
      } else {
        final days = remaining.inDays;
        final hours = remaining.inHours.remainder(24);
        final minutes = remaining.inMinutes.remainder(60);
        timeRemaining = days > 0
            ? '${days}d ${hours}h ${minutes}m'
            : '${hours}h ${minutes}m';
        isUrgent = remaining.inHours < 24; // Red if < 24 hours
      }
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        // Current Bid - Prominent
        if (currentBid != null) ...[
          Row(
            children: [
              Icon(Icons.gavel, size: AppIconSize.inlineGlyph, color: scheme.primary),
              const SizedBox(width: 4),
              Text(
                'KB: ${_formatPrice(currentBid)}',
                style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                  color: scheme.primary,
                  fontWeight: FontWeight.w700,
                ),
              ),
            ],
          ),
          const SizedBox(height: 8),
        ],

        // Row 1: Variety & Size
        if (variety != null || size != null)
          Row(
            children: [
              if (variety != null) ...[
                Icon(Icons.category, size: AppIconSize.inlineGlyph, color: secondaryTextColor),
                const SizedBox(width: 4),
                Flexible(
                  child: Text(
                    variety,
                    style: Theme.of(context).textTheme.bodySmall?.copyWith(
                      color: secondaryTextColor,
                    ),
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
              ],
              if (variety != null && size != null) const SizedBox(width: 12),
              if (size != null) ...[
                Icon(Icons.straighten, size: AppIconSize.inlineGlyph, color: secondaryTextColor),
                const SizedBox(width: 4),
                Text(
                  '${size.toStringAsFixed(0)} cm',
                  style: Theme.of(context).textTheme.bodySmall?.copyWith(
                    color: secondaryTextColor,
                  ),
                ),
              ],
            ],
          ),

        // Row 2: Time Remaining with urgency indicator
        if (timeRemaining != null) ...[
          const SizedBox(height: 4),
          Row(
            children: [
              Icon(
                Icons.access_time,
                size: AppIconSize.inlineGlyph,
                color: isUrgent ? context.statusColors.error : secondaryTextColor,
              ),
              const SizedBox(width: 4),
              Text(
                timeRemaining,
                style: Theme.of(context).textTheme.bodySmall?.copyWith(
                  color: isUrgent ? context.statusColors.error : secondaryTextColor,
                  fontWeight: isUrgent ? FontWeight.w600 : FontWeight.normal,
                ),
              ),
            ],
          ),
        ],
      ],
    );
  }

  /// Content metadata - show budget/variety when available
  Widget _buildContentMetadata(
    BuildContext context,
    Color textColor,
    Color secondaryTextColor,
  ) {
    final scheme = Theme.of(context).colorScheme;
    final budget = target.metadata['budget'] as num?;
    final maxBudget = target.metadata['maxBudget'] as num?;
    final location = target.metadata['location'] as String?;
    final variety = target.metadata['variety'] as String?;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        // Budget - Prominent
        if (budget != null || maxBudget != null) ...[
          Row(
            children: [
              Icon(
                Icons.account_balance_wallet,
                size: AppIconSize.inlineGlyph,
                color: scheme.secondary,
              ),
              const SizedBox(width: 4),
              Text(
                maxBudget != null
                    ? 'Budget: ${_formatPrice(maxBudget)}'
                    : 'Budget: ${_formatPrice(budget!)}',
                style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                  color: scheme.secondary,
                  fontWeight: FontWeight.w700,
                ),
              ),
            ],
          ),
          const SizedBox(height: 8),
        ],

        // Variety & Location
        if (variety != null)
          Row(
            children: [
              Icon(Icons.category, size: AppIconSize.inlineGlyph, color: secondaryTextColor),
              const SizedBox(width: 4),
              Flexible(
                child: Text(
                  variety,
                  style: Theme.of(context).textTheme.bodySmall?.copyWith(
                    color: secondaryTextColor,
                  ),
                  overflow: TextOverflow.ellipsis,
                ),
              ),
            ],
          ),

        if (location != null) ...[
          const SizedBox(height: 4),
          AddressLocationView(
            location: location,
            mode: AddressLocationMode.compact,
            icon: Icons.location_on,
            iconSize: AppIconSize.inlineGlyph,
            spacing: 4,
            style: Theme.of(context).textTheme.bodySmall?.copyWith(
              color: secondaryTextColor,
            ),
          ),
        ],
      ],
    );
  }

  /// Default metadata - show description
  Widget _buildDefaultMetadata(BuildContext context, Color secondaryTextColor) {
    if (target.description.isEmpty) return const SizedBox.shrink();

    return Text(
      target.description,
      style: Theme.of(context).textTheme.bodyMedium?.copyWith(color: secondaryTextColor),
      maxLines: 2,
      overflow: TextOverflow.ellipsis,
    );
  }

  IconData _getTypeIcon() {
    switch (target.type) {
      case ExternalShareType.post:
        return Icons.article;
      case ExternalShareType.forSale:
        return Icons.shopping_bag;
      case ExternalShareType.request:
        return Icons.help_outline;
      case ExternalShareType.auction:
        return Icons.gavel;
      case ExternalShareType.profile:
        return Icons.person;
    }
  }

  String _getTypeLabel() {
    switch (target.type) {
      case ExternalShareType.post:
        return 'Post';
      case ExternalShareType.forSale:
        return 'Produk';
      case ExternalShareType.request:
        return 'Request';
      case ExternalShareType.auction:
        return 'Lelang';
      case ExternalShareType.profile:
        return 'Profil';
    }
  }

  /// Formatting authority: the envelope entity owns the thousand separators.
  String _formatPrice(num price) {
    return 'Rp ${formatGroupedAmount(price.round())}';
  }
}
