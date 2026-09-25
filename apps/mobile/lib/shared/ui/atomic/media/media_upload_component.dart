import 'package:labuda/core/src/theme/app_colors.dart';
import 'package:flutter/material.dart';
import 'package:labuda/shared/ui/base/base_component.dart';
import 'package:labuda/core/media/media_upload_config.dart';
import 'package:labuda/core/media/media_upload_orchestrator.dart';

/// @deprecated Gunakan [MediaGridUploader] / [CompactMediaStrip] + [MediaUploadOrchestrator].
/// Wrapper dipertahankan untuk kompatibilitas, delegasi ke 1 mesin foto+video.
@Deprecated('Gunakan MediaGridUploader + MediaUploadOrchestrator (1 mesin foto+video)')
class MediaUploadComponent extends BaseComponent
    implements
        ValidatableComponent,
        DataComponent<List<String>>,
        ResettableComponent {
  final List<String>? initialMediaUrls;
  final int maxFiles;
  final List<String> allowedTypes;
  final double maxFileSizeMB;
  final bool showPreview;
  final bool allowReorder;
  final void Function(List<String>)? onMediaChanged;
  final String? Function(List<String>)? validator;

  const MediaUploadComponent({
    super.key,
    this.initialMediaUrls,
    this.maxFiles = 5,
    this.allowedTypes = const ['image', 'video'],
    this.maxFileSizeMB = 10.0,
    this.showPreview = true,
    this.allowReorder = true,
    this.onMediaChanged,
    this.validator,
    super.componentId,
    super.isRequired,
    super.errorMessage,
    super.isLoading,
    super.isDisabled,
  });

  @override
  Widget buildContent(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _buildUploadButton(context),
        const SizedBox(height: 8),
        if (showPreview) _buildPreviewGrid(context),
        _buildUploadInfo(context),
      ],
    );
  }

  Widget _buildUploadButton(BuildContext context) {
    return GestureDetector(
      onTap: isDisabled ? null : () => _handleUpload(context),
      child: Container(
        height: 120,
        width: double.infinity,
        decoration: BoxDecoration(
          border: Border.all(color: AppColors.neutral),
          borderRadius: BorderRadius.circular(8),
          color: isDisabled ? AppColors.neutralGray100 : null,
        ),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              Icons.cloud_upload_outlined,
              size: 32,
              color: isDisabled
                  ? AppColors.neutral
                  : Theme.of(context).primaryColor,
            ),
            const SizedBox(height: 8),
            Text(
              'Tap to upload ${allowedTypes.join(" or ")}',
              style: TextStyle(color: isDisabled ? AppColors.neutral : null),
            ),
            Text(
              'Max $maxFiles files, ${maxFileSizeMB}MB each',
              style: TextStyle(fontSize: 12, color: AppColors.neutralGray600),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildPreviewGrid(BuildContext context) {
    final mediaUrls = getData() ?? [];
    if (mediaUrls.isEmpty) return const SizedBox.shrink();

    return GridView.builder(
      shrinkWrap: true,
      physics: const NeverScrollableScrollPhysics(),
      gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
        crossAxisCount: 3,
        crossAxisSpacing: 8,
        mainAxisSpacing: 8,
      ),
      itemCount: mediaUrls.length,
      itemBuilder: (context, index) {
        return Stack(
          children: [
            Container(
              decoration: BoxDecoration(
                borderRadius: BorderRadius.circular(8),
                color: AppColors.neutralGray200,
              ),
              child: const Center(child: Icon(Icons.image_outlined)),
            ),
            Positioned(
              top: 4,
              right: 4,
              child: GestureDetector(
                onTap: () => _removeMedia(index),
                child: Container(
                  padding: const EdgeInsets.all(2),
                  decoration: const BoxDecoration(
                    color: AppColors.error,
                    shape: BoxShape.circle,
                  ),
                  child: const Icon(
                    Icons.close,
                    size: 12,
                    color: AppColors.light,
                  ),
                ),
              ),
            ),
          ],
        );
      },
    );
  }

  Widget _buildUploadInfo(BuildContext context) {
    return Text(
      'Supported: ${allowedTypes.join(", ")} • Max: ${maxFileSizeMB}MB per file',
      style: TextStyle(fontSize: 11, color: AppColors.neutralGray600),
    );
  }

  void _handleUpload(BuildContext context) {
    MediaUploadOrchestrator.showPicker(
      context: context,
      config: MediaUploadConfig(maxImages: maxFiles, maxVideos: maxFiles, maxTotal: maxFiles, maxImageSizeMb: maxFileSizeMB.round(), maxVideoSizeMb: maxFileSizeMB.round()),
      currentCount: (getData() ?? []).length,
      onUploaded: (urls) async {
        final cur = List<String>.from(getData() ?? []);
        cur.addAll(urls);
        onMediaChanged?.call(cur);
      },
    );
  }

  void _removeMedia(int index) {
    final currentMedia = List<String>.from(getData() ?? []);
    currentMedia.removeAt(index);
    onMediaChanged?.call(currentMedia);
  }

  @override
  String? validate() {
    return validator?.call(getData() ?? []) ??
        _defaultValidator(getData() ?? []);
  }

  @override
  List<String>? getData() => initialMediaUrls;

  @override
  void reset() {
    onMediaChanged?.call([]);
  }

  String? _defaultValidator(List<String> mediaUrls) {
    if (isRequired && mediaUrls.isEmpty) {
      return 'At least one media file is required';
    }
    if (mediaUrls.length > maxFiles) {
      return 'Maximum $maxFiles files allowed';
    }
    return null;
  }
}
