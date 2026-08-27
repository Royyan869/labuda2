import 'package:labuda/domains/social/content/data/dto/content_dto.dart';
import 'package:labuda/domains/social/content/domain/entities/content_resource_occurrence.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart'
    show ContentLocation, ContentVisibility;

class ContentCreateRequest {
  final String content;
  final ContentVisibility? visibility;
  final List<CreateContentMediaRequestDto>? media;
  final List<String>? tags;
  final ContentResourceOccurrence? resourceOccurrence;
  final ContentLocation? location;

  const ContentCreateRequest({
    required this.content,
    this.visibility,
    this.media,
    this.tags,
    this.resourceOccurrence,
    this.location,
  });

  Map<String, dynamic> toJson() => <String, dynamic>{
    'caption': content,
    if (visibility != null) 'visibility': _visibilityToWire(visibility!),
    if (media != null) 'media': media!.map((item) => item.toJson()).toList(),
    if (tags != null) 'tags': tags,
    if (resourceOccurrence != null)
      'resource_occurrence': resourceOccurrence!.toJson(),
    if (location != null) 'location': _locationToJson(location!),
  };
}

class ContentUpdateRequest {
  final String? content;
  final ContentVisibility? visibility;
  final List<CreateContentMediaRequestDto>? media;
  final List<String>? tags;
  final ContentLocation? location;

  const ContentUpdateRequest({
    this.content,
    this.visibility,
    this.media,
    this.tags,
    this.location,
  });

  Map<String, dynamic> toJson() => <String, dynamic>{
    if (content != null) 'caption': content,
    if (visibility != null) 'visibility': _visibilityToWire(visibility!),
    if (media != null) 'media': media!.map((item) => item.toJson()).toList(),
    if (tags != null) 'tags': tags,
    if (location != null) 'location': _locationToJson(location!),
  };
}

String _visibilityToWire(ContentVisibility value) {
  switch (value) {
    case ContentVisibility.public:
      return 'public';
    case ContentVisibility.followersOnly:
      return 'followers_only';
    case ContentVisibility.private:
      return 'private';
  }
}

Map<String, dynamic> _locationToJson(ContentLocation value) =>
    <String, dynamic>{
      if (value.city != null) 'city': value.city,
      if (value.province != null) 'province': value.province,
      if (value.country != null) 'country': value.country,
      if (value.latitude != null) 'latitude': value.latitude,
      if (value.longitude != null) 'longitude': value.longitude,
      if (value.placeName != null) 'placeName': value.placeName,
    };
