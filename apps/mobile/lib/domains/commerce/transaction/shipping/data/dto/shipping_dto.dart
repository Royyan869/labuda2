import 'package:equatable/equatable.dart';

// =====================================
// Shipping Option DTOs
// =====================================

/// Shipping Option API DTO
class ShippingSetupDto extends Equatable {
  final String id;
  final String name;
  final String type;
  final bool isActive;
  final List<ShippingCoverageDto>? coverages;
  final DateTime createdAt;
  final DateTime updatedAt;

  const ShippingSetupDto({
    required this.id,
    required this.name,
    required this.type,
    required this.isActive,
    this.coverages,
    required this.createdAt,
    required this.updatedAt,
  });

  factory ShippingSetupDto.fromJson(Map<String, dynamic> json) {
    return ShippingSetupDto(
      id: json['id'] as String,
      name: json['name'] as String,
      type: json['transport_type'] as String,
      isActive: json['is_active'] as bool,
      coverages: (json['coverages'] as List<dynamic>?)
          ?.map((e) => ShippingCoverageDto.fromJson(e as Map<String, dynamic>))
          .toList(),
      createdAt: DateTime.parse(json['created_at'] as String),
      updatedAt: DateTime.parse(json['updated_at'] as String),
    );
  }

  Map<String, dynamic> toJson() => {
    'id': id,
    'name': name,
    'transport_type': type,
    'is_active': isActive,
    if (coverages != null)
      'coverages': coverages!.map((e) => e.toJson()).toList(),
    'created_at': createdAt.toIso8601String(),
    'updated_at': updatedAt.toIso8601String(),
  };

  @override
  List<Object?> get props => [
    id,
    name,
    type,
    isActive,
    createdAt,
    updatedAt,
  ];
}

/// Shipping Coverage API DTO
class ShippingCoverageDto extends Equatable {
  final String id;
  final String shippingSetupId;
  final String provinceCode;
  final String provinceName;
  final double rate;
  final bool isAvailable;
  final DateTime createdAt;

  const ShippingCoverageDto({
    required this.id,
    required this.shippingSetupId,
    required this.provinceCode,
    required this.provinceName,
    required this.rate,
    required this.isAvailable,
    required this.createdAt,
  });

  factory ShippingCoverageDto.fromJson(Map<String, dynamic> json) {
    return ShippingCoverageDto(
      id: json['id'] as String,
      shippingSetupId: json['shipping_option_id'] as String,
      provinceCode: json['province_code'] as String,
      provinceName: json['province_name'] as String,
      rate: (json['rate'] as num).toDouble(),
      isAvailable: json['is_available'] as bool? ?? true,
      createdAt: DateTime.parse(json['created_at'] as String),
    );
  }

  Map<String, dynamic> toJson() => {
    'id': id,
    'shipping_option_id': shippingSetupId,
    'province_code': provinceCode,
    'province_name': provinceName,
    'rate': rate,
    'is_available': isAvailable,
    'created_at': createdAt.toIso8601String(),
  };

  @override
  List<Object?> get props => [
    id,
    shippingSetupId,
    provinceCode,
    provinceName,
    rate,
    isAvailable,
    createdAt,
  ];
}

/// City Rate API DTO
class CityRateDto extends Equatable {
  final String id;
  final String cityId;
  final String cityName;
  final double rate;
  final String? notes;

  const CityRateDto({
    required this.id,
    required this.cityId,
    required this.cityName,
    required this.rate,
    this.notes,
  });

  factory CityRateDto.fromJson(Map<String, dynamic> json) {
    return CityRateDto(
      id: json['id'] as String,
      cityId: json['city_id'] as String,
      cityName: json['city_name'] as String,
      rate: (json['rate'] as num).toDouble(),
      notes: json['notes'] as String?,
    );
  }

  Map<String, dynamic> toJson() => {
    'id': id,
    'city_id': cityId,
    'city_name': cityName,
    'rate': rate,
    if (notes != null) 'notes': notes,
  };

  @override
  List<Object?> get props => [id, cityId, cityName, rate];
}

/// Seller shipping options list envelope.
class SellerShippingSetupsEnvelopeDto extends Equatable {
  final List<ShippingSetupDto> shippingSetups;
  final int count;

  const SellerShippingSetupsEnvelopeDto({
    required this.shippingSetups,
    required this.count,
  });

  factory SellerShippingSetupsEnvelopeDto.fromJson(Map<String, dynamic> json) {
    final rawOptions = json['shipping_options'];
    if (rawOptions is! List) {
      throw FormatException(
        'Expected shipping_options to be a list, got ${rawOptions.runtimeType}',
      );
    }

    return SellerShippingSetupsEnvelopeDto(
      shippingSetups: rawOptions
          .map((e) => ShippingSetupDto.fromJson(e as Map<String, dynamic>))
          .toList(growable: false),
      count: (json['count'] as num?)?.toInt() ?? rawOptions.length,
    );
  }

  @override
  List<Object?> get props => [shippingSetups, count];
}

