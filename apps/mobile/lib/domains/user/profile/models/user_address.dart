import 'package:labuda/shared/shared.dart';

/// User Address Model untuk address screen
class UserAddress {
  final String? streetAddress;
  final String? postalCode;
  final String? country;
  final Province? province;
  final City? city;
  final District? district;
  final Village? village;
  final String? phoneNumber;
  final bool phoneVerified;
  final DateTime? phoneVerifiedAt;

  // DEPRECATED: KTP fields moved to KYC module
  // Use KycEntity from lib/features/kyc/ instead
  @Deprecated('Use KycEntity from kyc module')
  final String? ktpNumber;
  @Deprecated('Use KycEntity from kyc module')
  final String? ktpName;
  @Deprecated('Use KycEntity from kyc module')
  final String? ktpImageUrl;

  final bool sameAsBilling;

  const UserAddress({
    this.streetAddress,
    this.postalCode,
    this.country,
    this.province,
    this.city,
    this.district,
    this.village,
    this.phoneNumber,
    this.phoneVerified = false,
    this.phoneVerifiedAt,
    this.ktpNumber,
    this.ktpName,
    this.ktpImageUrl,
    this.sameAsBilling = true,
  });

  UserAddress copyWith({
    String? streetAddress,
    String? postalCode,
    String? country,
    Province? province,
    City? city,
    District? district,
    Village? village,
    String? phoneNumber,
    bool? phoneVerified,
    DateTime? phoneVerifiedAt,
    String? ktpNumber,
    String? ktpName,
    String? ktpImageUrl,
    bool? sameAsBilling,
  }) {
    return UserAddress(
      streetAddress: streetAddress ?? this.streetAddress,
      postalCode: postalCode ?? this.postalCode,
      country: country ?? this.country,
      province: province ?? this.province,
      city: city ?? this.city,
      district: district ?? this.district,
      village: village ?? this.village,
      phoneNumber: phoneNumber ?? this.phoneNumber,
      phoneVerified: phoneVerified ?? this.phoneVerified,
      phoneVerifiedAt: phoneVerifiedAt ?? this.phoneVerifiedAt,
      ktpNumber: ktpNumber ?? this.ktpNumber,
      ktpName: ktpName ?? this.ktpName,
      ktpImageUrl: ktpImageUrl ?? this.ktpImageUrl,
      sameAsBilling: sameAsBilling ?? this.sameAsBilling,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'street_address': streetAddress,
      'postal_code': postalCode,
      'country': country,
      'province': province?.toJson(),
      'city': city?.toJson(),
      'district': district?.toJson(),
      'village': village?.toJson(),
      'phone_number': phoneNumber,
      'phone_verified': phoneVerified,
      'phone_verified_at': phoneVerifiedAt?.toIso8601String(),
      'ktp_number': ktpNumber,
      'ktp_name': ktpName,
      'ktp_image_url': ktpImageUrl,
      'same_as_billing': sameAsBilling,
    };
  }

  factory UserAddress.fromJson(Map<String, dynamic> json) {
    return UserAddress(
      streetAddress: json['streetAddress'] ?? json['street_address'],
      postalCode: json['postalCode'] ?? json['postal_code'],
      country: json['country'],
      province: json['province'] != null
          ? Province.fromJson(json['province'])
          : null,
      city: json['city'] != null ? City.fromJson(json['city']) : null,
      district: json['district'] != null
          ? District.fromJson(json['district'])
          : null,
      village: json['village'] != null
          ? Village.fromJson(json['village'])
          : null,
      phoneNumber: json['phoneNumber'] ?? json['phone_number'],
      phoneVerified: json['phoneVerified'] ?? json['phone_verified'] ?? false,
      phoneVerifiedAt:
          json['phoneVerifiedAt'] ?? json['phone_verified_at'] != null
          ? DateTime.parse(json['phoneVerifiedAt'] ?? json['phone_verified_at'])
          : null,
      ktpNumber: json['ktpNumber'] ?? json['ktp_number'],
      ktpName: json['ktpName'] ?? json['ktp_name'],
      ktpImageUrl: json['ktpImageUrl'] ?? json['ktp_image_url'],
      sameAsBilling: json['sameAsBilling'] ?? json['same_as_billing'] ?? true,
    );
  }

  @override
  String toString() {
    return 'UserAddress('
        'streetAddress: $streetAddress, '
        'postalCode: $postalCode, '
        'province: ${province?.name}, '
        'city: ${city?.name}, '
        'district: ${district?.name}, '
        'village: ${village?.name}'
        ')';
  }
}
