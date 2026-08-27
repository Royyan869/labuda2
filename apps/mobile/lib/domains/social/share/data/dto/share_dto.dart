import '../../domain/domain.dart';

/// DTO for Firestore operations
/// Data layer - can contain Firebase-specific types
class ShareDto {
  final String targetId;
  final String targetType;
  final String? imageUrl;
  final String? deepLink;
  final Map<String, dynamic> metadata;

  const ShareDto({
    required this.targetId,
    required this.targetType,
    this.imageUrl,
    this.deepLink,
    this.metadata = const {},
  });

  /// Create from Firestore document
  factory ShareDto.fromJson(Map<String, dynamic> json) {
    return ShareDto(
      targetId: json['targetId'] as String,
      targetType: json['targetType'] as String,
      imageUrl: json['imageUrl'] as String?,
      deepLink: json['deepLink'] as String?,
      metadata: json['metadata'] as Map<String, dynamic>? ?? {},
    );
  }

  /// Convert to Firestore document
  Map<String, dynamic> toJson() {
    return {
      'targetId': targetId,
      'targetType': targetType,
      'imageUrl': imageUrl,
      'deepLink': deepLink,
      'metadata': metadata,
    };
  }
}

/// Mapper between DTO and Entity
class ShareMapper {
  /// Convert Entity to DTO
  static ShareDto toDto(ShareTarget entity) {
    return ShareDto(
      targetId: entity.id,
      targetType: entity.type.name,
      imageUrl: entity.imageUrl,
      deepLink: entity.publicShareUrl,
      metadata: entity.metadata,
    );
  }

  /// Convert DTO to Entity
  static ShareTarget toEntity(ShareDto dto) {
    return ShareTarget(
      id: dto.targetId,
      type: _parseTargetType(dto.targetType),
      title: dto.metadata['title'] as String? ?? '',
      description: dto.metadata['description'] as String? ?? '',
      imageUrl: dto.imageUrl,
      publicShareUrl: dto.deepLink,
      metadata: dto.metadata,
    );
  }

  static ExternalShareType _parseTargetType(String type) {
    return ExternalShareType.values.firstWhere(
      (e) => e.name == type,
      orElse: () => ExternalShareType.post,
    );
  }

  /// Generate default caption based on target type
  static String generateDefaultCaption(ShareTarget target) {
    switch (target.type) {
      case ExternalShareType.listing:
        return 'Lihat listing koi bagus ini!';
      case ExternalShareType.request:
        return 'Ada yang bisa bantu request ini?';
      case ExternalShareType.auction:
        return 'Lelang menarik! Buruan bid!';
      case ExternalShareType.post:
        return 'Worth sharing!';
      case ExternalShareType.profile:
        return 'Check out this profile!';
    }
  }
}
