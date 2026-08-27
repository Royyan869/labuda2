import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
// import 'package:labuda/shared/widgets/user_avatar.dart'; // TODO: Create UserAvatar widget
import 'list_item_types.dart';

/// Configuration for leading widget
class ListItemLeading {
  final LeadingType type;
  final IconData? icon;
  final String? imageUrl;
  final String? initials;
  final String? userId;
  final Color? backgroundColor;
  final Color? iconColor;
  final double size;
  final double borderRadius;
  final Widget? custom;

  const ListItemLeading._({
    required this.type,
    this.icon,
    this.imageUrl,
    this.initials,
    this.userId,
    this.backgroundColor,
    this.iconColor,
    this.size = 40,
    this.borderRadius = 10,
    this.custom,
  });

  /// Icon in colored container
  factory ListItemLeading.icon(
    IconData icon, {
    Color? backgroundColor,
    Color? iconColor,
    double size = 40,
    double borderRadius = 10,
  }) => ListItemLeading._(
    type: LeadingType.icon,
    icon: icon,
    backgroundColor: backgroundColor,
    iconColor: iconColor,
    size: size,
    borderRadius: borderRadius,
  );

  /// Simple icon without container
  factory ListItemLeading.simpleIcon(
    IconData icon, {
    Color? color,
    double size = 24,
  }) => ListItemLeading._(
    type: LeadingType.icon,
    icon: icon,
    iconColor: color,
    size: size,
    backgroundColor: Colors.transparent,
    borderRadius: 0,
  );

  /// Avatar with image or initials
  factory ListItemLeading.avatar({
    String? imageUrl,
    String? initials,
    String? userId,
    double size = 40,
  }) => ListItemLeading._(
    type: LeadingType.avatar,
    imageUrl: imageUrl,
    initials: initials,
    userId: userId,
    size: size,
  );

  /// Square/rounded image
  factory ListItemLeading.image(
    String imageUrl, {
    double size = 60,
    double borderRadius = 8,
  }) => ListItemLeading._(
    type: LeadingType.image,
    imageUrl: imageUrl,
    size: size,
    borderRadius: borderRadius,
  );

  /// Custom leading widget
  factory ListItemLeading.custom(Widget widget) =>
      ListItemLeading._(type: LeadingType.custom, custom: widget);
}

/// Build widget from ListItemLeading configuration
Widget buildListItemLeading(
  ListItemLeading config,
  bool isDark,
  Color defaultIconColor,
) {
  switch (config.type) {
    case LeadingType.icon:
      return Container(
        width: config.size,
        height: config.size,
        decoration: BoxDecoration(
          color:
              config.backgroundColor ??
              (isDark ? AppColors.darkGray700 : AppColors.neutralGray100),
          borderRadius: BorderRadius.circular(config.borderRadius),
        ),
        child: Icon(
          config.icon,
          size: config.size * 0.5,
          color: config.iconColor ?? defaultIconColor,
        ),
      );

    case LeadingType.avatar:
      // TODO: Implement UserAvatar widget
      // if (config.userId != null) {
      //   return UserAvatar(
      //     userId: config.userId!,
      //     size: config.size,
      //   );
      // }
      // Fallback to initials or image
      return Container(
        width: config.size,
        height: config.size,
        decoration: BoxDecoration(
          color: config.backgroundColor ?? AppColors.primaryBlue,
          borderRadius: BorderRadius.circular(config.size / 2),
          image: config.imageUrl != null
              ? DecorationImage(
                  image: NetworkImage(config.imageUrl!),
                  fit: BoxFit.cover,
                )
              : null,
        ),
        child: config.imageUrl == null
            ? Center(
                child: Text(
                  config.initials ?? '?',
                  style: AppTypography.bodyLarge.copyWith(
                    color: Colors.white,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              )
            : null,
      );

    case LeadingType.image:
      return ClipRRect(
        borderRadius: BorderRadius.circular(config.borderRadius),
        child: Image.network(
          config.imageUrl!,
          width: config.size,
          height: config.size,
          fit: BoxFit.cover,
          errorBuilder: (context, error, stackTrace) => Container(
            width: config.size,
            height: config.size,
            color: isDark ? AppColors.darkGray700 : AppColors.neutralGray100,
            child: Icon(
              Icons.image_not_supported_outlined,
              color: defaultIconColor,
            ),
          ),
        ),
      );

    case LeadingType.custom:
      return config.custom!;
  }
}
