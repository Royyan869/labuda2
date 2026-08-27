library;

import 'dart:io';

import 'package:equatable/equatable.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/core/src/utils/constants/app_constants.dart';
import 'package:labuda/core/services/s3_service.dart';
import 'package:labuda/domains/commerce/catalog/shared/data/dto/commerce_media_request_dto.dart';
import 'package:labuda/domains/commerce/catalog/shared/data/models/commerce_local_media_item.dart';

class CommerceMediaUploadBatch extends Equatable {
  final List<CommerceLocalMediaItem> items;
  final List<CommerceMediaRequestDto> media;
  final bool cancelled;
  final String? errorMessage;

  const CommerceMediaUploadBatch({
    required this.items,
    required this.media,
    this.cancelled = false,
    this.errorMessage,
  });

  @override
  List<Object?> get props => [items, media, cancelled, errorMessage];
}

class CommerceMediaUploadCoordinator {
  final S3Service _s3Service;
  bool _disposed = false;
  int _attemptCounter = 0;

  CommerceMediaUploadCoordinator({S3Service? s3Service})
    : _s3Service = s3Service ?? S3Service();

  void cancelActiveUploads() {
    _attemptCounter += 1;
  }

  void dispose() {
    _disposed = true;
    cancelActiveUploads();
  }

  Future<Result<CommerceMediaUploadBatch>> uploadTypedMedia(
    List<CommerceLocalMediaItem> items, {
    int maxVideoDurationMs = AppConstants.maxCommerceVideoDurationMs,
    String domainLabel = 'Commerce',
  }) async {
    if (items.isEmpty) {
      return Result.success(
        const CommerceMediaUploadBatch(items: [], media: []),
      );
    }

    final attemptId = ++_attemptCounter;
    final uploaded = <CommerceLocalMediaItem>[];

    for (var index = 0; index < items.length; index += 1) {
      if (_isStaleAttempt(attemptId)) {
        await _cleanupUploaded(uploaded);
        return Result.success(
          const CommerceMediaUploadBatch(
            items: [],
            media: [],
            cancelled: true,
            errorMessage: 'Upload dibatalkan',
          ),
        );
      }

      final source = items[index];
      final uploading = source.copyWith(
        uploadState: CommerceLocalMediaUploadState.uploading,
      );
      final result = uploading.mediaType == CommerceLocalMediaType.video
          ? await _s3Service.uploadVideoWithMeta(uploading.file)
          : await _s3Service.uploadImageWithMeta(uploading.file);

      if (_isStaleAttempt(attemptId)) {
        await _cleanupUploaded(uploaded);
        return Result.success(
          const CommerceMediaUploadBatch(
            items: [],
            media: [],
            cancelled: true,
            errorMessage: 'Upload dibatalkan',
          ),
        );
      }

      if (!result.isSuccess || result.data == null) {
        await _cleanupUploaded(uploaded);
        return Result.error(result.error ?? 'Gagal upload media');
      }

      final upload = result.data!;
      uploaded.add(
        source.copyWith(
          uploadState: CommerceLocalMediaUploadState.uploaded,
          uploadedUrl: upload.url,
          uploadedStorageKey: upload.key,
        ),
      );
    }

    final media = uploaded
        .asMap()
        .entries
        .map((entry) => entry.value.toRequestDto(position: entry.key))
        .toList(growable: false);
    return Result.success(
      CommerceMediaUploadBatch(items: uploaded, media: media),
    );
  }

  bool _isStaleAttempt(int attemptId) =>
      _disposed || attemptId != _attemptCounter;

  Future<void> _cleanupUploaded(List<CommerceLocalMediaItem> items) async {
    for (final item in items.reversed) {
      if (item.uploadedThumbnailStorageKey != null) {
        await _s3Service.deleteFile(item.uploadedThumbnailStorageKey!);
      }
      if (item.uploadedStorageKey != null) {
        await _s3Service.deleteFile(item.uploadedStorageKey!);
      }
      if (item.localPosterPath != null) {
        try {
          final posterFile = File(item.localPosterPath!);
          if (await posterFile.exists()) {
            await posterFile.delete();
          }
        } catch (_) {
          // Best-effort cleanup only.
        }
      }
    }
  }
}
