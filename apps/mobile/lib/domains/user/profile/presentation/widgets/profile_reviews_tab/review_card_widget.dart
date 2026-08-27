import 'package:flutter/material.dart';
import 'package:labuda/core/src/theme/app_colors.dart';
import 'package:labuda/shared/shared.dart';

/// Individual review card widget
class ReviewCardWidget extends StatelessWidget {
  final Map<String, dynamic> review;
  final VoidCallback onHelpfulTap;

  const ReviewCardWidget({
    super.key,
    required this.review,
    required this.onHelpfulTap,
  });

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final isReceived = review['isReceived'] as bool;

    return Card(
      margin: const EdgeInsets.only(bottom: 16),
      elevation: isDark ? 4 : 2,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
      color: isDark ? AppColors.darkGray700 : AppColors.neutralWhite,
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // Reviewer info
            Row(
              children: [
                // Avatar with real image or initial fallback (author avatar)
                CircleAvatar(
                  radius: 20,
                  backgroundColor: AppColors.primaryRed.withValues(alpha: 0.1),
                  backgroundImage: review['authorAvatar'] != null
                      ? NetworkImage(review['authorAvatar'])
                      : null,
                  child: review['authorAvatar'] == null
                      ? Text(
                          review['authorName'].toString().isNotEmpty
                              ? review['authorName']
                                    .toString()
                                    .substring(
                                      0,
                                      1.clamp(
                                        0,
                                        review['authorName'].toString().length,
                                      ),
                                    )
                                    .toUpperCase()
                              : 'U',
                          style: const TextStyle(
                            color: AppColors.primaryRed,
                            fontWeight: FontWeight.bold,
                          ),
                        )
                      : null,
                ),
                const SizedBox(width: 12),

                // Username info (simplified)
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      // Author username
                      Text(
                        '@${review['authorUsername']}',
                        style: TextStyle(
                          fontSize: 14,
                          fontWeight: FontWeight.w600,
                          color: isDark
                              ? AppColors.neutralWhite
                              : AppColors.neutralGray900,
                        ),
                        overflow: TextOverflow.ellipsis,
                        maxLines: 1,
                      ),
                      // Tab "Diberikan": show "Kepada @username"
                      if (!isReceived && review['recipientUsername'] != null)
                        Row(
                          children: [
                            Text(
                              'Kepada: ',
                              style: TextStyle(
                                fontSize: 12,
                                color: isDark
                                    ? AppColors.neutralGray400
                                    : AppColors.neutralGray500,
                              ),
                            ),
                            Flexible(
                              child: Text(
                                '@${review['recipientUsername']}',
                                style: TextStyle(
                                  fontSize: 12,
                                  color: isDark
                                      ? AppColors.neutralGray400
                                      : AppColors.neutralGray500,
                                ),
                                overflow: TextOverflow.ellipsis,
                                maxLines: 1,
                              ),
                            ),
                          ],
                        ),
                    ],
                  ),
                ),

                // Rating and date
                Column(
                  crossAxisAlignment: CrossAxisAlignment.end,
                  children: [
                    _buildStarRating(review['rating'].toDouble(), 14),
                    const SizedBox(height: 2),
                    TimeAgoWidget.compact(
                      dateTime: review['createdAt'] as DateTime,
                      color: isDark
                          ? AppColors.neutralGray400
                          : AppColors.neutralGray500,
                      fontSize: 11,
                    ),
                  ],
                ),
              ],
            ),

            const SizedBox(height: 12),

            // Review comment
            Text(
              review['comment'],
              style: TextStyle(
                fontSize: 14,
                height: 1.4,
                color: isDark
                    ? AppColors.neutralGray200
                    : AppColors.neutralGray700,
              ),
            ),

            const SizedBox(height: 12),

            // Helpful button
            Row(
              children: [
                const Spacer(),
                GestureDetector(
                  onTap: onHelpfulTap,
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(
                        review['isHelpful'] ?? false
                            ? Icons.thumb_up
                            : Icons.thumb_up_outlined,
                        size: 16,
                        color: review['isHelpful'] ?? false
                            ? AppColors.primaryRed
                            : (isDark
                                  ? AppColors.neutralGray400
                                  : AppColors.neutralGray500),
                      ),
                      const SizedBox(width: 4),
                      Text(
                        'Helpful (${review['helpfulCount'] ?? 0})',
                        style: TextStyle(
                          fontSize: 12,
                          color: review['isHelpful'] ?? false
                              ? AppColors.primaryRed
                              : (isDark
                                    ? AppColors.neutralGray400
                                    : AppColors.neutralGray500),
                        ),
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildStarRating(double rating, double size) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: List.generate(5, (index) {
        return Icon(
          index < rating.floor()
              ? Icons.star
              : index < rating
              ? Icons.star_half
              : Icons.star_border,
          size: size,
          color: Colors.amber,
        );
      }),
    );
  }
}
