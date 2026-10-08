library;

import 'dart:io';

import 'package:flutter/material.dart';
import 'package:labuda/shared/widgets/app_image.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/utils/app_formatters.dart';
import 'package:labuda/shared/widgets/app_bottom_sheet_actions.dart';
import 'package:labuda/shared/widgets/app_snackbar.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/external_product.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/external_product_media.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/external_product_review_status.dart';
import 'package:labuda/domains/commerce/pricing/promotion/presentation/providers/canonical_external_product_providers.dart';
import 'package:labuda/core/media/media_upload_config.dart';
import 'package:labuda/core/media/media_upload_orchestrator.dart';

class ExternalProductDetailScreen extends ConsumerStatefulWidget {
  final String productId;

  const ExternalProductDetailScreen({super.key, required this.productId});

  @override
  ConsumerState<ExternalProductDetailScreen> createState() =>
      _ExternalProductDetailScreenState();
}

class _ExternalProductDetailScreenState
    extends ConsumerState<ExternalProductDetailScreen> {
  bool _isSubmitting = false;

  @override
  Widget build(BuildContext context) {
    final detailAsync = ref.watch(
      externalProductDetailProvider(widget.productId),
    );

    return Scaffold(
      appBar: AppBar(title: const Text('External Product')),
      body: detailAsync.when(
        data: (result) {
          if (!result.isSuccess) {
            return Center(child: const Text('Data belum bisa dimuat.'));
          }

          final product = result.data;
          if (product == null) {
            return const Center(child: Text('Product not found'));
          }

          return _buildContent(context, product);
        },
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (error, _) =>
            const Center(child: Text('Data belum bisa dimuat.')),
      ),
    );
  }

  Widget _buildContent(BuildContext context, ExternalProduct product) {
    return ListView(
      padding: const EdgeInsets.all(AppMetrics.p16),
      children: [
        // Product info section
        _SectionCard(
          title: 'Product Info',
          children: [
            _kv('Title', product.title),
            _kv('URL', product.externalUrl),
            if (product.description != null)
              _kv('Description', product.description!),
            _kv('Review Status', _reviewStatusLabel(product.reviewStatus)),
            if (product.publicVisible) _kv('Public Visibility', 'Visible'),
            if (product.unsafeUrlFlag) _kv('URL Safety', 'Flagged as unsafe'),
            _kv('Created', AppFormatters.formatDateTime(product.createdAt)),
            _kv('Updated', AppFormatters.formatDateTime(product.updatedAt)),
            if (product.submittedAt != null)
              _kv(
                'Submitted',
                AppFormatters.formatDateTime(product.submittedAt!),
              ),
            if (product.approvedAt != null)
              _kv(
                'Approved',
                AppFormatters.formatDateTime(product.approvedAt!),
              ),
          ],
        ),

        // Rejection / request-changes reason section
        if (product.hasRejectionReason) ...[
          const SizedBox(height: 12),
          _buildDecisionReasonCard(product),
        ],

        // Media section
        const SizedBox(height: 12),
        _SectionCard(
          title: 'Media (${product.media.length})',
          children: [
            if (product.media.isEmpty)
              const Padding(
                padding: EdgeInsets.symmetric(vertical: AppMetrics.p8),
                child: Text('No media attached'),
              ),
            ...product.media.map(
              (media) => _MediaRow(
                media: media,
                onDelete: product.canEdit
                    ? () => _deleteMedia(product.id, media.id)
                    : null,
              ),
            ),
            if (product.canEdit) ...[
              const SizedBox(height: 8),
              OutlinedButton.icon(
                onPressed: () => _pickAndUploadMedia(context, product),
                icon: const Icon(Icons.add_photo_alternate_outlined),
                label: const Text('Attach Media'),
              ),
            ],
          ],
        ),

        // Actions
        const SizedBox(height: 18),

        if (product.canEdit)
          _actionButton(
            label: 'Edit',
            color: context.statusColors.info,
            onPressed: () => _showEditDialog(context, product),
          ),

        if (product.canSubmit) ...[
          const SizedBox(height: 10),
          _actionButton(
            label: 'Submit for Review',
            color: context.statusColors.success,
            onPressed: () => _submit(product.id),
          ),
        ],

        if (product.canResubmit) ...[
          const SizedBox(height: 10),
          _actionButton(
            label: 'Resubmit for Review',
            color: context.statusColors.warning,
            onPressed: () => _resubmit(product.id),
          ),
        ],
      ],
    );
  }

  Widget _actionButton({
    required String label,
    required Color color,
    required VoidCallback onPressed,
  }) {
    return SizedBox(
      width: double.infinity,
      child: ElevatedButton(
        onPressed: _isSubmitting ? null : onPressed,
        style: ElevatedButton.styleFrom(backgroundColor: color),
        child: _isSubmitting ? const CircularProgressIndicator() : Text(label),
      ),
    );
  }

  Widget _buildDecisionReasonCard(ExternalProduct product) {
    final isRequestChanges =
        product.reviewStatus == ExternalProductReviewStatus.requestChanges;
    final titleText = isRequestChanges ? 'Perlu Perbaikan' : 'Alasan Penolakan';
    final borderColor = isRequestChanges
        ? context.statusColors.warning
        : Theme.of(context).colorScheme.primary;
    final bgColor = isRequestChanges
        ? context.statusColors.warning.withValues(alpha: 0.05)
        : Theme.of(context).colorScheme.primary.withValues(alpha: 0.05);
    final textColor = isRequestChanges
        ? context.statusColors.warning
        : Theme.of(context).colorScheme.primary;

    return Container(
      padding: const EdgeInsets.all(AppMetrics.p16),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(AppShape.r12),
        border: Border.all(color: borderColor.withValues(alpha: 0.3)),
        color: bgColor,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            titleText,
            style: context.typeRoles.titleSection.copyWith(
              fontWeight: FontWeight.w700,
              color: textColor,
            ),
          ),
          const SizedBox(height: 6),
          Text(product.rejectionReason!),
        ],
      ),
    );
  }

  Future<void> _submit(String productId) async {
    setState(() => _isSubmitting = true);
    final controller = ref.read(externalProductControllerProvider);
    final result = await controller.submitExternalProduct(id: productId);
    _finishMutation(result.isSuccess, result.error ?? 'Submit failed');
  }

  Future<void> _resubmit(String productId) async {
    setState(() => _isSubmitting = true);
    final controller = ref.read(externalProductControllerProvider);
    final result = await controller.resubmitExternalProduct(id: productId);
    _finishMutation(result.isSuccess, result.error ?? 'Resubmit failed');
  }

  Future<void> _deleteMedia(String productId, String mediaId) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Delete Media'),
        content: const Text('Are you sure you want to delete this media?'),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: const Text('Cancel'),
          ),
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(true),
            child: Text(
              'Delete',
              style: TextStyle(color: Theme.of(context).colorScheme.primary),
            ),
          ),
        ],
      ),
    );

    if (confirmed != true || !mounted) return;
    setState(() => _isSubmitting = true);
    final controller = ref.read(externalProductControllerProvider);
    final result = await controller.deleteExternalProductMedia(
      externalProductId: productId,
      mediaId: mediaId,
    );
    _finishMutation(result.isSuccess, result.error ?? 'Delete failed');
  }

  Future<void> _showEditDialog(
    BuildContext context,
    ExternalProduct product,
  ) async {
    final titleCtrl = TextEditingController(text: product.title);
    final urlCtrl = TextEditingController(text: product.externalUrl);
    final descCtrl = TextEditingController(text: product.description ?? '');

    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('Edit External Product'),
        content: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              TextField(
                controller: titleCtrl,
                decoration: const InputDecoration(labelText: 'Title'),
              ),
              const SizedBox(height: 8),
              TextField(
                controller: urlCtrl,
                decoration: const InputDecoration(labelText: 'External URL'),
              ),
              const SizedBox(height: 8),
              TextField(
                controller: descCtrl,
                decoration: const InputDecoration(
                  labelText: 'Description (optional)',
                ),
                maxLines: 2,
              ),
              if (product.reviewStatus == ExternalProductReviewStatus.approved)
                Padding(
                  padding: EdgeInsets.only(top: AppMetrics.p8),
                  child: Text(
                    'Editing an approved product will return it to pending review.',
                    style: context.typeRoles.labelMicro.copyWith(
                      color: context.statusColors.warning,
                    ),
                  ),
                ),
            ],
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(dialogContext).pop(false),
            child: const Text('Cancel'),
          ),
          ElevatedButton(
            onPressed: () => Navigator.of(dialogContext).pop(true),
            child: const Text('Save'),
          ),
        ],
      ),
    );

    if (confirmed != true || !mounted) return;

    final newTitle = titleCtrl.text.trim();
    final newUrl = urlCtrl.text.trim();
    final newDesc = descCtrl.text.trim();

    setState(() => _isSubmitting = true);
    final controller = ref.read(externalProductControllerProvider);
    final result = await controller.updateExternalProduct(
      id: product.id,
      title: newTitle != product.title ? newTitle : null,
      externalUrl: newUrl != product.externalUrl ? newUrl : null,
      description: newDesc != (product.description ?? '') ? newDesc : null,
    );

    titleCtrl.dispose();
    urlCtrl.dispose();
    descCtrl.dispose();

    _finishMutation(result.isSuccess, result.error ?? 'Update failed');
  }

  Future<void> _pickAndUploadMedia(
    BuildContext context,
    ExternalProduct product,
  ) async {
    // Warn before touching an approved product (upload triggers re-review)
    if (product.reviewStatus == ExternalProductReviewStatus.approved) {
      final confirmed = await showDialog<bool>(
        context: context,
        builder: (ctx) => AlertDialog(
          title: const Text('Add Media to Approved Product'),
          content: Text(
            'Adding media to an approved product will return it to pending review.',
            style: context.typeRoles.bodyDense.copyWith(
              color: context.statusColors.warning,
            ),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.of(ctx).pop(false),
              child: const Text('Cancel'),
            ),
            ElevatedButton(
              onPressed: () => Navigator.of(ctx).pop(true),
              child: const Text('Continue'),
            ),
          ],
        ),
      );
      if (confirmed != true || !context.mounted) return;
    }

    // Media type selection — canonical action sheet.
    final mediaType = await AppBottomSheetActions.showActions<String>(
      context: context,
      title: 'Add Media',
      actions: [
        BottomSheetAction<String>(
          title: 'Image',
          icon: Icons.image_outlined,
          onPressed: () => Navigator.of(context).pop('image'),
        ),
        BottomSheetAction<String>(
          title: 'Video',
          icon: Icons.videocam_outlined,
          onPressed: () => Navigator.of(context).pop('video'),
        ),
      ],
    );
    if (mediaType == null || !context.mounted) return;

    // File picker — same engine as every surface (typed single attach is
    // the legitimate variant here; picking mechanics + MB caps are shared).
    File? file;
    if (mediaType == 'image') {
      final photos = await MediaUploadOrchestrator.pickGalleryImages(
        context: context,
        maxAssets: 1,
      );
      if (photos.isEmpty || !context.mounted) return;
      file = File(photos.first.path);
    } else {
      final video = await MediaUploadOrchestrator.pickGalleryVideo(
        context: context,
      );
      if (video == null || !context.mounted) return;
      file = File(video.path);
    }

    // S3 upload — returns both object key and CDN URL
    setState(() => _isSubmitting = true);
    final s3 = ref.read(s3ServiceProvider);
    final Result<S3UploadResult> uploadResult;
    if (mediaType == 'image') {
      uploadResult = await s3.uploadImageWithMeta(
        file,
        folder: MediaUploadConfig.forCommerce.imageFolder,
      );
    } else {
      uploadResult = await s3.uploadVideoWithMeta(
        file,
        folder: MediaUploadConfig.forCommerce.videoFolder,
      );
    }

    if (!context.mounted) return;
    if (!uploadResult.isSuccess) {
      setState(() => _isSubmitting = false);
      AppSnackBar.showError(context, uploadResult.error ?? 'Gagal mengunggah');
      return;
    }

    // Attach uploaded media to external product
    // storageKey = namespaced S3 object key (e.g. images/commerce/1234_photo.jpg)
    // url = public CDN URL for display
    final controller = ref.read(externalProductControllerProvider);
    final result = await controller.attachExternalProductMedia(
      externalProductId: product.id,
      mediaType: mediaType,
      storageKey: uploadResult.data!.key,
      url: uploadResult.data!.url,
    );
    _finishMutation(result.isSuccess, result.error ?? 'Attach failed');
  }

  void _finishMutation(bool success, String errorText) {
    if (!mounted) return;
    setState(() => _isSubmitting = false);
    if (success) {
      ref.invalidate(externalProductDetailProvider(widget.productId));
      ref.invalidate(myExternalProductsProvider);
      AppSnackBar.showSuccess(context, 'Berhasil');
      return;
    }
    AppSnackBar.showError(context, errorText);
  }

  static String _reviewStatusLabel(ExternalProductReviewStatus status) {
    return switch (status) {
      ExternalProductReviewStatus.draft => 'Draft',
      ExternalProductReviewStatus.pendingReview => 'Menunggu Review',
      ExternalProductReviewStatus.approved => 'Disetujui',
      ExternalProductReviewStatus.rejected => 'Ditolak',
      ExternalProductReviewStatus.requestChanges => 'Perlu Perbaikan',
      ExternalProductReviewStatus.hidden => 'Disembunyikan',
    };
  }

  static Widget _kv(String key, String value) {
    return Padding(
      padding: const EdgeInsets.only(bottom: AppMetrics.p8),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Expanded(flex: 2, child: Text(key)),
          Expanded(
            flex: 3,
            child: Text(
              value,
              style: const TextStyle(fontWeight: FontWeight.w600),
            ),
          ),
        ],
      ),
    );
  }
}

