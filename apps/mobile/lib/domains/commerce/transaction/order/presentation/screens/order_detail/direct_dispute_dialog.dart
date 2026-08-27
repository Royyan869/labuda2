/// Direct Dispute Dialog - Allows buyer to open a dispute directly on a shipped order
library;

import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:image_picker/image_picker.dart';
import 'package:labuda/core/core.dart' as core;
import 'package:labuda/core/providers/core_providers.dart';
import 'package:labuda/domains/commerce/transaction/order/data/dto/dispute_dto.dart';
import 'package:labuda/domains/commerce/transaction/order/data/order_providers.dart';
import 'package:labuda/shared/widgets/app_snackbar.dart';

/// Dispute reason codes matching backend RefundReason values
/// (reused for dispute since they share the same domain reasons).
enum DisputeReasonCode {
  itemNotReceived('item_not_received', 'Barang tidak diterima'),
  itemNotAsDescribed('item_not_as_described', 'Barang tidak sesuai deskripsi'),
  itemDamaged('item_damaged', 'Barang rusak'),
  defectiveItem('defective_item', 'Barang cacat/tidak berfungsi'),
  wrongItem('wrong_item', 'Barang salah'),
  deliveryDelay('delivery_delay', 'Pengiriman terlambat'),
  other('other', 'Lainnya');

  const DisputeReasonCode(this.apiValue, this.displayName);
  final String apiValue;
  final String displayName;
}

/// Dialog for opening a dispute directly (not via refund escalation)
class DirectDisputeDialog extends ConsumerStatefulWidget {
  final String orderId;
  final VoidCallback? onDisputeOpened;

  const DirectDisputeDialog({
    super.key,
    required this.orderId,
    this.onDisputeOpened,
  });

  @override
  ConsumerState<DirectDisputeDialog> createState() =>
      _DirectDisputeDialogState();

  /// Show the direct dispute dialog
  static Future<void> show({
    required BuildContext context,
    required String orderId,
    VoidCallback? onDisputeOpened,
  }) {
    return showDialog(
      context: context,
      builder: (ctx) => DirectDisputeDialog(
        orderId: orderId,
        onDisputeOpened: onDisputeOpened,
      ),
    );
  }
}

class _DirectDisputeDialogState extends ConsumerState<DirectDisputeDialog> {
  final _descriptionController = TextEditingController();
  final _picker = ImagePicker();
  DisputeReasonCode? _selectedReason;
  XFile? _videoFile;
  final List<XFile> _photoFiles = [];
  bool _isSubmitting = false;

  @override
  void dispose() {
    _descriptionController.dispose();
    super.dispose();
  }

