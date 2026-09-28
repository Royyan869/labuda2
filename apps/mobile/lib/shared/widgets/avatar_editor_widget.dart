import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:image_picker/image_picker.dart';
import 'package:labuda/shared/shared.dart';
import 'avatar_picker_options.dart';
import 'avatar_image_processor.dart';

/// Single crop entry for avatar, store photo, and cover.
///
/// Features:
/// - Image picker (camera/gallery, OS camera single-shot)
/// - Platform-specific cropping (crop_your_image on mobile, canvas on web)
/// - Returns the cropped local path; the caller uploads (S3 fixed key).
/// Crop shape is expressed honestly via [aspectRatio]/[circularCrop]/[cropTitle]:
/// avatar+store are 1:1 circular, cover is 16:9 rectangular.
///
/// Refactored into modular components:
/// - AvatarPickerOptions: For UI options
/// - AvatarImageProcessor: For image processing logic
class AvatarEditorWidget {
  static void showEditModal({
    required BuildContext context,
    required String userId,
    required Function(String? avatarUrl) onAvatarUpdated,
    double aspectRatio = 1.0,
    bool circularCrop = true,
    String cropTitle = 'Crop Avatar',
    String modalTitle = 'Change Profile Photo',
  }) {
    AppModal.show(
      context: context,
      title: modalTitle,
      width: 400,
      content: _AvatarPickerContent(
        userId: userId,
        onAvatarUpdated: onAvatarUpdated,
        parentContext: context,
        aspectRatio: aspectRatio,
        circularCrop: circularCrop,
        cropTitle: cropTitle,
      ),
    );
  }
}

class _AvatarPickerContent extends ConsumerStatefulWidget {
  final String userId;
  final Function(String? avatarUrl) onAvatarUpdated;
  final BuildContext parentContext;
  final double aspectRatio;
  final bool circularCrop;
  final String cropTitle;

  const _AvatarPickerContent({
    required this.userId,
    required this.onAvatarUpdated,
    required this.parentContext,
    this.aspectRatio = 1.0,
    this.circularCrop = true,
    this.cropTitle = 'Crop Avatar',
  });

  @override
  ConsumerState<_AvatarPickerContent> createState() =>
      _AvatarPickerContentState();
}

class _AvatarPickerContentState extends ConsumerState<_AvatarPickerContent> {
  final bool _isLoading = false;

  @override
  Widget build(BuildContext context) {
    return AvatarPickerOptions(
      isLoading: _isLoading,
      onTakePhoto: () => _pickAndCropImage(ImageSource.camera),
      onChooseGallery: () => _pickAndCropImage(ImageSource.gallery),
      showRemoveOption: false,
    );
  }

  Future<void> _pickAndCropImage(ImageSource source) async {
    try {
      if (kIsWeb && source == ImageSource.camera) {
        Navigator.of(context).pop();
        if (mounted) {
          AppSnackBar.showError(
            context,
            'Camera not supported on web. Please use gallery.',
          );
        }
        return;
      }

      // Store references before closing dialog
      final userId = widget.userId;
      final onAvatarUpdated = widget.onAvatarUpdated;
      final parentContext = widget.parentContext;

      // Close dialog before picking image
      Navigator.of(context).pop();

      await AvatarImageProcessor.pickAndCropImage(
        parentContext,
        source,
        userId,
        onAvatarUpdated,
        aspectRatio: widget.aspectRatio,
        circularCrop: widget.circularCrop,
        cropTitle: widget.cropTitle,
      );
    } catch (e) {
      // The dialog was already popped above before picking. Never pop again
      // here: an extra pop would close the caller's screen (P1: the seller
      // wizard died after crop and the app fell back to home).
      if (mounted && context.mounted) {
        AppSnackBar.showError(context, 'Failed to pick image: $e');
      }
    }
  }
}
