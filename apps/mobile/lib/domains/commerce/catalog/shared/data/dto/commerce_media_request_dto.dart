library;

import 'package:equatable/equatable.dart';

/// Typed commerce media payload sent to the backend create endpoints.
///
/// The backend accepts typed media rows with canonical `type` and `url`
/// plus optional metadata. We keep this shape shared across listing and
/// auction create flows so the producer path can stay backend-aligned.
class CommerceMediaRequestDto extends Equatable {
  final String type;
  final String url;
  final int? position;
  final int? width;
  final int? height;
  final int? duration;
  final String? thumbnailUrl;

  const CommerceMediaRequestDto({
    required this.type,
    required this.url,
    this.position,
    this.width,
    this.height,
    this.duration,
    this.thumbnailUrl,
  });

  factory CommerceMediaRequestDto.image({
    required String url,
    int? width,
    int? height,
  }) {
    return CommerceMediaRequestDto(
      type: 'image',
      url: url,
      position: null,
      width: width,
      height: height,
    );
  }

  factory CommerceMediaRequestDto.video({
    required String url,
    int? width,
    int? height,
    int? duration,
    String? thumbnailUrl,
  }) {
    return CommerceMediaRequestDto(
      type: 'video',
      url: url,
      position: null,
      width: width,
      height: height,
      duration: duration,
      thumbnailUrl: thumbnailUrl,
    );
  }

  CommerceMediaRequestDto copyWith({
    String? type,
    String? url,
    int? position,
    int? width,
    int? height,
    int? duration,
    String? thumbnailUrl,
    bool clearPosition = false,
    bool clearThumbnailUrl = false,
  }) {
    return CommerceMediaRequestDto(
      type: type ?? this.type,
      url: url ?? this.url,
      position: clearPosition ? null : position ?? this.position,
      width: width ?? this.width,
      height: height ?? this.height,
      duration: duration ?? this.duration,
      thumbnailUrl: clearThumbnailUrl
          ? null
          : thumbnailUrl ?? this.thumbnailUrl,
    );
  }

  factory CommerceMediaRequestDto.fromJson(Map<String, dynamic> json) {
    return CommerceMediaRequestDto(
      type: (json['type'] as String?) ?? 'image',
      url: (json['url'] as String?) ?? '',
      position: (json['position'] as num?)?.toInt(),
      width: (json['width'] as num?)?.toInt(),
      height: (json['height'] as num?)?.toInt(),
      duration: (json['duration'] as num?)?.toInt(),
      thumbnailUrl: json['thumbnail_url'] as String?,
    );
  }

  Map<String, dynamic> toJson() => {
    'type': type,
    'url': url,
    if (position != null) 'position': position,
    if (width != null) 'width': width,
    if (height != null) 'height': height,
    if (duration != null) 'duration': duration,
    if (thumbnailUrl != null) 'thumbnail_url': thumbnailUrl,
  };

  @override
  List<Object?> get props => [
    type,
    url,
    position,
    width,
    height,
    duration,
    thumbnailUrl,
  ];
}
