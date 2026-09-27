import 'package:flutter/material.dart';

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
    final scheme = Theme.of(context).colorScheme;

    if (isLoading) {
      return Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          CircularProgressIndicator(color: scheme.primary),
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
          onTap: onTakePhoto,
        ),
        const SizedBox(height: 16),
        AvatarPickerOption(
          icon: Icons.photo_library_outlined,
          label: 'Choose from Gallery',
          description: 'Select an existing photo',
          onTap: onChooseGallery,
        ),
        if (showRemoveOption && onRemovePhoto != null) ...[
          const SizedBox(height: 16),
          AvatarPickerOption(
            icon: Icons.delete_outline,
            label: 'Remove Photo',
            description: 'Use default avatar',
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
  final VoidCallback onTap;
  final bool isDestructive;

  const AvatarPickerOption({
    super.key,
    required this.icon,
    required this.label,
    required this.description,
    required this.onTap,
    this.isDestructive = false,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final iconColor = isDestructive ? scheme.error : scheme.primary;

    return Material(
      color: Colors.transparent,
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(12),
        child: Container(
          padding: const EdgeInsets.all(16),
          decoration: BoxDecoration(
            border: Border.all(color: scheme.outlineVariant),
            borderRadius: BorderRadius.circular(12),
          ),
          child: Row(
            children: [
              Container(
                padding: const EdgeInsets.all(12),
                decoration: BoxDecoration(
                  color: iconColor.withValues(alpha: 0.12),
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
                            ? scheme.error
                            : scheme.onSurface,
                      ),
                    ),
                    const SizedBox(height: 4),
                    Text(
                      description,
                      style: TextStyle(
                        fontSize: 14,
                        color: scheme.onSurfaceVariant,
                      ),
                    ),
                  ],
                ),
              ),
              Icon(
                Icons.arrow_forward_ios,
                color: scheme.onSurfaceVariant,
                size: 16,
              ),
            ],
          ),
        ),
      ),
    );
  }
}
