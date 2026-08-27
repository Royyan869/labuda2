import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'app_bottom_sheet_actions.dart';

/// AppBottomSheet for media picker
class AppBottomSheetMediaPicker {
  /// Show quick media picker (camera/gallery)
  static Future<String?> showQuickMediaPicker({
    required BuildContext context,
    String title = 'Select Media',
    bool allowVideo = true,
  }) {
    return AppBottomSheetActions.showActions<String>(
      context: context,
      title: title,
      showCancel: false,
      actions: [
        BottomSheetAction<String>(
          title: 'Camera',
          subtitle: 'Take a new photo',
          icon: Icons.camera_alt_rounded,
          iconColor: AppColors.successGreen,
          onPressed: () => Navigator.of(context).pop('camera'),
        ),
        BottomSheetAction<String>(
          title: 'Photo Gallery',
          subtitle: 'Choose from your photos',
          icon: Icons.photo_library_rounded,
          iconColor: AppColors.primaryBlue,
          onPressed: () => Navigator.of(context).pop('gallery_photo'),
        ),
        if (allowVideo) ...[
          BottomSheetAction<String>(
            title: 'Video Gallery',
            subtitle: 'Choose from your videos',
            icon: Icons.video_library_rounded,
            iconColor: AppColors.primaryRed,
            onPressed: () => Navigator.of(context).pop('gallery_video'),
          ),
        ],
      ],
    );
  }
}