  Future<void> _pickVideo() async {
    try {
      final video = await _picker.pickVideo(
        source: ImageSource.gallery,
        maxDuration: const Duration(minutes: 2),
      );
      if (video != null) {
        setState(() {
          _videoFile = video;
        });
      }
    } catch (e) {
      if (mounted) {
        AppSnackBar.showError(context, 'Gagal memilih video. Coba lagi.');
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
      if (photos.isNotEmpty && _photoFiles.length + photos.length <= 5) {
        setState(() {
          _photoFiles.addAll(photos);
        });
      }
    } catch (e) {
      if (mounted) {
        AppSnackBar.showError(context, 'Gagal memilih foto. Coba lagi.');
      }
    }
  }

  Future<void> _submitDispute() async {
    if (_selectedReason == null) {
      AppSnackBar.showError(context, 'Pilih alasan sengketa');
      return;
    }

    if (_videoFile == null) {
      AppSnackBar.showError(
        context,
        'Bukti video wajib diunggah untuk mengajukan sengketa',
      );
      return;
    }

    setState(() {
      _isSubmitting = true;
    });

    try {
      final s3Service = ref.read(s3ServiceProvider);
      final evidenceUrls = <String>[];
      String? videoUrl;

      // Upload video (required)
      final videoResult = await s3Service.uploadVideo(File(_videoFile!.path));
      if (videoResult.isSuccess && videoResult.data != null) {
        videoUrl = videoResult.data!;
      } else {
        throw Exception('Video upload gagal: ${videoResult.error}');
      }

      // Upload photos (optional)
      for (final photo in _photoFiles) {
        final result = await s3Service.uploadImage(File(photo.path));
        if (result.isSuccess && result.data != null) {
          evidenceUrls.add(result.data!);
        }
      }

      final datasource = ref.read(orderApiDatasourceProvider);

      await datasource.createDispute(
        widget.orderId,
        CreateDisputeDto(
          reason: _selectedReason!.displayName,
          reasonCode: _selectedReason!.apiValue,
          description: _descriptionController.text.trim().isEmpty
              ? null
              : _descriptionController.text.trim(),
          videoUrl: videoUrl,
          evidenceUrls: evidenceUrls.isNotEmpty ? evidenceUrls : null,
        ),
      );

      if (mounted) {
        Navigator.of(context).pop();
        AppSnackBar.showSuccess(context, 'Sengketa berhasil diajukan');
        widget.onDisputeOpened?.call();
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _isSubmitting = false;
        });

        // Surface backend validation errors clearly
        final errorMsg = e.toString();
        String displayError;
        if (errorMsg.contains('video')) {
          displayError = 'Bukti video diperlukan untuk mengajukan sengketa.';
        } else if (errorMsg.contains('window') ||
            errorMsg.contains('expired')) {
          displayError = 'Batas waktu pengajuan sengketa telah berakhir.';
        } else if (errorMsg.contains('already has an active dispute')) {
          displayError = 'Pesanan ini sudah memiliki sengketa aktif.';
        } else if (errorMsg.contains('active refund')) {
          displayError =
              'Pesanan ini memiliki permintaan refund aktif. Tunggu keputusan penjual atau ajukan eskalasi setelah ditolak.';
        } else {
          displayError = 'Gagal mengajukan sengketa: $errorMsg';
        }
        AppSnackBar.showError(context, displayError);
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;

    return AlertDialog(
      title: Row(
        children: [
          Icon(
            Icons.report_problem_rounded,
            color: core.AppColors.statusWarning,
            size: 24,
          ),
          const SizedBox(width: 12),
          const Expanded(
            child: Text(
              'Buka Dispute',
              style: TextStyle(fontSize: 18, fontWeight: FontWeight.w600),
            ),
          ),
        ],
      ),
      content: SizedBox(
        width: double.maxFinite,
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // Info message
              Container(
                padding: const EdgeInsets.all(12),
                decoration: BoxDecoration(
                  color: core.AppColors.primaryBlue.withValues(alpha: 0.1),
                  borderRadius: BorderRadius.circular(8),
                  border: Border.all(
                    color: core.AppColors.primaryBlue.withValues(alpha: 0.3),
                  ),
                ),
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Icon(
                      Icons.info_outline_rounded,
                      color: core.AppColors.primaryBlue,
                      size: 18,
                    ),
                    const SizedBox(width: 8),
                    Expanded(
                      child: Text(
                        'Admin akan meninjau kasus ini secara adil berdasarkan bukti dari kedua pihak.',
                        style: TextStyle(
                          fontSize: 12,
                          color: isDark ? Colors.white70 : Colors.black87,
                        ),
                      ),
                    ),
                  ],
                ),
              ),
              const SizedBox(height: 16),

              // Reason selector
              Text(
                'Alasan Sengketa *',
                style: theme.textTheme.bodyMedium?.copyWith(
                  fontWeight: FontWeight.w500,
                ),
              ),
              const SizedBox(height: 8),
              DropdownButtonFormField<DisputeReasonCode>(
                initialValue: _selectedReason,
                decoration: InputDecoration(
                  hintText: 'Pilih alasan...',
                  border: OutlineInputBorder(
                    borderRadius: BorderRadius.circular(8),
                  ),
                  contentPadding: const EdgeInsets.symmetric(
                    horizontal: 12,
                    vertical: 12,
                  ),
                ),
                items: DisputeReasonCode.values
                    .map(
                      (reason) => DropdownMenuItem(
                        value: reason,
                        child: Text(
                          reason.displayName,
                          style: const TextStyle(fontSize: 14),
                        ),
                      ),
                    )
                    .toList(),
                onChanged: (value) {
                  setState(() {
                    _selectedReason = value;
                  });
                },
              ),
              const SizedBox(height: 16),

              // Description field
              Text(
                'Jelaskan Masalah',
                style: theme.textTheme.bodyMedium?.copyWith(
                  fontWeight: FontWeight.w500,
                ),
              ),
              const SizedBox(height: 8),
              TextField(
                controller: _descriptionController,
                maxLines: 3,
                maxLength: 500,
                decoration: InputDecoration(
                  hintText: 'Jelaskan secara detail masalah yang Anda alami...',
                  border: OutlineInputBorder(
                    borderRadius: BorderRadius.circular(8),
                  ),
                  filled: isDark,
                  fillColor: isDark
                      ? const Color(0xFF2A2A2A)
                      : Colors.grey[100],
                ),
              ),
              const SizedBox(height: 16),

              // Video upload (required)
              Text(
                'Bukti Video (Wajib) *',
                style: theme.textTheme.bodyMedium?.copyWith(
                  fontWeight: FontWeight.w500,
                ),
              ),
              const SizedBox(height: 8),
              if (_videoFile != null)
                Container(
                  padding: const EdgeInsets.all(8),
                  decoration: BoxDecoration(
                    color: isDark ? const Color(0xFF2A2A2A) : Colors.grey[100],
                    borderRadius: BorderRadius.circular(8),
                    border: Border.all(color: Colors.grey.shade300),
                  ),
                  child: Row(
                    children: [
                      Icon(
                        Icons.videocam_rounded,
                        color: core.AppColors.primaryGreen,
                        size: 20,
                      ),
                      const SizedBox(width: 8),
                      Expanded(
                        child: Text(
                          _videoFile!.name,
                          style: const TextStyle(fontSize: 12),
                          overflow: TextOverflow.ellipsis,
                        ),
                      ),
                      IconButton(
                        icon: const Icon(Icons.close, size: 18),
                        onPressed: () {
                          setState(() {
                            _videoFile = null;
                          });
                        },
                        padding: EdgeInsets.zero,
                        constraints: const BoxConstraints(),
                      ),
                    ],
                  ),
                )
              else
                OutlinedButton.icon(
                  onPressed: _isSubmitting ? null : _pickVideo,
                  icon: const Icon(Icons.videocam_rounded, size: 18),
                  label: const Text('Pilih Video Bukti'),
                  style: OutlinedButton.styleFrom(
                    minimumSize: const Size(double.infinity, 44),
                  ),
                ),
              const SizedBox(height: 4),
              Text(
                'Rekam video unboxing atau bukti masalah (maks. 2 menit)',
                style: TextStyle(fontSize: 11, color: Colors.grey[600]),
              ),
              const SizedBox(height: 16),

              // Photo upload (optional)
              Text(
                'Foto Bukti (Opsional)',
                style: theme.textTheme.bodyMedium?.copyWith(
                  fontWeight: FontWeight.w500,
                ),
              ),
              const SizedBox(height: 8),
              if (_photoFiles.isNotEmpty)
                Wrap(
                  spacing: 8,
                  runSpacing: 8,
                  children: [
                    ..._photoFiles.asMap().entries.map(
                      (entry) => Stack(
                        children: [
                          ClipRRect(
                            borderRadius: BorderRadius.circular(8),
                            child: Image.file(
                              File(entry.value.path),
                              width: 60,
                              height: 60,
                              fit: BoxFit.cover,
                            ),
                          ),
                          Positioned(
                            top: -4,
                            right: -4,
                            child: GestureDetector(
                              onTap: () {
                                setState(() {
                                  _photoFiles.removeAt(entry.key);
                                });
                              },
                              child: Container(
                                padding: const EdgeInsets.all(2),
                                decoration: const BoxDecoration(
                                  color: Colors.red,
                                  shape: BoxShape.circle,
                                ),
                                child: const Icon(
                                  Icons.close,
                                  size: 12,
                                  color: Colors.white,
                                ),
                              ),
                            ),
                          ),
                        ],
                      ),
                    ),
                    if (_photoFiles.length < 5)
                      GestureDetector(
                        onTap: _isSubmitting ? null : _pickPhotos,
                        child: Container(
                          width: 60,
                          height: 60,
                          decoration: BoxDecoration(
                            borderRadius: BorderRadius.circular(8),
                            border: Border.all(color: Colors.grey.shade400),
                          ),
                          child: Icon(
                            Icons.add_photo_alternate,
                            color: Colors.grey[600],
                          ),
                        ),
                      ),
                  ],
                )
              else
                OutlinedButton.icon(
                  onPressed: _isSubmitting ? null : _pickPhotos,
                  icon: const Icon(Icons.add_photo_alternate, size: 18),
                  label: const Text('Tambah Foto'),
                  style: OutlinedButton.styleFrom(
                    minimumSize: const Size(double.infinity, 44),
                  ),
                ),
              const SizedBox(height: 12),

              // Escrow freeze warning
              Container(
                padding: const EdgeInsets.all(12),
                decoration: BoxDecoration(
                  color: core.AppColors.statusWarning.withValues(alpha: 0.1),
                  borderRadius: BorderRadius.circular(8),
                ),
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Icon(
                      Icons.lock_clock,
                      color: core.AppColors.statusWarning,
                      size: 16,
                    ),
                    const SizedBox(width: 8),
                    Expanded(
                      child: Text(
                        'Dana akan dibekukan selama proses peninjauan admin.',
                        style: TextStyle(
                          fontSize: 11,
                          color: core.AppColors.statusWarning,
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          onPressed: _isSubmitting ? null : () => Navigator.of(context).pop(),
          child: const Text('Batal'),
        ),
        ElevatedButton(
          onPressed: _isSubmitting ? null : _submitDispute,
          style: ElevatedButton.styleFrom(
            backgroundColor: core.AppColors.statusWarning,
            foregroundColor: Colors.white,
            disabledBackgroundColor: Colors.grey,
          ),
          child: _isSubmitting
              ? const SizedBox(
                  width: 16,
                  height: 16,
                  child: CircularProgressIndicator(
                    strokeWidth: 2,
                    color: Colors.white,
                  ),
                )
              : const Text('Ajukan Sengketa'),
        ),
      ],
    );
  }
}
