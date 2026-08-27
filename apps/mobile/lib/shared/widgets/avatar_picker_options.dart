import 'package:flutter/material.dart';
import 'package:labuda/core/src/theme/app_colors.dart';

/// Avatar picker options UI component
///
/// Features:
/// - Camera option
/// - Gallery option
/// - Remove option (optional)
/// - Consistent styling
class AvatarPickerOptions extends StatelessWidget {
  final VoidCallback onTakePhoto;
  final VoidCallback onChooseGallery;
  final VoidCallback? onRemovePhoto;
  final bool isLoading;
  final bool showRemoveOption;

  const AvatarPickerOptions({
    super.key,
    required this.onTakePhoto,
    required this.onChooseGallery,
    this.onRemovePhoto,
    this.isLoading = false,
    this.showRemoveOption = true,
  });

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    if (isLoading) {
      return const Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          CircularProgressIndicator(color: AppColors.primaryRed),
          SizedBox(height: 20),
          Text('Processing image...', style: TextStyle(fontSize: 14)),
        ],
      );
    }

    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        AvatarPickerOption(
          icon: Icons.camera_alt_outlined,
          label: 'Take Photo',
          description: 'Use camera to take a new photo',
          isDark: isDark,
          onTap: onTakePhoto,
        ),
        const SizedBox(height: 16),
        AvatarPickerOption(
          icon: Icons.photo_library_outlined,
          label: 'Choose from Gallery',
          description: 'Select an existing photo',
          isDark: isDark,
          onTap: onChooseGallery,
        ),
        if (showRemoveOption && onRemovePhoto != null) ...[
          const SizedBox(height: 16),
          AvatarPickerOption(
            icon: Icons.delete_outline,
            label: 'Remove Photo',
            description: 'Use default avatar',
            isDark: isDark,
            isDestructive: true,
            onTap: onRemovePhoto!,
          ),
        ],
      ],
    );
  }
}

/// Individual avatar picker option widget
class AvatarPickerOption extends StatelessWidget {
  final IconData icon;
  final String label;
  final String description;
  final bool isDark;
  final VoidCallback onTap;
  final bool isDestructive;

  const AvatarPickerOption({
    super.key,
    required this.icon,
    required this.label,
    required this.description,
    required this.isDark,
    required this.onTap,
    this.isDestructive = false,
  });

  @override
  Widget build(BuildContext context) {
    final iconColor = isDestructive
        ? AppColors.statusError
        : AppColors.primaryRed;

    return Material(
      color: Colors.transparent,
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(12),
        child: Container(
          padding: const EdgeInsets.all(16),
          decoration: BoxDecoration(
            border: Border.all(
              color: isDark ? AppColors.darkGray600 : AppColors.neutralGray200,
            ),
            borderRadius: BorderRadius.circular(12),
          ),
          child: Row(
            children: [
              Container(
                padding: const EdgeInsets.all(12),
                decoration: BoxDecoration(
                  color: iconColor.withValues(alpha: 0.1),
                  borderRadius: BorderRadius.circular(8),
                ),
                child: Icon(icon, color: iconColor, size: 24),
              ),
              const SizedBox(width: 16),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      label,
                      style: TextStyle(
                        fontSize: 16,
                        fontWeight: FontWeight.w500,
                        color: isDestructive
                            ? AppColors.statusError
                            : (isDark
                                  ? AppColors.neutralGray200
                                  : AppColors.neutralGray800),
                      ),
                    ),
                    const SizedBox(height: 4),
                    Text(
                      description,
                      style: TextStyle(
                        fontSize: 14,
                        color: isDark
                            ? AppColors.neutralGray400
                            : AppColors.neutralGray500,
                      ),
                    ),
                  ],
                ),
              ),
              Icon(
                Icons.arrow_forward_ios,
                color: isDark
                    ? AppColors.neutralGray400
                    : AppColors.neutralGray500,
                size: 16,
              ),
            ],
          ),
        ),
      ),
    );
  }
}
