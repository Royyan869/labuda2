library;

import 'dart:io';

import 'package:equatable/equatable.dart';
import 'package:labuda/domains/commerce/catalog/shared/data/dto/commerce_media_request_dto.dart';

enum CommerceLocalMediaType { image, video }

enum CommerceLocalMediaUploadState {
  selected,
  uploading,
  uploaded,
  failed,
  cancelled,
}

CommerceLocalMediaType commerceLocalMediaTypeFromFile(File file) {
  final name = file.path.split(Platform.pathSeparator).last.toLowerCase();
  final ext = name.contains('.') ? name.split('.').last : '';
  switch (ext) {
    case 'mp4':
    case 'mov':
    case 'avi':
    case 'mkv':
    case 'webm':
    case 'm4v':
    case '3gp':
    case 'wmv':
      return CommerceLocalMediaType.video;
    default:
      return CommerceLocalMediaType.image;
  }
}

String buildStableCommerceLocalMediaId(
  File file, {
  required int selectionOrder,
  required CommerceLocalMediaType mediaType,
}) {
  final path = file.path.replaceAll('\\', '/');
  final length = file.existsSync() ? file.lengthSync() : 0;
  final fingerprint = '$path|$length|$selectionOrder|$mediaType';
  var hash = 0x811c9dc5;
  for (final unit in fingerprint.codeUnits) {
    hash ^= unit;
    hash = (hash * 0x01000193) & 0xffffffff;
  }
  return hash.toRadixString(16).padLeft(8, '0');
}

class CommerceLocalMediaItem extends Equatable {
  final String stableLocalId;
  final File file;
  final CommerceLocalMediaType mediaType;
  final int selectionOrder;
  final String? localPosterPath;
  final int? width;
  final int? height;
  final int? durationMs;
  final CommerceLocalMediaUploadState uploadState;
  final String? uploadedUrl;
  final String? uploadedThumbnailUrl;
  final String? uploadedStorageKey;
  final String? uploadedThumbnailStorageKey;

  const CommerceLocalMediaItem({
    required this.stableLocalId,
    required this.file,
    required this.mediaType,
    required this.selectionOrder,
    this.localPosterPath,
    this.width,
    this.height,
    this.durationMs,
    this.uploadState = CommerceLocalMediaUploadState.selected,
    this.uploadedUrl,
    this.uploadedThumbnailUrl,
    this.uploadedStorageKey,
    this.uploadedThumbnailStorageKey,
  });

  factory CommerceLocalMediaItem.fromFile(
    File file, {
    required int selectionOrder,
  }) {
    final mediaType = commerceLocalMediaTypeFromFile(file);
    return CommerceLocalMediaItem(
      stableLocalId: buildStableCommerceLocalMediaId(
        file,
        selectionOrder: selectionOrder,
        mediaType: mediaType,
      ),
      file: file,
      mediaType: mediaType,
      selectionOrder: selectionOrder,
    );
  }

  CommerceLocalMediaItem copyWith({
    String? stableLocalId,
    File? file,
    CommerceLocalMediaType? mediaType,
    int? selectionOrder,
    String? localPosterPath,
    int? width,
    int? height,
    int? durationMs,
    CommerceLocalMediaUploadState? uploadState,
    String? uploadedUrl,
    String? uploadedThumbnailUrl,
    String? uploadedStorageKey,
    String? uploadedThumbnailStorageKey,
    bool clearLocalPosterPath = false,
    bool clearUploadedUrl = false,
    bool clearUploadedThumbnailUrl = false,
    bool clearUploadedStorageKey = false,
    bool clearUploadedThumbnailStorageKey = false,
  }) {
    return CommerceLocalMediaItem(
      stableLocalId: stableLocalId ?? this.stableLocalId,
      file: file ?? this.file,
      mediaType: mediaType ?? this.mediaType,
      selectionOrder: selectionOrder ?? this.selectionOrder,
      localPosterPath: clearLocalPosterPath
          ? null
          : localPosterPath ?? this.localPosterPath,
      width: width ?? this.width,
      height: height ?? this.height,
      durationMs: durationMs ?? this.durationMs,
      uploadState: uploadState ?? this.uploadState,
      uploadedUrl: clearUploadedUrl ? null : uploadedUrl ?? this.uploadedUrl,
      uploadedThumbnailUrl: clearUploadedThumbnailUrl
          ? null
          : uploadedThumbnailUrl ?? this.uploadedThumbnailUrl,
      uploadedStorageKey: clearUploadedStorageKey
          ? null
          : uploadedStorageKey ?? this.uploadedStorageKey,
      uploadedThumbnailStorageKey: clearUploadedThumbnailStorageKey
          ? null
          : uploadedThumbnailStorageKey ?? this.uploadedThumbnailStorageKey,
    );
  }

  CommerceMediaRequestDto toRequestDto({required int position}) {
    return CommerceMediaRequestDto(
      type: mediaType == CommerceLocalMediaType.video ? 'video' : 'image',
      url: uploadedUrl ?? file.path,
      position: position,
      width: width,
      height: height,
      duration: durationMs,
      thumbnailUrl: uploadedThumbnailUrl,
    );
  }

  @override
  List<Object?> get props => [
    stableLocalId,
    file.path,
    mediaType,
    selectionOrder,
    localPosterPath,
    width,
    height,
    durationMs,
    uploadState,
    uploadedUrl,
    uploadedThumbnailUrl,
    uploadedStorageKey,
    uploadedThumbnailStorageKey,
  ];
}
