import 'package:flutter/material.dart';
import 'package:wechat_assets_picker/wechat_assets_picker.dart';
import 'package:labuda/core/core.dart';
import 'package:permission_handler/permission_handler.dart';

/// Media Picker Helper
/// Wrapper for wechat_assets_picker with custom theming
class MediaPickerHelper {
  /// Open gallery to pick photos and videos
  static Future<List<String>?> pickMedia({
    required BuildContext context,
    int maxAssets = 10,
    RequestType requestType = RequestType.common, // photo + video
  }) async {
    try {
      // Request permission first - handle both legacy and Android 13+ permissions
      bool hasPermission = false;

      // For mixed photo+video, request both permissions
      if (requestType == RequestType.common) {
        final photoStatus = await Permission.photos.request();
        final videoStatus = await Permission.videos.request();
        hasPermission = photoStatus.isGranted || videoStatus.isGranted;
      } else if (requestType == RequestType.image) {
        final status = await Permission.photos.request();
        hasPermission = status.isGranted;
      } else if (requestType == RequestType.video) {
        final status = await Permission.videos.request();
        hasPermission = status.isGranted;
      }

      if (!hasPermission) {
        debugPrint('Media permission denied');
        return null;
      }

      // Check if context is still mounted before using it
      if (!context.mounted) return null;

      final List<AssetEntity>? result = await AssetPicker.pickAssets(
        context,
        pickerConfig: AssetPickerConfig(
          maxAssets: maxAssets,
          requestType: requestType,
          themeColor: AppColors.primaryRed,
          textDelegate: const EnglishAssetPickerTextDelegate(),
        ),
      );

      if (result == null || result.isEmpty) return null;

      // Convert AssetEntity to file paths
      final List<String> filePaths = [];
      for (final asset in result) {
        final file = await asset.file;
        if (file != null) {
          filePaths.add(file.path);
        }
      }

      return filePaths.isEmpty ? null : filePaths;
    } catch (e) {
      debugPrint('Error picking media: $e');
      return null;
    }
  }

  /// Open gallery to pick only photos
  static Future<List<String>?> pickPhotos({
    required BuildContext context,
    int maxAssets = 10,
  }) async {
    return pickMedia(
      context: context,
      maxAssets: maxAssets,
      requestType: RequestType.image,
    );
  }

  /// Open gallery to pick only videos
  static Future<List<String>?> pickVideos({
    required BuildContext context,
    int maxAssets = 5,
  }) async {
    return pickMedia(
      context: context,
      maxAssets: maxAssets,
      requestType: RequestType.video,
    );
  }
}
