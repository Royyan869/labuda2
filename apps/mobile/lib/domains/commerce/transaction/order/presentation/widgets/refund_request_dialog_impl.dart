library;

/// Refund Request Dialog - Full implementation for trust flow
///
/// This dialog allows buyers to submit refund requests with:
/// - Reason selection (categorized with emoji icons)
/// - Optional description
/// - REQUIRED video upload (unboxing video)
/// - OPTIONAL photo evidence upload

import 'dart:io';
import 'package:flutter/material.dart';
import 'package:image_picker/image_picker.dart';
import 'package:labuda/core/src/theme/app_theme.dart';
import 'package:labuda/core/media/media_upload_orchestrator.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/entities/refund_request.dart';

class RefundRequestDialog extends StatefulWidget {
  final String orderId;
  final VoidCallback onCancel;
  final Function(
    RefundReason reason,
    String? description,
    XFile? unboxingVideo,
    List<XFile> evidencePhotos,
  )
  onSubmit;

  const RefundRequestDialog({
    super.key,
    required this.orderId,
    required this.onCancel,
    required this.onSubmit,
  });

  @override
  State<RefundRequestDialog> createState() => _RefundRequestDialogState();
}

class _RefundRequestDialogState extends State<RefundRequestDialog> {
  RefundReason? _selectedReason;
  final _descController = TextEditingController();

  XFile? _unboxingVideo;
  List<XFile> _evidencePhotos = [];
  bool _isSubmitting = false;

  @override
  void dispose() {
    _descController.dispose();
    super.dispose();
  }

  bool get _canSubmit {
    return _selectedReason != null && _unboxingVideo != null && !_isSubmitting;
  }

  Future<void> _pickVideo() async {
    final video = await MediaUploadOrchestrator.pickGalleryVideo(
      context: context,
    );
    if (video != null && mounted) {
      setState(() => _unboxingVideo = video);
    }
  }

  Future<void> _pickPhotos() async {
    final photos = await MediaUploadOrchestrator.pickGalleryImages(
      context: context,
      maxAssets: 5,
    );
    if (photos.isNotEmpty && mounted) {
      setState(() {
        // Limit to 5 photos max
        _evidencePhotos = [..._evidencePhotos, ...photos].take(5).toList();
      });
    }
  }

  void _removePhoto(int index) {
    setState(() {
      _evidencePhotos.removeAt(index);
    });
  }

  void _handleSubmit() {
    if (!_canSubmit) return;

    setState(() => _isSubmitting = true);

    widget.onSubmit(
      _selectedReason!,
      _descController.text.trim().isEmpty ? null : _descController.text.trim(),
      _unboxingVideo,
      _evidencePhotos,
    );
  }

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;

    return Dialog(
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(AppShape.r16)),
      child: Container(
        constraints: const BoxConstraints(maxWidth: 500, maxHeight: 700),
        decoration: BoxDecoration(
          color: colorScheme.surface,
          borderRadius: BorderRadius.circular(AppShape.r16),
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            // Header
            _buildHeader(colorScheme),

            // Content
            Expanded(
              child: SingleChildScrollView(
                padding: const EdgeInsets.all(AppMetrics.p20),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    // Reason selection
                    _buildReasonSection(colorScheme),
                    const SizedBox(height: 20),

                    // Description
                    _buildDescriptionSection(colorScheme),
                    const SizedBox(height: 20),

                    // Video upload (REQUIRED)
                    _buildVideoSection(colorScheme),
                    const SizedBox(height: 20),

                    // Photo upload (OPTIONAL)
                    _buildPhotoSection(colorScheme),
                    const SizedBox(height: 20),
                  ],
                ),
              ),
            ),

