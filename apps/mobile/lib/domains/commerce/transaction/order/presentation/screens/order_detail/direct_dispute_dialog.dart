/// Direct Dispute Dialog - Allows buyer to open a dispute directly on a shipped order
library;

import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:image_picker/image_picker.dart';
import 'package:labuda/core/core.dart' as core;
import 'package:labuda/core/media/media_upload_config.dart';
import 'package:labuda/core/media/media_upload_orchestrator.dart';
import 'package:labuda/core/providers/core_providers.dart';
import 'package:labuda/core/src/theme/app_theme.dart';
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
    final video = await MediaUploadOrchestrator.pickGalleryVideo(
      context: context,
    );
    if (video != null && mounted) {
      setState(() {
        _videoFile = video;
      });
    }
  }

  Future<void> _pickPhotos() async {
    final photos = await MediaUploadOrchestrator.pickGalleryImages(
      context: context,
      maxAssets: 5,
    );
    if (photos.isNotEmpty &&
        mounted &&
        _photoFiles.length + photos.length <= 5) {
      setState(() {
        _photoFiles.addAll(photos);
      });
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

    // SUBMISSION SNAPSHOT: capture the dispute intent before any upload await
    // so editing the reason, description or photos while uploading cannot
    // change the request that is already in flight.
    final reason = _selectedReason!;
    final description = _descriptionController.text.trim();
    final photos = List<XFile>.of(_photoFiles);

    try {
      final s3Service = ref.read(s3ServiceProvider);
      final evidenceUrls = <String>[];
      String? videoUrl;

      // Upload video (required)
      final videoResult = await s3Service.uploadVideo(
        File(_videoFile!.path),
        folder: MediaUploadConfig.forEvidence.videoFolder,
      );
      if (videoResult.isSuccess && videoResult.data != null) {
        videoUrl = videoResult.data!;
      } else {
        throw Exception('Video upload gagal: ${videoResult.error}');
      }

      // Upload photos (optional)
      for (final photo in photos) {
        final result = await s3Service.uploadImage(
          File(photo.path),
          folder: MediaUploadConfig.forEvidence.imageFolder,
        );
        if (result.isSuccess && result.data != null) {
          evidenceUrls.add(result.data!);
        }
      }

      final datasource = ref.read(orderApiDatasourceProvider);

      await datasource.createDispute(
        widget.orderId,
        CreateDisputeDto(
          reason: reason.displayName,
          reasonCode: reason.apiValue,
          description: description.isEmpty ? null : description,
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
    final colorScheme = theme.colorScheme;

    return AlertDialog(
      title: Row(
        children: [
          Icon(
            Icons.report_problem_rounded,
            color: context.statusColors.warning,
            size: AppIconSize.header,
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Text(
              'Buka Dispute',
              style: context.typeRoles.titleSection.copyWith(
                fontWeight: FontWeight.w600,
              ),
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
                padding: const EdgeInsets.all(core.AppMetrics.p12),
                decoration: BoxDecoration(
                  color: colorScheme.secondary.withValues(alpha: 0.1),
                  borderRadius: BorderRadius.circular(core.AppShape.r8),
                  border: Border.all(
                    color: colorScheme.secondary.withValues(alpha: 0.3),
                  ),
                ),
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Icon(
                      Icons.info_outline_rounded,
                      color: colorScheme.secondary,
                      size: AppIconSize.action,
                    ),
                    const SizedBox(width: 8),
                    Expanded(
                      child: Text(
                        'Admin akan meninjau kasus ini secara adil berdasarkan bukti dari kedua pihak.',
                        style: context.typeRoles.bodyDense.copyWith(
                          color: colorScheme.onSurface,
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
                // Border/fill/geometry come from `inputDecorationTheme`
                // (AppTheme) — the one form-field authority.
                decoration: const InputDecoration(hintText: 'Pilih alasan...'),
                items: DisputeReasonCode.values
                    .map(
                      (reason) => DropdownMenuItem(
                        value: reason,
                        child: Text(
                          reason.displayName,
                          style: Theme.of(context).textTheme.bodyLarge,
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
                // Border/fill come from `inputDecorationTheme` (AppTheme) —
                // the one form-field authority (the fill role is the same
                // surfaceContainerHigh this site used to restate).
                decoration: const InputDecoration(
                  hintText: 'Jelaskan secara detail masalah yang Anda alami...',
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
                  padding: const EdgeInsets.all(core.AppMetrics.p8),
                  decoration: BoxDecoration(
                    color: colorScheme.surfaceContainerHigh,
                    borderRadius: BorderRadius.circular(core.AppShape.r8),
                    border: Border.all(color: colorScheme.outlineVariant),
                  ),
                  child: Row(
                    children: [
                      Icon(
                        Icons.videocam_rounded,
                        color: context.statusColors.success,
                        size: AppIconSize.action,
                      ),
                      const SizedBox(width: 8),
                      Expanded(
                        child: Text(
                          _videoFile!.name,
                          style: context.typeRoles.labelMicro,
                          overflow: TextOverflow.ellipsis,
                        ),
                      ),
                      IconButton(
                        icon: const Icon(Icons.close, size: AppIconSize.action, semanticLabel: 'Hapus video'),
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
                  icon: const Icon(
                    Icons.videocam_rounded,
                    size: AppIconSize.action,
                  ),
                  label: const Text('Pilih Video Bukti'),
                  style: OutlinedButton.styleFrom(
                    minimumSize: const Size(double.infinity, 44),
                  ),
                ),
              const SizedBox(height: 4),
              Text(
                'Rekam video unboxing atau bukti masalah (maks. 2 menit)',
                style: context.typeRoles.labelMicro.copyWith(
                  color: colorScheme.onSurfaceVariant,
                ),
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
                            borderRadius: BorderRadius.circular(
                              core.AppShape.r8,
                            ),
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
                                padding: const EdgeInsets.all(
                                  core.AppMetrics.p4,
                                ),
                                decoration: BoxDecoration(
                                  color: colorScheme.error,
                                  shape: BoxShape.circle,
                                ),
                                child: Icon(
                                  Icons.close,
                                  size: AppIconSize.inlineGlyph,
                                  color: colorScheme.onError,
                                  semanticLabel: 'Hapus foto',
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
                            borderRadius: BorderRadius.circular(
                              core.AppShape.r8,
                            ),
                            border: Border.all(
                              color: colorScheme.outlineVariant,
                            ),
                          ),
                          child: Icon(
                            Icons.add_photo_alternate,
                            color: colorScheme.onSurfaceVariant,
                            semanticLabel: 'Tambah foto',
                          ),
                        ),
                      ),
                  ],
                )
              else
                OutlinedButton.icon(
                  onPressed: _isSubmitting ? null : _pickPhotos,
                  icon: const Icon(
                    Icons.add_photo_alternate,
                    size: AppIconSize.action,
                  ),
                  label: const Text('Tambah Foto'),
                  style: OutlinedButton.styleFrom(
                    minimumSize: const Size(double.infinity, 44),
                  ),
                ),
              const SizedBox(height: 12),

              // Escrow freeze warning
              Container(
                padding: const EdgeInsets.all(core.AppMetrics.p12),
                decoration: BoxDecoration(
                  color: context.statusColors.warning.withValues(alpha: 0.1),
                  borderRadius: BorderRadius.circular(core.AppShape.r8),
                ),
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Icon(
                      Icons.lock_clock,
                      color: context.statusColors.warning,
                      size: AppIconSize.inlineGlyph,
                    ),
                    const SizedBox(width: 8),
                    Expanded(
                      child: Text(
                        'Dana akan dibekukan selama proses peninjauan admin.',
                        style: context.typeRoles.labelMicro.copyWith(
                          color: context.statusColors.warning,
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
          child: _isSubmitting
              ? SizedBox(
                  width: 16,
                  height: 16,
                  child: CircularProgressIndicator(
                    strokeWidth: 2,
                    color: colorScheme.onSurfaceVariant,
                  ),
                )
              : const Text('Ajukan Sengketa'),
        ),
      ],
    );
  }
}
