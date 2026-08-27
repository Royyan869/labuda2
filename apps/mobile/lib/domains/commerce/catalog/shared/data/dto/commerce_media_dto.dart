library;

import 'package:equatable/equatable.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';

class CommerceMediaDto extends Equatable {
  final String? id;
  final String url;
  final String type;
  final int? position;
  final DateTime? createdAt;
  final String? blurhash;
  final int? duration;
  final int? width;
  final int? height;
  final Map<String, String> variants;

  const CommerceMediaDto({
    this.id,
    required this.url,
    required this.type,
    this.position,
    this.createdAt,
    this.blurhash,
    this.duration,
    this.width,
    this.height,
    this.variants = const {},
  });

  factory CommerceMediaDto.fromJson(Map<String, dynamic> json) {
    final variants = json['variants'];
    final thumbnailUrl = json['thumbnail_url'] as String?;
    final resolvedVariants = variants is Map<String, dynamic>
        ? variants.map((key, value) => MapEntry(key, value.toString()))
        : <String, String>{};

    if (thumbnailUrl != null && thumbnailUrl.isNotEmpty) {
      resolvedVariants.putIfAbsent('thumbnail', () => thumbnailUrl);
    }

    return CommerceMediaDto(
      id: json['id'] as String?,
      url: _readUrl(json),
      type: _readType(json),
      position: (json['position'] as num?)?.toInt(),
      createdAt: _readCreatedAt(json),
      blurhash: json['blurhash'] as String?,
      duration: (json['duration'] as num?)?.toInt(),
      width: (json['width'] as num?)?.toInt(),
      height: (json['height'] as num?)?.toInt(),
      variants: resolvedVariants,
    );
  }

  factory CommerceMediaDto.legacyUrl(
    String url, {
    int position = 0,
    DateTime? createdAt,
  }) {
    return CommerceMediaDto(
      id: _stableMediaId(url),
      url: url,
      type: _inferType(url),
      position: position,
      createdAt: createdAt,
    );
  }

  MediaEntity toEntity({DateTime? fallbackCreatedAt}) {
    final parsedType = _mapType(type);
    final resolvedCreatedAt =
        createdAt ?? fallbackCreatedAt ?? _zeroTimestampUtc;
    final dimensions = width != null && height != null
        ? MediaDimensions(width: width!, height: height!)
        : null;

    return MediaEntity(
      id: id ?? _stableMediaId(url),
      originalUrl: url,
      type: parsedType,
      position: position,
      blurhash: blurhash,
      dimensions: dimensions,
      duration: duration,
      createdAt: resolvedCreatedAt,
      variants: variants,
    );
  }

  static MediaType _mapType(String value) {
    final normalized = value.trim().toLowerCase();
    if (normalized == 'video') return MediaType.video;
    if (normalized == 'image') return MediaType.image;
    return _inferType(value) == 'video' ? MediaType.video : MediaType.image;
  }

  static String _inferType(String value) {
    final lower = value.trim().toLowerCase();
    if (lower.endsWith('.mp4') ||
        lower.endsWith('.mov') ||
        lower.endsWith('.webm') ||
        lower.endsWith('.m4v') ||
        lower.endsWith('.avi') ||
        lower.endsWith('.mkv') ||
        lower.endsWith('.3gp') ||
        lower.endsWith('.wmv')) {
      return 'video';
    }
    return 'image';
  }

  static String _readUrl(Map<String, dynamic> json) {
    final url = json['url'] ?? json['media_url'] ?? json['original_url'];
    if (url is String && url.isNotEmpty) {
      return url;
    }
    return '';
  }

  static String _readType(Map<String, dynamic> json) {
    final type = json['type'] ?? json['media_type'];
    if (type is String && type.isNotEmpty) {
      return type;
    }
    return _inferType(_readUrl(json));
  }

  static DateTime? _readCreatedAt(Map<String, dynamic> json) {
    final raw = json['created_at'] ?? json['createdAt'];
    if (raw is String && raw.isNotEmpty) {
      return DateTime.parse(raw);
    }
    return null;
  }

  static String _stableMediaId(String url) {
    final uri = Uri.tryParse(url);
    if (uri != null && uri.pathSegments.isNotEmpty) {
      final candidate = uri.pathSegments.last.split('.').first;
      if (candidate.isNotEmpty) {
        return candidate;
      }
    }
    return url.isEmpty ? 'media-unknown' : url.hashCode.toString();
  }

  @override
  List<Object?> get props => [
    id,
    url,
    type,
    position,
    createdAt,
    blurhash,
    duration,
    width,
    height,
    variants,
  ];
}

final DateTime _zeroTimestampUtc = DateTime.fromMillisecondsSinceEpoch(
  0,
  isUtc: true,
);
