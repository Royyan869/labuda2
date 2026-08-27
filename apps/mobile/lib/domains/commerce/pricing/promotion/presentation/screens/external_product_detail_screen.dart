library;

import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:intl/intl.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/external_product.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/external_product_media.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/external_product_review_status.dart';
import 'package:labuda/domains/commerce/pricing/promotion/presentation/providers/promotion_providers.dart';
import 'package:labuda/shared/ui/src/helpers/media_picker_helper.dart';

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
      padding: const EdgeInsets.all(16),
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
            _kv('Created', _dateTime(product.createdAt)),
            _kv('Updated', _dateTime(product.updatedAt)),
            if (product.submittedAt != null)
              _kv('Submitted', _dateTime(product.submittedAt!)),
            if (product.approvedAt != null)
              _kv('Approved', _dateTime(product.approvedAt!)),
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
                padding: EdgeInsets.symmetric(vertical: 8),
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
            color: Colors.blue,
            onPressed: () => _showEditDialog(context, product),
          ),

        if (product.canSubmit) ...[
          const SizedBox(height: 10),
          _actionButton(
            label: 'Submit for Review',
            color: AppColors.successGreen,
            onPressed: () => _submit(product.id),
          ),
        ],

        if (product.canResubmit) ...[
          const SizedBox(height: 10),
          _actionButton(
            label: 'Resubmit for Review',
            color: Colors.orange,
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
        style: ElevatedButton.styleFrom(
          backgroundColor: color,
          foregroundColor: Colors.white,
        ),
        child: _isSubmitting ? const CircularProgressIndicator() : Text(label),
      ),
    );
  }

  Widget _buildDecisionReasonCard(ExternalProduct product) {
    final isRequestChanges =
        product.reviewStatus == ExternalProductReviewStatus.requestChanges;
    final titleText = isRequestChanges ? 'Perlu Perbaikan' : 'Alasan Penolakan';
    final borderColor = isRequestChanges ? Colors.orange : AppColors.primaryRed;
    final bgColor = isRequestChanges
        ? Colors.orange.withValues(alpha: 0.05)
        : AppColors.primaryRed.withValues(alpha: 0.05);
    final textColor = isRequestChanges ? Colors.orange : AppColors.primaryRed;

    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: borderColor.withValues(alpha: 0.3)),
        color: bgColor,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            titleText,
            style: TextStyle(
              fontSize: 14,
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
    final controller = ref.read(promotionControllerProvider);
    final result = await controller.submitExternalProduct(id: productId);
    _finishMutation(result.isSuccess, result.error ?? 'Submit failed');
  }

  Future<void> _resubmit(String productId) async {
    setState(() => _isSubmitting = true);
    final controller = ref.read(promotionControllerProvider);
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
            child: const Text(
              'Delete',
              style: TextStyle(color: AppColors.primaryRed),
            ),
          ),
        ],
      ),
    );

    if (confirmed != true || !mounted) return;
    setState(() => _isSubmitting = true);
    final controller = ref.read(promotionControllerProvider);
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
                const Padding(
                  padding: EdgeInsets.only(top: 8),
                  child: Text(
                    'Editing an approved product will return it to pending review.',
                    style: TextStyle(fontSize: 12, color: Colors.orange),
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
    final controller = ref.read(promotionControllerProvider);
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
          content: const Text(
            'Adding media to an approved product will return it to pending review.',
            style: TextStyle(fontSize: 13, color: Colors.orange),
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

    // Media type selection via bottom sheet
    final mediaType = await showModalBottomSheet<String>(
      context: context,
      builder: (ctx) => SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            ListTile(
              leading: const Icon(Icons.image_outlined),
              title: const Text('Image'),
              onTap: () => Navigator.of(ctx).pop('image'),
            ),
            ListTile(
              leading: const Icon(Icons.videocam_outlined),
              title: const Text('Video'),
              onTap: () => Navigator.of(ctx).pop('video'),
            ),
          ],
        ),
      ),
    );
    if (mediaType == null || !context.mounted) return;

    // File picker
    final List<String>? paths;
    if (mediaType == 'image') {
      paths = await MediaPickerHelper.pickPhotos(
        context: context,
        maxAssets: 1,
      );
    } else {
      paths = await MediaPickerHelper.pickVideos(
        context: context,
        maxAssets: 1,
      );
    }
    if (paths == null || paths.isEmpty || !context.mounted) return;

    // S3 upload — returns both object key and CDN URL
    setState(() => _isSubmitting = true);
    final s3 = ref.read(s3ServiceProvider);
    final file = File(paths.first);
    final Result<S3UploadResult> uploadResult;
    if (mediaType == 'image') {
      uploadResult = await s3.uploadImageWithMeta(file);
    } else {
      uploadResult = await s3.uploadVideoWithMeta(file);
    }

    if (!context.mounted) return;
    if (!uploadResult.isSuccess) {
      setState(() => _isSubmitting = false);
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(uploadResult.error ?? 'Upload failed'),
          backgroundColor: AppColors.primaryRed,
        ),
      );
      return;
    }

    // Attach uploaded media to external product
    // storageKey = raw S3 object key (e.g. images/1234_photo.jpg)
    // url = public CDN URL for display
    final controller = ref.read(promotionControllerProvider);
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
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(const SnackBar(content: Text('Success')));
      return;
    }
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(errorText), backgroundColor: AppColors.primaryRed),
    );
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
      padding: const EdgeInsets.only(bottom: 8),
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

  static String _dateTime(DateTime d) =>
      DateFormat('dd MMM yyyy, HH:mm').format(d);
}

class _SectionCard extends StatelessWidget {
  final String title;
  final List<Widget> children;

  const _SectionCard({required this.title, required this.children});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: AppColors.neutralGray200),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            title,
            style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w700),
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
      margin: const EdgeInsets.only(bottom: 8),
      padding: const EdgeInsets.all(10),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(8),
        color: AppColors.neutralGray50,
      ),
      child: Row(
        children: [
          Container(
            width: 48,
            height: 48,
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(6),
              color: AppColors.neutralGray100,
            ),
            clipBehavior: Clip.antiAlias,
            child: media.mediaType == 'image'
                ? Image.network(
                    media.thumbnailUrl ?? media.url,
                    fit: BoxFit.cover,
                    errorBuilder: (_, _, _) =>
                        const Icon(Icons.broken_image_outlined),
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
                  style: TextStyle(
                    fontSize: 11,
                    color: AppColors.neutralGray600,
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
              icon: const Icon(Icons.delete_outline, size: 20),
              color: AppColors.primaryRed,
            ),
        ],
      ),
    );
  }
}
