import 'package:flutter/material.dart';
import 'package:cached_network_image/cached_network_image.dart';
import 'package:labuda/core/core.dart';
import '../../domain/entities/share_target.dart';

/// Preview card showing what content will be shared
class SharePreviewCard extends StatelessWidget {
  final ShareTarget target;
  final bool isDark;

  const SharePreviewCard({
    super.key,
    required this.target,
    this.isDark = false,
  });

  @override
  Widget build(BuildContext context) {
    final cardColor = isDark ? AppColors.darkGray700 : AppColors.neutralGray50;
    final borderColor = isDark
        ? AppColors.darkGray600
        : AppColors.neutralGray200;
    final textColor = isDark
        ? AppColors.neutralGray100
        : AppColors.neutralGray900;
    final secondaryTextColor = isDark
        ? AppColors.neutralGray400
        : AppColors.neutralGray500;
    final placeholderColor = isDark
        ? AppColors.darkGray600
        : AppColors.neutralGray200;

    return Container(
      margin: const EdgeInsets.all(16),
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: cardColor,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: borderColor, width: 1),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          // Image preview - SQUARE (if exists)
          if (target.imageUrl != null) ...[
            ClipRRect(
              borderRadius: BorderRadius.circular(8),
              child: AspectRatio(
                aspectRatio: 1.0, // Square image
                child: CachedNetworkImage(
                  imageUrl: target.imageUrl!,
                  width: double.infinity,
                  fit: BoxFit.cover,
                  placeholder: (context, url) => Container(
                    color: placeholderColor,
                    child: const Center(child: CircularProgressIndicator()),
                  ),
                  errorWidget: (context, url, error) => Container(
                    color: placeholderColor,
                    child: Icon(
                      Icons.broken_image,
                      size: 48,
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
              Icon(_getTypeIcon(), size: 16, color: secondaryTextColor),
              const SizedBox(width: 4),
              Text(
                _getTypeLabel(),
                style: AppTypography.caption.copyWith(
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
            style: AppTypography.bodyLarge.copyWith(
              fontWeight: FontWeight.w600,
              color: textColor,
            ),
            maxLines: 2,
            overflow: TextOverflow.ellipsis,
          ),
          const SizedBox(height: 8),

          // Metadata display berdasarkan content type
          _buildMetadataSection(textColor, secondaryTextColor),
        ],
      ),
    );
  }

  /// Build metadata section based on content type
  Widget _buildMetadataSection(Color textColor, Color secondaryTextColor) {
    switch (target.type) {
      case ExternalShareType.listing:
        return _buildListingMetadata(textColor, secondaryTextColor);
      case ExternalShareType.auction:
        return _buildAuctionMetadata(textColor, secondaryTextColor);
      case ExternalShareType.request:
        return _buildRequestMetadata(textColor, secondaryTextColor);
      case ExternalShareType.post:
      case ExternalShareType.profile:
        return _buildDefaultMetadata(secondaryTextColor);
    }
  }

  /// Listing metadata - compact dengan info variety & size
  Widget _buildListingMetadata(Color textColor, Color secondaryTextColor) {
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
                Icon(Icons.category, size: 14, color: secondaryTextColor),
                const SizedBox(width: 4),
                Flexible(
                  child: Text(
                    variety,
                    style: AppTypography.caption.copyWith(
                      color: secondaryTextColor,
                    ),
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
              ],
              if (variety != null && size != null) const SizedBox(width: 12),
              if (size != null) ...[
                Icon(Icons.straighten, size: 14, color: secondaryTextColor),
                const SizedBox(width: 4),
                Text(
                  '${size.toStringAsFixed(0)} cm',
                  style: AppTypography.caption.copyWith(
                    color: secondaryTextColor,
                  ),
                ),
              ],
            ],
          ),

        // Row 2: Location
        if (location != null) ...[
          const SizedBox(height: 4),
          Row(
            children: [
              Icon(Icons.location_on, size: 14, color: secondaryTextColor),
              const SizedBox(width: 4),
              Flexible(
                child: Text(
                  location,
                  style: AppTypography.caption.copyWith(
                    color: secondaryTextColor,
                  ),
                  overflow: TextOverflow.ellipsis,
                ),
              ),
            ],
          ),
        ],
      ],
    );
  }

  /// Auction metadata - show current bid & time remaining
  Widget _buildAuctionMetadata(Color textColor, Color secondaryTextColor) {
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
              const Icon(Icons.gavel, size: 16, color: AppColors.primaryRed),
              const SizedBox(width: 4),
              Text(
                'KB: ${_formatPrice(currentBid)}',
                style: AppTypography.bodyMedium.copyWith(
                  color: AppColors.primaryRed,
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
                Icon(Icons.category, size: 14, color: secondaryTextColor),
                const SizedBox(width: 4),
                Flexible(
                  child: Text(
                    variety,
                    style: AppTypography.caption.copyWith(
                      color: secondaryTextColor,
                    ),
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
              ],
              if (variety != null && size != null) const SizedBox(width: 12),
              if (size != null) ...[
                Icon(Icons.straighten, size: 14, color: secondaryTextColor),
                const SizedBox(width: 4),
                Text(
                  '${size.toStringAsFixed(0)} cm',
                  style: AppTypography.caption.copyWith(
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
                size: 14,
                color: isUrgent ? AppColors.statusError : secondaryTextColor,
              ),
              const SizedBox(width: 4),
              Text(
                timeRemaining,
                style: AppTypography.caption.copyWith(
                  color: isUrgent ? AppColors.statusError : secondaryTextColor,
                  fontWeight: isUrgent ? FontWeight.w600 : FontWeight.normal,
                ),
              ),
            ],
          ),
        ],
      ],
    );
  }

  /// Request metadata - show budget
  Widget _buildRequestMetadata(Color textColor, Color secondaryTextColor) {
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
              const Icon(
                Icons.account_balance_wallet,
                size: 16,
                color: AppColors.primaryBlue,
              ),
              const SizedBox(width: 4),
              Text(
                maxBudget != null
                    ? 'Budget: ${_formatPrice(maxBudget)}'
                    : 'Budget: ${_formatPrice(budget!)}',
                style: AppTypography.bodyMedium.copyWith(
                  color: AppColors.primaryBlue,
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
              Icon(Icons.category, size: 14, color: secondaryTextColor),
              const SizedBox(width: 4),
              Flexible(
                child: Text(
                  variety,
                  style: AppTypography.caption.copyWith(
                    color: secondaryTextColor,
                  ),
                  overflow: TextOverflow.ellipsis,
                ),
              ),
            ],
          ),

        if (location != null) ...[
          const SizedBox(height: 4),
          Row(
            children: [
              Icon(Icons.location_on, size: 14, color: secondaryTextColor),
              const SizedBox(width: 4),
              Flexible(
                child: Text(
                  location,
                  style: AppTypography.caption.copyWith(
                    color: secondaryTextColor,
                  ),
                  overflow: TextOverflow.ellipsis,
                ),
              ),
            ],
          ),
        ],
      ],
    );
  }

  /// Default metadata - show description
  Widget _buildDefaultMetadata(Color secondaryTextColor) {
    if (target.description.isEmpty) return const SizedBox.shrink();

    return Text(
      target.description,
      style: AppTypography.bodyMedium.copyWith(color: secondaryTextColor),
      maxLines: 2,
      overflow: TextOverflow.ellipsis,
    );
  }

  IconData _getTypeIcon() {
    switch (target.type) {
      case ExternalShareType.post:
        return Icons.article;
      case ExternalShareType.listing:
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
      case ExternalShareType.listing:
        return 'Produk';
      case ExternalShareType.request:
        return 'Request';
      case ExternalShareType.auction:
        return 'Lelang';
      case ExternalShareType.profile:
        return 'Profil';
    }
  }

  String _formatPrice(num price) {
    final priceStr = price.toStringAsFixed(0);
    return 'Rp ${priceStr.replaceAllMapped(RegExp(r'(\d{1,3})(?=(\d{3})+(?!\d))'), (Match m) => '${m[1]}.')}';
  }
}
