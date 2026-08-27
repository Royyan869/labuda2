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
import 'package:labuda/core/core.dart' show AppColors;
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
  final ImagePicker _picker = ImagePicker();

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
    try {
      final video = await _picker.pickVideo(
        source: ImageSource.gallery,
        maxDuration: const Duration(minutes: 2),
      );

      if (video != null && mounted) {
        setState(() => _unboxingVideo = video);
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('Gagal memilih video: $e')));
      }
    }
  }

  Future<void> _pickPhotos() async {
    try {
      final photos = await _picker.pickMultiImage(
        imageQuality: 80,
        maxWidth: 1920,
        maxHeight: 1920,
      );

      if (photos.isNotEmpty && mounted) {
        setState(() {
          // Limit to 5 photos max
          _evidencePhotos = [..._evidencePhotos, ...photos].take(5).toList();
        });
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('Gagal memilih foto: $e')));
      }
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
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;

    return Dialog(
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
      child: Container(
        constraints: const BoxConstraints(maxWidth: 500, maxHeight: 700),
        decoration: BoxDecoration(
          color: isDark ? const Color(0xFF1E1E1E) : Colors.white,
          borderRadius: BorderRadius.circular(16),
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            // Header
            _buildHeader(isDark),

            // Content
            Expanded(
              child: SingleChildScrollView(
                padding: const EdgeInsets.all(20),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    // Reason selection
                    _buildReasonSection(isDark),
                    const SizedBox(height: 20),

                    // Description
                    _buildDescriptionSection(isDark),
                    const SizedBox(height: 20),

                    // Video upload (REQUIRED)
                    _buildVideoSection(isDark),
                    const SizedBox(height: 20),

                    // Photo upload (OPTIONAL)
                    _buildPhotoSection(isDark),
                    const SizedBox(height: 20),
                  ],
                ),
              ),
            ),

            // Footer
            _buildFooter(isDark, theme),
          ],
        ),
      ),
    );
  }

  Widget _buildHeader(bool isDark) {
    return Container(
      padding: const EdgeInsets.all(20),
      decoration: BoxDecoration(
        color: isDark ? const Color(0xFF2A2A2A) : Colors.grey[100],
        borderRadius: const BorderRadius.only(
          topLeft: Radius.circular(16),
          topRight: Radius.circular(16),
        ),
      ),
      child: Row(
        children: [
          Icon(Icons.currency_exchange, color: AppColors.primaryRed, size: 24),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Ajukan Pengembalian',
                  style: TextStyle(
                    fontSize: 18,
                    fontWeight: FontWeight.bold,
                    color: isDark ? Colors.white : Colors.black87,
                  ),
                ),
                Text(
                  'Order #${widget.orderId.substring(0, 8)}...',
                  style: TextStyle(fontSize: 12, color: Colors.grey[600]),
                ),
              ],
            ),
          ),
          IconButton(
            icon: const Icon(Icons.close),
            onPressed: widget.onCancel,
            color: isDark ? Colors.white : Colors.black54,
          ),
        ],
      ),
    );
  }

  Widget _buildReasonSection(bool isDark) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Text(
              'Alasan Pengembalian',
              style: TextStyle(
                fontSize: 16,
                fontWeight: FontWeight.w600,
                color: isDark ? Colors.white : Colors.black87,
              ),
            ),
            const Text(' *', style: TextStyle(color: Colors.red, fontSize: 16)),
          ],
        ),
        const SizedBox(height: 12),
        Wrap(
          spacing: 8,
          runSpacing: 8,
          children: RefundReason.values.map((reason) {
            final isSelected = _selectedReason == reason;
            return ChoiceChip(
              label: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text(reason.emoji, style: const TextStyle(fontSize: 14)),
                  const SizedBox(width: 6),
                  Text(
                    reason.displayName,
                    style: const TextStyle(fontSize: 12),
                  ),
                ],
              ),
              selected: isSelected,
              onSelected: (selected) {
                setState(() {
                  _selectedReason = selected ? reason : null;
                });
              },
              selectedColor: AppColors.primaryRed.withValues(alpha: 0.15),
              backgroundColor: isDark ? Colors.grey[850] : Colors.grey[100],
              labelStyle: TextStyle(
                color: isSelected
                    ? AppColors.primaryRed
                    : (isDark ? Colors.white70 : Colors.black87),
                fontWeight: isSelected ? FontWeight.w600 : FontWeight.normal,
              ),
              side: BorderSide(
                color: isSelected ? AppColors.primaryRed : Colors.transparent,
                width: 1.5,
              ),
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(8),
              ),
            );
          }).toList(),
        ),
      ],
    );
  }

  Widget _buildDescriptionSection(bool isDark) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'Deskripsi Tambahan',
          style: TextStyle(
            fontSize: 16,
            fontWeight: FontWeight.w600,
            color: isDark ? Colors.white : Colors.black87,
          ),
        ),
        const SizedBox(height: 8),
        TextField(
          controller: _descController,
          maxLines: 3,
          maxLength: 500,
          style: TextStyle(color: isDark ? Colors.white : Colors.black87),
          decoration: InputDecoration(
            hintText: 'Jelaskan detail masalah Anda...',
            hintStyle: TextStyle(
              color: isDark ? Colors.white38 : Colors.black38,
            ),
            border: OutlineInputBorder(borderRadius: BorderRadius.circular(12)),
            filled: true,
            fillColor: isDark ? Colors.grey[850] : Colors.grey[50],
          ),
        ),
      ],
    );
  }

  Widget _buildVideoSection(bool isDark) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Text(
              'Video Unboxing',
              style: TextStyle(
                fontSize: 16,
                fontWeight: FontWeight.w600,
                color: isDark ? Colors.white : Colors.black87,
              ),
            ),
            const Text(' *', style: TextStyle(color: Colors.red, fontSize: 16)),
            const SizedBox(width: 8),
            Icon(Icons.info_outline, size: 16, color: Colors.grey[600]),
          ],
        ),
        const SizedBox(height: 4),
        Text(
          'Wajib unggah video unboxing untuk bukti',
          style: TextStyle(fontSize: 12, color: Colors.grey[600]),
        ),
        const SizedBox(height: 12),

        if (_unboxingVideo != null)
          _buildVideoPreview(isDark)
        else
          _buildUploadButton(
            isDark: isDark,
            label: 'Pilih Video',
            icon: Icons.videocam_outlined,
            color: Colors.blue,
            onTap: _pickVideo,
          ),
      ],
    );
  }

  Widget _buildVideoPreview(bool isDark) {
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: isDark ? Colors.grey[850] : Colors.grey[100],
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: Colors.blue.withValues(alpha: 0.3)),
      ),
      child: Row(
        children: [
          Container(
            width: 48,
            height: 48,
            decoration: BoxDecoration(
              color: Colors.blue.withValues(alpha: 0.1),
              borderRadius: BorderRadius.circular(8),
            ),
            child: const Icon(Icons.play_arrow, color: Colors.blue),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  _unboxingVideo!.name.split('/').last,
                  style: TextStyle(
                    fontSize: 14,
                    fontWeight: FontWeight.w500,
                    color: isDark ? Colors.white : Colors.black87,
                  ),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
                Text(
                  'Video unboxing',
                  style: TextStyle(fontSize: 12, color: Colors.grey[600]),
                ),
              ],
            ),
          ),
          IconButton(
            icon: const Icon(Icons.close, color: Colors.red),
            onPressed: () => setState(() => _unboxingVideo = null),
          ),
        ],
      ),
    );
  }

  Widget _buildPhotoSection(bool isDark) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Text(
              'Foto Bukti',
              style: TextStyle(
                fontSize: 16,
                fontWeight: FontWeight.w600,
                color: isDark ? Colors.white : Colors.black87,
              ),
            ),
            const SizedBox(width: 8),
            Text(
              '(Opsional)',
              style: TextStyle(fontSize: 12, color: Colors.grey[600]),
            ),
          ],
        ),
        const SizedBox(height: 12),

        if (_evidencePhotos.isNotEmpty) ...[
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: List.generate(_evidencePhotos.length, (index) {
              return _buildPhotoThumbnail(index, isDark);
            }),
          ),
          if (_evidencePhotos.length < 5) ...[
            const SizedBox(height: 8),
            _buildUploadButton(
              isDark: isDark,
              label: 'Tambah Foto',
              icon: Icons.add_photo_alternate_outlined,
              color: Colors.green,
              onTap: _pickPhotos,
            ),
          ],
        ] else
          _buildUploadButton(
            isDark: isDark,
            label: 'Pilih Foto',
            icon: Icons.photo_library_outlined,
            color: Colors.green,
            onTap: _pickPhotos,
          ),
      ],
    );
  }

  Widget _buildPhotoThumbnail(int index, bool isDark) {
    return Stack(
      children: [
        ClipRRect(
          borderRadius: BorderRadius.circular(8),
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
              padding: const EdgeInsets.all(4),
              decoration: const BoxDecoration(
                color: Colors.red,
                shape: BoxShape.circle,
              ),
              child: const Icon(Icons.close, color: Colors.white, size: 14),
            ),
          ),
        ),
      ],
    );
  }

  Widget _buildUploadButton({
    required bool isDark,
    required String label,
    required IconData icon,
    required Color color,
    required VoidCallback onTap,
  }) {
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(12),
      child: Container(
        padding: const EdgeInsets.all(16),
        decoration: BoxDecoration(
          color: color.withValues(alpha: 0.1),
          borderRadius: BorderRadius.circular(12),
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

  Widget _buildFooter(bool isDark, ThemeData theme) {
    return Container(
      padding: const EdgeInsets.all(20),
      decoration: BoxDecoration(
        color: isDark ? const Color(0xFF2A2A2A) : Colors.grey[100],
        borderRadius: const BorderRadius.only(
          bottomLeft: Radius.circular(16),
          bottomRight: Radius.circular(16),
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
                  foregroundColor: isDark ? Colors.white70 : Colors.black54,
                  side: BorderSide(
                    color: isDark ? Colors.grey[700]! : Colors.grey[300]!,
                  ),
                  padding: const EdgeInsets.symmetric(vertical: 14),
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(10),
                  ),
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
                  backgroundColor: AppColors.primaryRed,
                  foregroundColor: Colors.white,
                  disabledBackgroundColor: Colors.grey,
                  padding: const EdgeInsets.symmetric(vertical: 14),
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(10),
                  ),
                ),
                child: _isSubmitting
                    ? const SizedBox(
                        height: 20,
                        width: 20,
                        child: CircularProgressIndicator(
                          strokeWidth: 2,
                          valueColor: AlwaysStoppedAnimation<Color>(
                            Colors.white,
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
