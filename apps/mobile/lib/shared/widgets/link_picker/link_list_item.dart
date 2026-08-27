import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:cached_network_image/cached_network_image.dart';

/// Reusable list item for link picker
class LinkListItem extends StatelessWidget {
  final String? imageUrl;
  final String title;
  final String subtitle;
  final String? price;
  final String? badge;
  final Color? badgeColor;
  final bool isSelected;
  final VoidCallback onTap;

  const LinkListItem({
    super.key,
    this.imageUrl,
    required this.title,
    required this.subtitle,
    this.price,
    this.badge,
    this.badgeColor,
    this.isSelected = false,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Material(
      color: Colors.transparent,
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(12),
        child: Container(
          padding: const EdgeInsets.all(12),
          decoration: BoxDecoration(
            color: isSelected
                ? (isDark
                      ? AppColors.primaryRed.withValues(alpha: 0.15)
                      : AppColors.primaryRed.withValues(alpha: 0.08))
                : (isDark ? AppColors.darkGray700 : AppColors.neutralGray50),
            borderRadius: BorderRadius.circular(12),
            border: Border.all(
              color: isSelected
                  ? AppColors.primaryRed
                  : (isDark ? AppColors.darkGray600 : AppColors.neutralGray200),
              width: isSelected ? 2 : 1,
            ),
          ),
          child: Row(
            children: [
              // Image
              if (imageUrl != null) ...[
                ClipRRect(
                  borderRadius: BorderRadius.circular(8),
                  child: CachedNetworkImage(
                    imageUrl: imageUrl!,
                    width: 60,
                    height: 60,
                    fit: BoxFit.cover,
                    placeholder: (context, url) => Container(
                      width: 60,
                      height: 60,
                      color: isDark
                          ? AppColors.darkGray600
                          : AppColors.neutralGray200,
                      child: Icon(
                        Icons.image,
                        color: isDark
                            ? AppColors.neutralGray400
                            : AppColors.neutralGray500,
                      ),
                    ),
                    errorWidget: (context, url, error) => Container(
                      width: 60,
                      height: 60,
                      color: isDark
                          ? AppColors.darkGray600
                          : AppColors.neutralGray200,
                      child: Icon(
                        Icons.broken_image,
                        color: isDark
                            ? AppColors.neutralGray400
                            : AppColors.neutralGray500,
                      ),
                    ),
                  ),
                ),
              ],
              const SizedBox(width: 12),

              // Content
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    // Badge + Title row
                    Row(
                      children: [
                        if (badge != null) ...[
                          Container(
                            padding: const EdgeInsets.symmetric(
                              horizontal: 6,
                              vertical: 2,
                            ),
                            decoration: BoxDecoration(
                              color: (badgeColor ?? AppColors.primaryRed)
                                  .withValues(alpha: 0.15),
                              borderRadius: BorderRadius.circular(4),
                            ),
                            child: Text(
                              badge!,
                              style: TextStyle(
                                fontSize: 10,
                                fontWeight: FontWeight.w600,
                                color: badgeColor ?? AppColors.primaryRed,
                              ),
                            ),
                          ),
                          const SizedBox(width: 6),
                        ],
                        Expanded(
                          child: Text(
                            title,
                            style: TextStyle(
                              fontSize: 14,
                              fontWeight: FontWeight.w600,
                              color: isDark
                                  ? Colors.white
                                  : AppColors.neutralGray900,
                            ),
                            maxLines: 2,
                            overflow: TextOverflow.ellipsis,
                          ),
                        ),
                      ],
                    ),
                    const SizedBox(height: 4),
                    // Subtitle + Price row
                    Row(
                      children: [
                        Expanded(
                          child: Text(
                            subtitle,
                            style: TextStyle(
                              fontSize: 12,
                              color: isDark
                                  ? AppColors.neutralGray400
                                  : AppColors.neutralGray600,
                            ),
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                          ),
                        ),
                        if (price != null) ...[
                          const SizedBox(width: 8),
                          Text(
                            price!,
                            style: TextStyle(
                              fontSize: 13,
                              fontWeight: FontWeight.w600,
                              color: AppColors.primaryGreen,
                            ),
                          ),
                        ],
                      ],
                    ),
                  ],
                ),
              ),

              // Selection indicator
              if (isSelected)
                Container(
                  padding: const EdgeInsets.all(4),
                  decoration: BoxDecoration(
                    color: AppColors.primaryRed,
                    shape: BoxShape.circle,
                  ),
                  child: const Center(
                    child: Icon(
                      Icons.check_circle,
                      color: Colors.white,
                      size: 28,
                    ),
                  ),
                ),
            ],
          ),
        ),
      ),
    );
  }
}