            // Footer
            _buildFooter(colorScheme),
          ],
        ),
      ),
    );
  }

  Widget _buildHeader(ColorScheme colorScheme) {
    return Container(
      padding: const EdgeInsets.all(AppMetrics.p20),
      decoration: BoxDecoration(
        color: colorScheme.surfaceContainerHigh,
        borderRadius: const BorderRadius.only(
          topLeft: Radius.circular(AppShape.r16),
          topRight: Radius.circular(AppShape.r16),
        ),
      ),
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Ajukan Refund',
                  style: TextStyle(
                    fontSize: AppType.s18,
                    fontWeight: FontWeight.bold,
                    color: colorScheme.onSurface,
                  ),
                ),
                Text(
                  'Order #${widget.orderId.substring(0, 8)}...',
                  style: TextStyle(
                    fontSize: AppType.s12,
                    color: colorScheme.onSurfaceVariant,
                  ),
                ),
              ],
            ),
          ),
          IconButton(
            icon: const Icon(Icons.close),
            onPressed: widget.onCancel,
            color: colorScheme.onSurfaceVariant,
          ),
        ],
      ),
    );
  }

  Widget _buildReasonSection(ColorScheme colorScheme) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Text(
              'Alasan Refund',
              style: TextStyle(
                fontSize: AppType.s16,
                fontWeight: FontWeight.w600,
                color: colorScheme.onSurface,
              ),
            ),
            Text(
              ' *',
              style: TextStyle(color: colorScheme.error, fontSize: AppType.s16),
            ),
          ],
        ),
        const SizedBox(height: 12),
        Wrap(
          spacing: 8,
          runSpacing: 8,
          children: RefundReason.values.map((reason) {
            final isSelected = _selectedReason == reason;
            return ChoiceChip(
              label: Text('${reason.emoji} ${reason.displayName}'),
              selected: isSelected,
              onSelected: (_) {
                setState(() => _selectedReason = reason);
              },
              selectedColor: colorScheme.primaryContainer,
              backgroundColor: colorScheme.surfaceContainerHigh,
              labelStyle: TextStyle(
                color: isSelected
                    ? colorScheme.onPrimaryContainer
                    : colorScheme.onSurfaceVariant,
                fontWeight: isSelected ? FontWeight.w600 : FontWeight.normal,
              ),
              side: BorderSide(
                color: isSelected ? colorScheme.primary : Colors.transparent,
                width: 1.5,
              ),
              showCheckmark: false,
            );
          }).toList(),
        ),
      ],
    );
  }

  Widget _buildDescriptionSection(ColorScheme colorScheme) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'Deskripsi Tambahan',
          style: TextStyle(
            fontSize: AppType.s16,
            fontWeight: FontWeight.w600,
            color: colorScheme.onSurface,
          ),
        ),
        const SizedBox(height: 8),
        TextField(
          controller: _descController,
          maxLines: 3,
          maxLength: 500,
          style: TextStyle(color: colorScheme.onSurface),
          decoration: InputDecoration(
            hintText: 'Jelaskan detail masalah Anda...',
            hintStyle: TextStyle(color: colorScheme.onSurfaceVariant),
            border: OutlineInputBorder(borderRadius: BorderRadius.circular(AppShape.r12)),
            filled: true,
            fillColor: colorScheme.surfaceContainerHigh,
          ),
        ),
      ],
    );
  }

  Widget _buildVideoSection(ColorScheme colorScheme) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Text(
              'Video Unboxing',
              style: TextStyle(
                fontSize: AppType.s16,
                fontWeight: FontWeight.w600,
                color: colorScheme.onSurface,
              ),
            ),
            Text(
              ' *',
              style: TextStyle(color: colorScheme.error, fontSize: AppType.s16),
            ),
            const SizedBox(width: 8),
            Icon(
              Icons.info_outline,
              size: 16,
              color: colorScheme.onSurfaceVariant,
            ),
          ],
        ),
        const SizedBox(height: 4),
        Text(
          'Wajib unggah video unboxing untuk bukti',
          style: TextStyle(fontSize: AppType.s12, color: colorScheme.onSurfaceVariant),
        ),
        const SizedBox(height: 12),

        if (_unboxingVideo != null)
          _buildVideoPreview(colorScheme)
        else
          _buildUploadButton(
            label: 'Pilih Video',
            icon: Icons.videocam_outlined,
            color: colorScheme.secondary,
            onTap: _pickVideo,
          ),
      ],
    );
  }

  Widget _buildVideoPreview(ColorScheme colorScheme) {
    return Container(
      padding: const EdgeInsets.all(AppMetrics.p12),
      decoration: BoxDecoration(
        color: colorScheme.surfaceContainerHigh,
        borderRadius: BorderRadius.circular(AppShape.r12),
        border: Border.all(
          color: colorScheme.secondary.withValues(alpha: 0.3),
        ),
      ),
      child: Row(
        children: [
          Container(
            width: 48,
            height: 48,
            decoration: BoxDecoration(
              color: colorScheme.secondary.withValues(alpha: 0.1),
              borderRadius: BorderRadius.circular(AppShape.r8),
            ),
            child: Icon(Icons.play_arrow, color: colorScheme.secondary),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  _unboxingVideo!.name.split('/').last,
                  style: TextStyle(
                    fontSize: AppType.s14,
                    fontWeight: FontWeight.w500,
                    color: colorScheme.onSurface,
                  ),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
                Text(
                  'Video unboxing',
                  style: TextStyle(
                    fontSize: AppType.s12,
                    color: colorScheme.onSurfaceVariant,
                  ),
                ),
              ],
            ),
          ),
          IconButton(
            icon: Icon(Icons.close, color: colorScheme.error),
            onPressed: () => setState(() => _unboxingVideo = null),
          ),
        ],
      ),
    );
  }

  Widget _buildPhotoSection(ColorScheme colorScheme) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Text(
              'Foto Bukti',
              style: TextStyle(
                fontSize: AppType.s16,
                fontWeight: FontWeight.w600,
                color: colorScheme.onSurface,
              ),
            ),
            const SizedBox(width: 8),
            Text(
              '(Opsional)',
              style: TextStyle(
                fontSize: AppType.s12,
                color: colorScheme.onSurfaceVariant,
              ),
            ),
          ],
        ),
        const SizedBox(height: 12),

        if (_evidencePhotos.isNotEmpty) ...[
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: List.generate(_evidencePhotos.length, (index) {
              return _buildPhotoThumbnail(index, colorScheme);
            }),
          ),
          if (_evidencePhotos.length < 5) ...[
            const SizedBox(height: 8),
            _buildUploadButton(
              label: 'Tambah Foto',
              icon: Icons.add_photo_alternate_outlined,
              color: context.statusColors.success,
              onTap: _pickPhotos,
            ),
          ],
        ] else
          _buildUploadButton(
            label: 'Pilih Foto',
            icon: Icons.photo_library_outlined,
            color: context.statusColors.success,
            onTap: _pickPhotos,
          ),
      ],
    );
  }

  Widget _buildPhotoThumbnail(int index, ColorScheme colorScheme) {
    return Stack(
      children: [
        ClipRRect(
          borderRadius: BorderRadius.circular(AppShape.r8),
          child: Image.file(
            File(_evidencePhotos[index].path),
            width: 80,
            height: 80,
            fit: BoxFit.cover,
          ),
        ),
        Positioned(
          top: 4,
          right: 4,
          child: GestureDetector(
            onTap: () => _removePhoto(index),
            child: Container(
              padding: const EdgeInsets.all(AppMetrics.p4),
              decoration: BoxDecoration(
                color: colorScheme.error,
                shape: BoxShape.circle,
              ),
              child: Icon(
                Icons.close,
                color: colorScheme.onPrimary,
                size: 14,
              ),
            ),
          ),
        ),
      ],
    );
  }

  Widget _buildUploadButton({
    required String label,
    required IconData icon,
    required Color color,
    required VoidCallback onTap,
  }) {
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(AppShape.r12),
      child: Container(
        padding: const EdgeInsets.all(AppMetrics.p16),
        decoration: BoxDecoration(
          color: color.withValues(alpha: 0.1),
          borderRadius: BorderRadius.circular(AppShape.r12),
          border: Border.all(
            color: color.withValues(alpha: 0.3),
            style: BorderStyle.solid,
          ),
        ),
        child: Row(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(icon, color: color),
            const SizedBox(width: 8),
            Text(
              label,
              style: TextStyle(color: color, fontWeight: FontWeight.w600),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildFooter(ColorScheme colorScheme) {
    return Container(
      padding: const EdgeInsets.all(AppMetrics.p20),
      decoration: BoxDecoration(
        color: colorScheme.surfaceContainerHigh,
        borderRadius: const BorderRadius.only(
          bottomLeft: Radius.circular(AppShape.r16),
          bottomRight: Radius.circular(AppShape.r16),
        ),
      ),
      child: SafeArea(
        top: false,
        child: Row(
          children: [
            Expanded(
              child: OutlinedButton(
                onPressed: _isSubmitting ? null : widget.onCancel,
                style: OutlinedButton.styleFrom(
                  foregroundColor: colorScheme.onSurfaceVariant,
                  padding: const EdgeInsets.symmetric(vertical: AppMetrics.p14),
                ),
                child: const Text(
                  'Batal',
                  style: TextStyle(fontWeight: FontWeight.w600),
                ),
              ),
            ),
            const SizedBox(width: 12),
            Expanded(
              flex: 2,
              child: ElevatedButton(
                onPressed: _canSubmit ? _handleSubmit : null,
                style: ElevatedButton.styleFrom(
                  disabledBackgroundColor:
                      colorScheme.surfaceContainerHighest,
                  padding: const EdgeInsets.symmetric(vertical: AppMetrics.p14),
                ),
                child: _isSubmitting
                    ? SizedBox(
                        height: 20,
                        width: 20,
                        child: CircularProgressIndicator(
                          strokeWidth: 2,
                          valueColor: AlwaysStoppedAnimation<Color>(
                            colorScheme.onPrimary,
                          ),
                        ),
                      )
                    : const Text(
                        'Ajukan',
                        style: TextStyle(fontWeight: FontWeight.w600),
                      ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