class _SectionCard extends StatelessWidget {
  final String title;
  final List<Widget> children;

  const _SectionCard({required this.title, required this.children});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(AppMetrics.p16),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(AppShape.r12),
        border: Border.all(color: Theme.of(context).colorScheme.outlineVariant),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            title,
            style: context.typeRoles.titleSection.copyWith(
              fontWeight: FontWeight.w700,
            ),
          ),
          const SizedBox(height: 10),
          ...children,
        ],
      ),
    );
  }
}

class _MediaRow extends StatelessWidget {
  final ExternalProductMedia media;
  final VoidCallback? onDelete;

  const _MediaRow({required this.media, this.onDelete});

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.only(bottom: AppMetrics.p8),
      padding: const EdgeInsets.all(AppMetrics.p12),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(AppShape.r8),
        color: Theme.of(context).colorScheme.surfaceContainer,
      ),
      child: Row(
        children: [
          Container(
            width: 48,
            height: 48,
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(AppShape.r6),
              color: Theme.of(context).colorScheme.surfaceContainer,
            ),
            clipBehavior: Clip.antiAlias,
            child: media.mediaType == 'image'
                ? AppImage(
                    imageUrl: media.thumbnailUrl ?? media.url,
                    fit: BoxFit.cover,
                    errorWidget: const Icon(Icons.broken_image_outlined),
                  )
                : const Icon(Icons.videocam_outlined),
          ),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  media.mediaType,
                  style: const TextStyle(fontWeight: FontWeight.w600),
                ),
                Text(
                  media.url,
                  style: context.typeRoles.labelMicro.copyWith(
                    color: Theme.of(context).colorScheme.onSurfaceVariant,
                  ),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
              ],
            ),
          ),
          if (onDelete != null)
            IconButton(
              onPressed: onDelete,
              icon: const Icon(Icons.delete_outline, size: AppIconSize.action, semanticLabel: 'Hapus'),
              color: Theme.of(context).colorScheme.primary,
            ),
        ],
      ),
    );
  }
}
