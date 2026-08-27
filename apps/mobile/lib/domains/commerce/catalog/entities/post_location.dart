/// Enhanced location entity dengan coordinates support
/// Digunakan untuk Post, Request, dan fitur lain yang butuh location
/// MIGRATED: Firestore methods removed, now using JSON only
class PostLocation {
  final String
  address; // Display address (e.g., "Jakarta Selatan, DKI Jakarta")
  final double? latitude; // GPS latitude
  final double? longitude; // GPS longitude
  final String? placeId; // Google Place ID (optional, untuk advanced features)

  const PostLocation({
    required this.address,
    this.latitude,
    this.longitude,
    this.placeId,
  });

  /// Create from JSON
  factory PostLocation.fromJson(Map<String, dynamic> json) {
    return PostLocation(
      address: json['address'] as String? ?? '',
      latitude: json['latitude'] != null
          ? (json['latitude'] as num).toDouble()
          : null,
      longitude: json['longitude'] != null
          ? (json['longitude'] as num).toDouble()
          : null,
      placeId: json['placeId'] as String?,
    );
  }

  /// Convert to JSON
  Map<String, dynamic> toJson() {
    return {
      'address': address,
      'latitude': latitude,
      'longitude': longitude,
      'placeId': placeId,
    };
  }

  /// Check if location has coordinates
  bool get hasCoordinates => latitude != null && longitude != null;

  /// Get coordinates as LatLng (for maps)
  /// Returns null if coordinates not available
  LatLng? get latLng {
    if (latitude == null || longitude == null) return null;
    return LatLng(latitude!, longitude!);
  }

  /// Copy with
  PostLocation copyWith({
    String? address,
    double? latitude,
    double? longitude,
    String? placeId,
  }) {
    return PostLocation(
      address: address ?? this.address,
      latitude: latitude ?? this.latitude,
      longitude: longitude ?? this.longitude,
      placeId: placeId ?? this.placeId,
    );
  }

  @override
  String toString() => address;

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is PostLocation &&
          runtimeType == other.runtimeType &&
          address == other.address &&
          latitude == other.latitude &&
          longitude == other.longitude &&
          placeId == other.placeId;

  @override
  int get hashCode =>
      address.hashCode ^
      latitude.hashCode ^
      longitude.hashCode ^
      placeId.hashCode;
}

/// Simple LatLng class untuk compatibility
class LatLng {
  final double latitude;
  final double longitude;

  const LatLng(this.latitude, this.longitude);

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is LatLng &&
          runtimeType == other.runtimeType &&
          latitude == other.latitude &&
          longitude == other.longitude;

  @override
  int get hashCode => latitude.hashCode ^ longitude.hashCode;
}
