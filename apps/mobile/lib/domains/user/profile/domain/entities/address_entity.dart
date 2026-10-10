import 'package:equatable/equatable.dart';
import 'package:hishumi/shared/shared.dart';

/// A saved address owned by the account.
///
/// CANONICAL TRUTH: the account owns one address book (0..N addresses).
/// Exactly one address is primary when the account has any active address.
/// The primary address is the account's single default address: the default
/// destination at checkout and the default origin for every product. There are
/// no shipping/sender roles, no tags, and no address purpose.
///
/// Business rules:
/// - Many addresses per account; at most one `isPrimary` across the whole
///   account (the backend enforces this).
/// - Max 10 addresses per account.
class AddressEntity extends Equatable {
  final String id;
  final String userId;

  /// Optional user-defined label for recognition only (e.g., "Rumah",
  /// "Kantor"). Never a business role and never used for branching.
  final String? nickname;

  final String recipientName; // Nama penerima (bisa beda dari user)
  final String phone; // Nomor telepon penerima
  final Province province;
  final City city;
  final District district;
  final Village village;
  final String streetAddress;
  final String postalCode;
  final String? notes; // Optional notes/instructions

  /// The account's one default address. Setting it clears it everywhere else.
  final bool isPrimary;

  final double? latitude; // Optional GPS coordinate
  final double? longitude; // Optional GPS coordinate
  final DateTime createdAt;
  final DateTime updatedAt;

  const AddressEntity({
    required this.id,
    required this.userId,
    this.nickname,
    required this.recipientName,
    required this.phone,
    required this.province,
    required this.city,
    required this.district,
    required this.village,
    required this.streetAddress,
    required this.postalCode,
    this.notes,
    this.isPrimary = false,
    this.latitude,
    this.longitude,
    required this.createdAt,
    required this.updatedAt,
  });

  @override
  List<Object?> get props => [
        id,
        userId,
        nickname,
        recipientName,
        phone,
        province,
        city,
        district,
        village,
        streetAddress,
        postalCode,
        notes,
        isPrimary,
        latitude,
        longitude,
        createdAt,
        updatedAt,
      ];

  /// Get full formatted address string
  String get fullAddress =>
      '$streetAddress, ${village.name}, ${district.name}, ${city.name}, '
      '${province.name} $postalCode';

  /// Check if address has GPS coordinates
  bool get hasCoordinates => latitude != null && longitude != null;

  AddressEntity copyWith({
    String? id,
    String? userId,
    String? nickname,
    String? recipientName,
    String? phone,
    Province? province,
    City? city,
    District? district,
    Village? village,
    String? streetAddress,
    String? postalCode,
    String? notes,
    bool? isPrimary,
    double? latitude,
    double? longitude,
    DateTime? createdAt,
    DateTime? updatedAt,
  }) {
    return AddressEntity(
      id: id ?? this.id,
      userId: userId ?? this.userId,
      nickname: nickname ?? this.nickname,
      recipientName: recipientName ?? this.recipientName,
      phone: phone ?? this.phone,
      province: province ?? this.province,
      city: city ?? this.city,
      district: district ?? this.district,
      village: village ?? this.village,
      streetAddress: streetAddress ?? this.streetAddress,
      postalCode: postalCode ?? this.postalCode,
      notes: notes ?? this.notes,
      isPrimary: isPrimary ?? this.isPrimary,
      latitude: latitude ?? this.latitude,
      longitude: longitude ?? this.longitude,
      createdAt: createdAt ?? this.createdAt,
      updatedAt: updatedAt ?? this.updatedAt,
    );
  }

  /// Convert to JSON (snake_case sesuai Kepmendagri standard)
  Map<String, dynamic> toJson() {
    return {
      'user_id': userId,
      'nickname': nickname,
      'recipient_name': recipientName,
      'phone': phone,
      'province': province.toJson(),
      'city': city.toJson(),
      'district': district.toJson(),
      'village': village.toJson(),
      'street_address': streetAddress,
      'postal_code': postalCode,
      'notes': notes,
      'is_primary': isPrimary,
      'latitude': latitude,
      'longitude': longitude,
      'created_at': createdAt.toIso8601String(),
      'updated_at': updatedAt.toIso8601String(),
    };
  }

  /// Create from the backend wire shape.
  factory AddressEntity.fromJson(Map<String, dynamic> json, String id) {
    return AddressEntity(
      id: id,
      userId: (json['userId'] ?? json['user_id']) as String,
      nickname: json['nickname'] as String?,
      recipientName:
          (json['recipientName'] ?? json['recipient_name'] ?? '') as String,
      phone: (json['phone'] ?? '') as String,
      province: Province.fromJson(json['province'] as Map<String, dynamic>),
      city: City.fromJson(json['city'] as Map<String, dynamic>),
      district: District.fromJson(json['district'] as Map<String, dynamic>),
      village: Village.fromJson(json['village'] as Map<String, dynamic>),
      streetAddress:
          (json['streetAddress'] ?? json['street_address']) as String,
      postalCode: (json['postalCode'] ?? json['postal_code']) as String,
      notes: json['notes'] as String?,
      isPrimary: (json['isPrimary'] ?? json['is_primary']) as bool? ?? false,
      latitude: json['latitude'] != null
          ? (json['latitude'] as num).toDouble()
          : null,
      longitude: json['longitude'] != null
          ? (json['longitude'] as num).toDouble()
          : null,
      createdAt: DateTime.parse(
        (json['createdAt'] ?? json['created_at']) as String,
      ),
      updatedAt: DateTime.parse(
        (json['updatedAt'] ?? json['updated_at']) as String,
      ),
    );
  }

  @override
  String toString() {
    return 'AddressEntity(id: $id, nickname: $nickname, '
        'isPrimary: $isPrimary, address: $fullAddress)';
  }
}
