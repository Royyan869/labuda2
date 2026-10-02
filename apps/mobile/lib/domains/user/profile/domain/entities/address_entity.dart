import 'package:equatable/equatable.dart';
import 'package:labuda/shared/shared.dart';

/// Role tags describing how a saved address may be used.
///
/// CANONICAL TRUTH: an address belongs to the ACCOUNT, not to a role. Tags are
/// a SET — one address may be both a shipping destination and a sender origin,
/// and promotion scope will attach further tags later without a new entity.
enum AddressTag {
  /// Destination the account can be delivered to (any account may create one).
  shipping,

  /// Origin goods are shipped from (the account's farm/warehouse).
  sender,
}

extension AddressTagExtension on AddressTag {
  /// Wire value accepted by the backend (`tags: [...]`).
  String get wireValue => name;

  String get label {
    switch (this) {
      case AddressTag.shipping:
        return 'Shipping Address';
      case AddressTag.sender:
        return 'Sender Address';
    }
  }

  String get description {
    switch (this) {
      case AddressTag.shipping:
        return 'Address for receiving packages/shipments';
      case AddressTag.sender:
        return 'Origin address for goods (for seller)';
    }
  }

  String get shortLabel {
    switch (this) {
      case AddressTag.shipping:
        return 'Shipping';
      case AddressTag.sender:
        return 'Sender';
    }
  }

  /// Parses a wire value. Unknown values resolve to null — the caller decides
  /// whether to fail or to drop the tag, never to silently invent a role.
  static AddressTag? parse(String raw) {
    for (final tag in AddressTag.values) {
      if (tag.wireValue == raw) return tag;
    }
    return null;
  }
}

/// A saved address owned by the account.
///
/// Business rules:
/// - Many addresses per account; at most one `isPrimary` across the whole
///   account (the backend enforces this — there is no per-tag primary).
/// - `tags` must be non-empty; an address may carry both tags.
/// - Max 10 addresses per account.
class AddressEntity extends Equatable {
  final String id;
  final String userId;

  /// How this address may be used. A set, not a single role.
  final List<AddressTag> tags;

  /// Optional user-defined nickname (e.g., "Rumah Utama", "Kantor", "Farm Sukabumi")
  final String? nickname;

  final String recipientName; // Nama penerima/pengirim (bisa beda dari user)
  final String phone; // Nomor telepon penerima/pengirim
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
    required this.tags,
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
        tags,
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

  /// Whether this address carries [tag].
  bool hasTag(AddressTag tag) => tags.contains(tag);

  /// Wire representation of the tag set.
  List<String> get tagValues => tags.map((tag) => tag.wireValue).toList();

  /// Display label priority: nickname, then the joined tag labels.
  String get displayLabel {
    final tagText = tags.map((tag) => tag.label).join(', ');
    if (nickname != null && nickname!.isNotEmpty) {
      return tags.isEmpty ? nickname! : '$nickname ($tagText)';
    }
    return tagText;
  }

  /// Get full formatted address string
  String get fullAddress =>
      '$streetAddress, ${village.name}, ${district.name}, ${city.name}, '
      '${province.name} $postalCode';

  /// Check if address has GPS coordinates
  bool get hasCoordinates => latitude != null && longitude != null;

  /// Only addresses tagged for shipping are selectable at checkout.

  AddressEntity copyWith({
    String? id,
    String? userId,
    List<AddressTag>? tags,
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
      tags: tags ?? this.tags,
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
      'tags': tagValues,
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
  ///
  /// An unknown tag value is dropped rather than defaulted to a role: an
  /// address never silently claims a usage the backend did not declare.
  factory AddressEntity.fromJson(Map<String, dynamic> json, String id) {
    final rawTags = (json['tags'] as List<dynamic>? ?? const [])
        .whereType<String>()
        .toList();

    return AddressEntity(
      id: id,
      userId: (json['userId'] ?? json['user_id']) as String,
      tags: rawTags
          .map(AddressTagExtension.parse)
          .whereType<AddressTag>()
          .toList(),
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
    return 'AddressEntity(id: $id, tags: ${tagValues.join('|')}, '
        'nickname: $nickname, isPrimary: $isPrimary, address: $fullAddress)';
  }
}
