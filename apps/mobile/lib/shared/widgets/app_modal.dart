import 'package:flutter/material.dart';
import 'package:image_picker/image_picker.dart';
import 'package:labuda/core/core.dart';

/// Reusable Modal Dialog System
///
/// Provides consistent modal dialogs across the app instead of bottom sheets
/// Features:
/// - Professional dialog styling
/// - Dark/Light mode support
/// - Responsive sizing
/// - Consistent animations
class AppModal {
  /// Show a generic modal dialog
  static Future<T?> show<T>({
    required BuildContext context,
    required String title,
    required Widget content,
    List<Widget>? actions,
    bool barrierDismissible = true,
    double? width,
    double? height,
    EdgeInsetsGeometry? contentPadding,
  }) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return showDialog<T>(
      context: context,
      barrierDismissible: barrierDismissible,
      builder: (BuildContext context) {
        return Dialog(
          backgroundColor: isDark
              ? AppColors.darkGray800
              : AppColors.neutralWhite,
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(16),
          ),
          child: Container(
            width: width,
            height: height,
            constraints: BoxConstraints(
              maxWidth: MediaQuery.of(context).size.width * 0.9,
              maxHeight: MediaQuery.of(context).size.height * 0.8,
            ),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                // Header
                Container(
                  padding: const EdgeInsets.fromLTRB(16, 12, 8, 12),
                  decoration: BoxDecoration(
                    color: isDark
                        ? AppColors.darkGray700
                        : AppColors.neutralGray50,
                    borderRadius: const BorderRadius.vertical(
                      top: Radius.circular(16),
                    ),
                  ),
                  child: Row(
                    children: [
                      Expanded(
                        child: Text(
                          title,
                          style: TextStyle(
                            fontSize: 16,
                            fontWeight: FontWeight.w600,
                            color: isDark
                                ? AppColors.neutralWhite
                                : AppColors.neutralGray900,
                          ),
                        ),
                      ),
                      IconButton(
                        onPressed: () => Navigator.of(context).pop(),
                        icon: Icon(
                          Icons.close,
                          color: isDark
                              ? AppColors.neutralGray400
                              : AppColors.neutralGray600,
                          size: 18,
                        ),
                        iconSize: 18,
                        padding: const EdgeInsets.all(8),
                        constraints: const BoxConstraints(),
                      ),
                    ],
                  ),
                ),

                // Content
                Flexible(
                  child: Container(
                    padding: contentPadding ?? const EdgeInsets.all(16),
                    child: content,
                  ),
                ),

                // Actions
                if (actions != null && actions.isNotEmpty) ...[
                  Container(
                    padding: const EdgeInsets.fromLTRB(16, 8, 16, 16),
                    child: Row(
                      mainAxisAlignment: MainAxisAlignment.end,
                      children: actions
                          .map(
                            (action) => Padding(
                              padding: const EdgeInsets.only(left: 12),
                              child: action,
                            ),
                          )
                          .toList(),
                    ),
                  ),
                ],
              ],
            ),
          ),
        );
      },
    );
  }

  /// Show image source selection modal (Camera/Gallery)
  static Future<ImageSource?> showImageSourcePicker({
    required BuildContext context,
    String title = 'Select Image Source',
    String cameraLabel = 'Camera',
    String galleryLabel = 'Gallery',
  }) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return show<ImageSource>(
      context: context,
      title: title,
      width: 320,
      contentPadding: const EdgeInsets.symmetric(horizontal: 20, vertical: 16),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          _buildImageSourceOption(
            icon: Icons.camera_alt_outlined,
            label: cameraLabel,
            isDark: isDark,
            onTap: () => Navigator.of(context).pop(ImageSource.camera),
          ),
          const SizedBox(height: 12),
          _buildImageSourceOption(
            icon: Icons.photo_library_outlined,
            label: galleryLabel,
            isDark: isDark,
            onTap: () => Navigator.of(context).pop(ImageSource.gallery),
          ),
        ],
      ),
    );
  }

  /// Show confirmation modal
  static Future<bool?> showConfirmation({
    required BuildContext context,
    required String title,
    required String message,
    String confirmLabel = 'Confirm',
    String cancelLabel = 'Cancel',
    Color? confirmColor,
    IconData? icon,
  }) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return show<bool>(
      context: context,
      title: title,
      width: 400,
      content: Row(
        children: [
          if (icon != null) ...[
            Icon(icon, color: confirmColor ?? AppColors.primaryRed, size: 24),
            const SizedBox(width: 16),
          ],
          Expanded(
            child: Text(
              message,
              style: TextStyle(
                fontSize: 16,
                color: isDark
                    ? AppColors.neutralGray200
                    : AppColors.neutralGray700,
                height: 1.4,
              ),
            ),
          ),
        ],
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(false),
          child: Text(
            cancelLabel,
            style: TextStyle(
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray600,
            ),
          ),
        ),
        ElevatedButton(
          onPressed: () => Navigator.of(context).pop(true),
          style: ElevatedButton.styleFrom(
            backgroundColor: confirmColor ?? AppColors.primaryRed,
            foregroundColor: AppColors.neutralWhite,
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(8),
            ),
          ),
          child: Text(confirmLabel),
        ),
      ],
    );
  }

  /// Show loading modal
  static Future<T?> showLoading<T>({
    required BuildContext context,
    String title = 'Loading...',
    String message = 'Please wait...',
  }) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return show<T>(
      context: context,
      title: title,
      width: 300,
      barrierDismissible: false,
      content: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          const CircularProgressIndicator(color: AppColors.primaryRed),
          const SizedBox(height: 20),
          Text(
            message,
            textAlign: TextAlign.center,
            style: TextStyle(
              fontSize: 14,
              color: isDark
                  ? AppColors.neutralGray300
                  : AppColors.neutralGray600,
            ),
          ),
        ],
      ),
    );
  }

  static Widget _buildImageSourceOption({
    required IconData icon,
    required String label,
    required bool isDark,
    required VoidCallback onTap,
  }) {
    return Material(
      color: Colors.transparent,
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(12),
        child: Container(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 16),
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
                  color: AppColors.primaryRed.withValues(alpha: 0.1),
                  borderRadius: BorderRadius.circular(8),
                ),
                child: Icon(icon, color: AppColors.primaryRed, size: 24),
              ),
              const SizedBox(width: 16),
              Expanded(
                child: Text(
                  label,
                  style: TextStyle(
                    fontSize: 16,
                    fontWeight: FontWeight.w500,
                    color: isDark
                        ? AppColors.neutralGray200
                        : AppColors.neutralGray800,
                  ),
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
