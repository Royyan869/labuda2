import 'package:json_annotation/json_annotation.dart';

part 'address_api_models.g.dart';

/// Address purpose enum matching backend
enum AddressPurposeApi {
  @JsonValue('shipping')
  shipping,
  @JsonValue('sender')
  sender,
}

/// Extension to convert API enum to domain enum
extension AddressPurposeApiExt on AddressPurposeApi {
  String get value {
    switch (this) {
      case AddressPurposeApi.shipping:
        return 'shipping';
      case AddressPurposeApi.sender:
        return 'sender';
    }
  }
}

// =====================================
// Request DTOs
// =====================================

/// Create address request matching backend
@JsonSerializable()
class CreateAddressRequestApi {
  final String purpose;
  final String? nickname;
  @JsonKey(name: 'recipient_name')
  final String recipientName;
  final String phone;
  @JsonKey(name: 'province_id')
  final String provinceId;
  @JsonKey(name: 'province_name')
  final String provinceName;
  @JsonKey(name: 'city_id')
  final String cityId;
  @JsonKey(name: 'city_name')
  final String cityName;
  @JsonKey(name: 'district_id')
  final String districtId;
  @JsonKey(name: 'district_name')
  final String districtName;
  @JsonKey(name: 'village_id')
  final String villageId;
  @JsonKey(name: 'village_name')
  final String villageName;
  @JsonKey(name: 'street_address')
  final String streetAddress;
  @JsonKey(name: 'postal_code')
  final String postalCode;
  final String? notes;
  @JsonKey(name: 'is_primary')
  final bool isPrimary;
  final double? latitude;
  final double? longitude;

  const CreateAddressRequestApi({
    required this.purpose,
    this.nickname,
    required this.recipientName,
    required this.phone,
    required this.provinceId,
    required this.provinceName,
    required this.cityId,
    required this.cityName,
    required this.districtId,
    required this.districtName,
    required this.villageId,
    required this.villageName,
    required this.streetAddress,
    required this.postalCode,
    this.notes,
    this.isPrimary = false,
    this.latitude,
    this.longitude,
  });

  factory CreateAddressRequestApi.fromJson(Map<String, dynamic> json) =>
      _$CreateAddressRequestApiFromJson(json);

  Map<String, dynamic> toJson() => _$CreateAddressRequestApiToJson(this);
}

/// Update address request matching backend
@JsonSerializable()
class UpdateAddressRequestApi {
  final String? nickname;
  @JsonKey(name: 'recipient_name')
  final String? recipientName;
  final String? phone;
  @JsonKey(name: 'province_id')
  final String? provinceId;
  @JsonKey(name: 'province_name')
  final String? provinceName;
  @JsonKey(name: 'city_id')
  final String? cityId;
  @JsonKey(name: 'city_name')
  final String? cityName;
  @JsonKey(name: 'district_id')
  final String? districtId;
  @JsonKey(name: 'district_name')
  final String? districtName;
  @JsonKey(name: 'village_id')
  final String? villageId;
  @JsonKey(name: 'village_name')
  final String? villageName;
  @JsonKey(name: 'street_address')
  final String? streetAddress;
  @JsonKey(name: 'postal_code')
  final String? postalCode;
  final String? notes;
  final double? latitude;
  final double? longitude;

  const UpdateAddressRequestApi({
    this.nickname,
    this.recipientName,
    this.phone,
    this.provinceId,
    this.provinceName,
    this.cityId,
    this.cityName,
    this.districtId,
    this.districtName,
    this.villageId,
    this.villageName,
    this.streetAddress,
    this.postalCode,
    this.notes,
    this.latitude,
    this.longitude,
  });

  factory UpdateAddressRequestApi.fromJson(Map<String, dynamic> json) =>
      _$UpdateAddressRequestApiFromJson(json);

  Map<String, dynamic> toJson() => _$UpdateAddressRequestApiToJson(this);
}

// =====================================
// Response DTOs
// =====================================

/// Address response matching backend
@JsonSerializable()
class AddressResponseApi {
  final String id;
  @JsonKey(name: 'user_id')
  final String userId;
  final String purpose;
  @JsonKey(name: 'purpose_label')
  final String purposeLabel;
  final String? nickname;
  @JsonKey(name: 'display_label')
  final String displayLabel;
  @JsonKey(name: 'recipient_name')
  final String recipientName;
  final String phone;
  @JsonKey(name: 'province_id')
  final String provinceId;
  @JsonKey(name: 'province_name')
  final String provinceName;
  @JsonKey(name: 'city_id')
  final String cityId;
  @JsonKey(name: 'city_name')
  final String cityName;
  @JsonKey(name: 'district_id')
  final String districtId;
  @JsonKey(name: 'district_name')
  final String districtName;
  @JsonKey(name: 'village_id')
  final String villageId;
  @JsonKey(name: 'village_name')
  final String villageName;
  @JsonKey(name: 'street_address')
  final String streetAddress;
  @JsonKey(name: 'postal_code')
  final String postalCode;
  final String? notes;
  @JsonKey(name: 'is_primary')
  final bool isPrimary;
  @JsonKey(name: 'is_available_for_checkout')
  final bool isAvailableForCheckout;
  final double? latitude;
  final double? longitude;
  @JsonKey(name: 'has_coordinates')
  final bool hasCoordinates;
  @JsonKey(name: 'full_address')
  final String fullAddress;
  @JsonKey(name: 'created_at')
  final DateTime createdAt;
  @JsonKey(name: 'updated_at')
  final DateTime updatedAt;

  const AddressResponseApi({
    required this.id,
    required this.userId,
    required this.purpose,
    required this.purposeLabel,
    this.nickname,
    required this.displayLabel,
    required this.recipientName,
    required this.phone,
    required this.provinceId,
    required this.provinceName,
    required this.cityId,
    required this.cityName,
    required this.districtId,
    required this.districtName,
    required this.villageId,
    required this.villageName,
    required this.streetAddress,
    required this.postalCode,
    this.notes,
    required this.isPrimary,
    required this.isAvailableForCheckout,
    this.latitude,
    this.longitude,
    required this.hasCoordinates,
    required this.fullAddress,
    required this.createdAt,
    required this.updatedAt,
  });

  factory AddressResponseApi.fromJson(Map<String, dynamic> json) =>
      _$AddressResponseApiFromJson(json);

  Map<String, dynamic> toJson() => _$AddressResponseApiToJson(this);
}

/// Address list response matching backend
@JsonSerializable()
class AddressListResponseApi {
  final List<AddressResponseApi> data;
  final int total;

  const AddressListResponseApi({required this.data, required this.total});

  factory AddressListResponseApi.fromJson(Map<String, dynamic> json) =>
      _$AddressListResponseApiFromJson(json);

  Map<String, dynamic> toJson() => _$AddressListResponseApiToJson(this);
}

/// Address count response matching backend
@JsonSerializable()
class AddressCountResponseApi {
  final int total;
  @JsonKey(name: 'shipping_count')
  final int shippingCount;
  @JsonKey(name: 'sender_count')
  final int senderCount;

  const AddressCountResponseApi({
    required this.total,
    required this.shippingCount,
    required this.senderCount,
  });

  factory AddressCountResponseApi.fromJson(Map<String, dynamic> json) =>
      _$AddressCountResponseApiFromJson(json);

  Map<String, dynamic> toJson() => _$AddressCountResponseApiToJson(this);
}
