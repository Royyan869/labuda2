import 'package:equatable/equatable.dart';
import 'package:labuda/shared/shared.dart';

/// Purpose-based address categorization
/// Clear separation between shipping (destination) and sender (origin) addresses
enum AddressPurpose {
  shipping, // Alamat tujuan pengiriman (buyer & seller punya ini)
  sender, // Alamat pengirim/asal barang (seller only)
}

/// Helper extension for AddressPurpose
extension AddressPurposeExtension on AddressPurpose {
  String get label {
    switch (this) {
      case AddressPurpose.shipping:
        return 'Shipping Address';
      case AddressPurpose.sender:
        return 'Sender Address';
    }
  }

  String get description {
    switch (this) {
      case AddressPurpose.shipping:
        return 'Address for receiving packages/shipments';
      case AddressPurpose.sender:
        return 'Origin address for goods (for seller)';
    }
  }
}

/// Address Entity untuk multiple addresses support
/// Purpose-based separation: shipping (tujuan) vs sender (pengirim)
///
/// Business Rules:
/// - Buyer: minimal 1 shipping address
/// - Seller: minimal 1 shipping + 1 sender address
/// - Max 10 addresses per purpose per user
/// - isPrimary: untuk menandai alamat default (per purpose)
class AddressEntity extends Equatable {
  final String id;
  final String userId;

  /// Purpose of this address (shipping destination vs sender origin)
  final AddressPurpose purpose;

  /// Optional user-defined nickname (e.g., "Rumah Utama", "Kantor", "Farm Sukabumi")
  /// Replaces the old label system with more flexible user input
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

  /// Default address for this purpose
  /// When set as primary, other addresses with same purpose will be unset
  final bool isPrimary;

  final double? latitude; // Optional GPS coordinate
  final double? longitude; // Optional GPS coordinate
  final DateTime createdAt;
  final DateTime updatedAt;

  const AddressEntity({
    required this.id,
    required this.userId,
    required this.purpose,
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
    purpose,
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

  /// Get display label for this address
  /// Priority: nickname > purpose label
  String get displayLabel {
    if (nickname != null && nickname!.isNotEmpty) {
      return nickname!;
    }
    return purpose.label;
  }

  /// Get full formatted address string
  String get fullAddress {
    return '$streetAddress, ${village.name}, ${district.name}, ${city.name}, ${province.name} $postalCode';
  }

  /// Check if address has GPS coordinates
  bool get hasCoordinates => latitude != null && longitude != null;

  /// Check if this address can be used for shipping/checkout
  /// Only shipping addresses are available for checkout
  bool get isAvailableForCheckout => purpose == AddressPurpose.shipping;

  /// Copy with method for immutability
  AddressEntity copyWith({
    String? id,
    String? userId,
    AddressPurpose? purpose,
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
      purpose: purpose ?? this.purpose,
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

  /// Convert to JSON for Firestore (snake_case sesuai Kepmendagri standard)
  Map<String, dynamic> toJson() {
    return {
      'user_id': userId,
      'purpose': purpose.name, // Store as string: 'shipping' or 'sender'
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

  /// Create from JSON (backward compatible: supports both camelCase and snake_case)
  /// Migrates old label-based data to purpose-based system
  factory AddressEntity.fromJson(Map<String, dynamic> json, String id) {
    // Backward compatibility: migrate old 'label' to 'purpose'
    AddressPurpose purpose;
    if (json.containsKey('purpose')) {
      purpose = AddressPurpose.values.byName(json['purpose'] as String);
    } else {
      // Migration: old label to purpose
      final oldLabel = json['label'] as String?;
      purpose = _migrateLabelToPurpose(oldLabel);
    }

    return AddressEntity(
      id: id,
      userId: (json['userId'] ?? json['user_id']) as String,
      purpose: purpose,
      nickname: json['nickname'] as String?,
      // Backward compatibility: fallback to empty strings for old data
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

  /// Migrate old label-based system to purpose-based
  static AddressPurpose _migrateLabelToPurpose(String? oldLabel) {
    if (oldLabel == null) return AddressPurpose.shipping;

    // Farm addresses are typically sender addresses for sellers
    if (oldLabel.toLowerCase() == 'farm' ||
        oldLabel.toLowerCase() == 'warehouse') {
      return AddressPurpose.sender;
    }

    // Home, Office, Other are shipping addresses (destinations)
    return AddressPurpose.shipping;
  }

  @override
  String toString() {
    return 'AddressEntity(id: $id, purpose: ${purpose.name}, nickname: $nickname, isPrimary: $isPrimary, address: $fullAddress)';
  }
}
