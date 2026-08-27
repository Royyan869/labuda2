import 'package:equatable/equatable.dart';

// =====================================
// Shipping Option DTOs
// =====================================

/// Shipping Option API DTO
class ShippingOptionDto extends Equatable {
  final String id;
  final String name;
  final String type;
  final String? expeditionName;
  final bool isActive;
  final List<ShippingCoverageDto>? coverages;
  final DateTime createdAt;
  final DateTime updatedAt;

  const ShippingOptionDto({
    required this.id,
    required this.name,
    required this.type,
    this.expeditionName,
    required this.isActive,
    this.coverages,
    required this.createdAt,
    required this.updatedAt,
  });

  factory ShippingOptionDto.fromJson(Map<String, dynamic> json) {
    return ShippingOptionDto(
      id: json['id'] as String,
      name: json['name'] as String,
      type: json['transport_type'] as String,
      expeditionName: json['expedition_name'] as String?,
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
    if (expeditionName != null) 'expedition_name': expeditionName,
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
    expeditionName,
    isActive,
    createdAt,
    updatedAt,
  ];
}

/// Shipping Coverage API DTO
class ShippingCoverageDto extends Equatable {
  final String id;
  final String shippingOptionId;
  final String provinceCode;
  final String provinceName;
  final double rate;
  final String? estimatedDays;
  final bool isAvailable;
  final DateTime createdAt;

  const ShippingCoverageDto({
    required this.id,
    required this.shippingOptionId,
    required this.provinceCode,
    required this.provinceName,
    required this.rate,
    this.estimatedDays,
    required this.isAvailable,
    required this.createdAt,
  });

  factory ShippingCoverageDto.fromJson(Map<String, dynamic> json) {
    return ShippingCoverageDto(
      id: json['id'] as String,
      shippingOptionId: json['shipping_option_id'] as String,
      provinceCode: json['province_code'] as String,
      provinceName: json['province_name'] as String,
      rate: (json['rate'] as num).toDouble(),
      estimatedDays: json['estimated_days'] as String?,
      isAvailable: json['is_available'] as bool? ?? true,
      createdAt: DateTime.parse(json['created_at'] as String),
    );
  }

  Map<String, dynamic> toJson() => {
    'id': id,
    'shipping_option_id': shippingOptionId,
    'province_code': provinceCode,
    'province_name': provinceName,
    'rate': rate,
    if (estimatedDays != null) 'estimated_days': estimatedDays,
    'is_available': isAvailable,
    'created_at': createdAt.toIso8601String(),
  };

  @override
  List<Object?> get props => [
    id,
    shippingOptionId,
    provinceCode,
    provinceName,
    rate,
    estimatedDays,
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
  final String? estimatedDays;
  final String? notes;

  const CityRateDto({
    required this.id,
    required this.cityId,
    required this.cityName,
    required this.rate,
    this.estimatedDays,
    this.notes,
  });

  factory CityRateDto.fromJson(Map<String, dynamic> json) {
    return CityRateDto(
      id: json['id'] as String,
      cityId: json['city_id'] as String,
      cityName: json['city_name'] as String,
      rate: (json['rate'] as num).toDouble(),
      estimatedDays: json['estimated_days'] as String?,
      notes: json['notes'] as String?,
    );
  }

  Map<String, dynamic> toJson() => {
    'id': id,
    'city_id': cityId,
    'city_name': cityName,
    'rate': rate,
    if (estimatedDays != null) 'estimated_days': estimatedDays,
    if (notes != null) 'notes': notes,
  };

  @override
  List<Object?> get props => [id, cityId, cityName, rate];
}

/// Seller shipping options list envelope.
class SellerShippingOptionsEnvelopeDto extends Equatable {
  final List<ShippingOptionDto> shippingOptions;
  final int count;

  const SellerShippingOptionsEnvelopeDto({
    required this.shippingOptions,
    required this.count,
  });

  factory SellerShippingOptionsEnvelopeDto.fromJson(Map<String, dynamic> json) {
    final rawOptions = json['shipping_options'];
    if (rawOptions is! List) {
      throw FormatException(
        'Expected shipping_options to be a list, got ${rawOptions.runtimeType}',
      );
    }

    return SellerShippingOptionsEnvelopeDto(
      shippingOptions: rawOptions
          .map((e) => ShippingOptionDto.fromJson(e as Map<String, dynamic>))
          .toList(growable: false),
      count: (json['count'] as num?)?.toInt() ?? rawOptions.length,
    );
  }

  @override
  List<Object?> get props => [shippingOptions, count];
}

// =====================================
// Delivery Check DTOs
// =====================================

/// Check Delivery Response DTO
class CheckDeliveryResponseDto extends Equatable {
  final bool available;
  final List<DeliveryOptionDto> options;

  const CheckDeliveryResponseDto({
    required this.available,
    required this.options,
  });

  factory CheckDeliveryResponseDto.fromJson(Map<String, dynamic> json) {
    return CheckDeliveryResponseDto(
      available: json['available'] as bool,
      options: (json['options'] as List<dynamic>)
          .map((e) => DeliveryOptionDto.fromJson(e as Map<String, dynamic>))
          .toList(),
    );
  }

  Map<String, dynamic> toJson() => {
    'available': available,
    'options': options.map((e) => e.toJson()).toList(),
  };

  @override
  List<Object?> get props => [available, options];
}

/// Delivery Option API DTO
class DeliveryOptionDto extends Equatable {
  final String shippingOptionId;
  final String displayName;
  final String type;
  final double rate;
  final String? estimatedDays;
  final String? notes;
  final String source;

  const DeliveryOptionDto({
    required this.shippingOptionId,
    required this.displayName,
    required this.type,
    required this.rate,
    this.estimatedDays,
    this.notes,
    required this.source,
  });

  factory DeliveryOptionDto.fromJson(Map<String, dynamic> json) {
    return DeliveryOptionDto(
      shippingOptionId: json['shipping_option_id'] as String,
      displayName: json['display_name'] as String,
      type: json['type'] as String,
      rate: (json['rate'] as num).toDouble(),
      estimatedDays: json['estimated_days'] as String?,
      notes: json['notes'] as String?,
      source: json['source'] as String,
    );
  }

  Map<String, dynamic> toJson() => {
    'shipping_option_id': shippingOptionId,
    'display_name': displayName,
    'type': type,
    'rate': rate,
    if (estimatedDays != null) 'estimated_days': estimatedDays,
    if (notes != null) 'notes': notes,
    'source': source,
  };

  @override
  List<Object?> get props => [shippingOptionId, type, rate, source];
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