// =====================================
// Delivery Check DTOs
// =====================================

/// Check Delivery Response DTO
///
/// Mirrors the canonical wire contract of POST /api/v1/shipping/check
/// (backend/internal/commerce/shipping/delivery/http/shipping_handler.go).
/// `productConfigured` / `count` are wire facts only: no read-time UI state is
/// derived from them yet.
class CheckDeliveryResponseDto extends Equatable {
  final String productId;
  final String province;
  final String city;
  final List<DeliveryOptionDto> options;
  final int count;
  final bool productConfigured;

  const CheckDeliveryResponseDto({
    required this.productId,
    required this.province,
    required this.city,
    required this.options,
    required this.count,
    required this.productConfigured,
  });

  factory CheckDeliveryResponseDto.fromJson(Map<String, dynamic> json) {
    return CheckDeliveryResponseDto(
      productId: json['product_id'] as String,
      province: json['province'] as String,
      city: json['city'] as String,
      options: (json['options'] as List<dynamic>)
          .map((e) => DeliveryOptionDto.fromJson(e as Map<String, dynamic>))
          .toList(),
      count: json['count'] as int,
      productConfigured: json['product_configured'] as bool,
    );
  }

  Map<String, dynamic> toJson() => {
    'product_id': productId,
    'province': province,
    'city': city,
    'options': options.map((e) => e.toJson()).toList(),
    'count': count,
    'product_configured': productConfigured,
  };

  @override
  List<Object?> get props => [
    productId,
    province,
    city,
    options,
    count,
    productConfigured,
  ];
}

/// Delivery Option API DTO — one deliverable option in the canonical wire shape.
class DeliveryOptionDto extends Equatable {
  final String shippingOptionId;
  final String name;
  final String transportType;
  final double rate;
  final bool isAvailable;

  const DeliveryOptionDto({
    required this.shippingOptionId,
    required this.name,
    required this.transportType,
    required this.rate,
    required this.isAvailable,
  });

  factory DeliveryOptionDto.fromJson(Map<String, dynamic> json) {
    return DeliveryOptionDto(
      shippingOptionId: json['shipping_option_id'] as String,
      name: json['name'] as String,
      transportType: json['transport_type'] as String,
      rate: (json['rate'] as num).toDouble(),
      isAvailable: json['is_available'] as bool,
    );
  }

  Map<String, dynamic> toJson() => {
    'shipping_option_id': shippingOptionId,
    'name': name,
    'transport_type': transportType,
    'rate': rate,
    'is_available': isAvailable,
  };

  @override
  List<Object?> get props => [
    shippingOptionId,
    name,
    transportType,
    rate,
    isAvailable,
  ];
}

// =====================================
// Shipping Proof DTOs
// =====================================

/// Shipping Proof API DTO
class ShippingProofDto extends Equatable {
  final String id;
  final String orderId;
  final String sellerId;
  final List<String> photos;
  final List<String> videos;
  final String? shippingReference;
  final String? referenceType;
  final String? shippingNote;
  final String? courierPhone;
  final String? formattedCourierPhone;
  final DateTime createdAt;
  final DateTime updatedAt;

  const ShippingProofDto({
    required this.id,
    required this.orderId,
    required this.sellerId,
    required this.photos,
    required this.videos,
    this.shippingReference,
    this.referenceType,
    this.shippingNote,
    this.courierPhone,
    this.formattedCourierPhone,
    required this.createdAt,
    required this.updatedAt,
  });

  factory ShippingProofDto.fromJson(Map<String, dynamic> json) {
    return ShippingProofDto(
      id: json['id'] as String,
      orderId: json['order_id'] as String,
      sellerId: json['seller_id'] as String,
      photos: (json['photos'] as List<dynamic>).cast<String>(),
      videos: (json['videos'] as List<dynamic>).cast<String>(),
      shippingReference:
          json['shipping_reference'] as String? ??
          json['tracking_number'] as String?, // Backward compatibility
      referenceType: json['reference_type'] as String?,
      shippingNote: json['shipping_note'] as String?,
      courierPhone: json['courier_phone'] as String?,
      formattedCourierPhone: json['formatted_courier_phone'] as String?,
      createdAt: DateTime.parse(json['created_at'] as String),
      updatedAt: DateTime.parse(json['updated_at'] as String),
    );
  }

  Map<String, dynamic> toJson() => {
    'id': id,
    'order_id': orderId,
    'seller_id': sellerId,
    'photos': photos,
    'videos': videos,
    if (shippingReference != null) 'shipping_reference': shippingReference,
    if (referenceType != null) 'reference_type': referenceType,
    if (shippingNote != null) 'shipping_note': shippingNote,
    if (courierPhone != null) 'courier_phone': courierPhone,
    if (formattedCourierPhone != null)
      'formatted_courier_phone': formattedCourierPhone,
    'created_at': createdAt.toIso8601String(),
    'updated_at': updatedAt.toIso8601String(),
  };

  @override
  List<Object?> get props => [id, orderId, sellerId, createdAt];
}
