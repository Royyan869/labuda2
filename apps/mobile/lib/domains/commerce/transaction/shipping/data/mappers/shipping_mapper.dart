import 'package:labuda/domains/commerce/transaction/shipping/data/dto/shipping_dto.dart';
import 'package:labuda/domains/commerce/transaction/shipping/domain/domain.dart';

/// Mapper untuk Shipping Option Entity ↔ DTO
class ShippingSetupMapper {
  /// Convert DTO to Entity
  static ShippingSetup toEntity(ShippingSetupDto dto) {
    return ShippingSetup(
      id: dto.id,
      name: dto.name,
      type: ShippingType.fromString(dto.type) ?? ShippingType.custom,
      coverageAreas:
          dto.coverages
              ?.map((c) => ShippingCoverageMapper.toEntity(c))
              .toList() ??
          const [],
      isActive: dto.isActive,
      // Seller-private note (internal_purpose). Never render on buyer surfaces.
      internalNote: dto.internalPurpose,
      createdAt: dto.createdAt,
      updatedAt: dto.updatedAt,
    );
  }

  /// Convert a one-package create request to the canonical wire payload:
  /// identity (name, transport_type, internal_purpose) + destinations
  /// (provinces with all-in shipping+packing rates + city qualifications).
  static Map<String, dynamic> toCreateJson(
    CreateShippingSetupRequest request,
  ) {
    return request.toJson();
  }

  /// Convert list of DTOs to Entities
  static List<ShippingSetup> toEntityList(List<ShippingSetupDto> dtos) {
    return dtos.map((dto) => toEntity(dto)).toList();
  }
}

/// Mapper untuk Shipping Coverage Entity ↔ DTO
class ShippingCoverageMapper {
  /// Convert DTO to Entity (hydrates city qualifications)
  static ShippingCoverage toEntity(ShippingCoverageDto dto) {
    return ShippingCoverage(
      provinceId: dto.provinceCode,
      provinceName: dto.provinceName,
      provinceRate: dto.rate,
      isAvailable: dto.isAvailable,
      cityOverrides:
          dto.cityQualifications
              .map(
                (city) => CityShippingRate(
                  cityId: city.cityCode,
                  cityName: city.cityName,
                  rate: city.rate ?? 0,
                  excluded: city.isAvailable == false,
                ),
              )
              .toList(growable: false),
    );
  }

  /// Convert list of DTOs to Entities
  static List<ShippingCoverage> toEntityList(List<ShippingCoverageDto> dtos) {
    return dtos.map((dto) => toEntity(dto)).toList();
  }
}

/// Mapper untuk Delivery Option DTO → Entity
///
/// Wire vocabulary (`shipping_option_id`, `name`, `transport_type`) is translated
/// here onto the canonical shipping domain vocabulary (`shippingSetupId`,
/// `displayName`, `type`).
class DeliveryOptionMapper {
  /// Convert DTO to Entity
  static DeliveryOption toEntity(DeliveryOptionDto dto) {
    return DeliveryOption(
      shippingSetupId: dto.shippingOptionId,
      displayName: dto.name,
      type: dto.transportType,
      rate: dto.rate,
    );
  }

  /// Convert Check Delivery Request to JSON
  ///
  /// Canonical wire keys for POST /api/v1/shipping/check:
  ///   product_id (uuid), province_code (2-digit BPS), city_code (4-digit BPS).
  /// The request carries no city name — the backend resolves coverage from codes.
  static Map<String, dynamic> checkDeliveryToJson(
    CheckDeliveryRequest request,
  ) {
    return {
      'product_id': request.productId,
      'province_code': request.provinceId,
      'city_code': request.cityId,
    };
  }

  /// Convert list of DTOs to Entities
  static List<DeliveryOption> toEntityList(List<DeliveryOptionDto> dtos) {
    return dtos.map((dto) => toEntity(dto)).toList();
  }
}

/// Mapper untuk Shipping Proof Entity ↔ DTO
class ShippingProofMapper {
  /// Convert DTO to Entity
  static ShippingProof toEntity(ShippingProofDto dto) {
    return ShippingProof(
      id: dto.id,
      orderId: dto.orderId,
      sellerId: dto.sellerId,
      photos: dto.photos,
      videos: dto.videos,
      shippingReference: dto.shippingReference,
      referenceType: dto.referenceType,
      shippingNote: dto.shippingNote,
      courierPhone: dto.courierPhone,
      formattedCourierPhone: dto.formattedCourierPhone,
      createdAt: dto.createdAt,
      updatedAt: dto.updatedAt,
    );
  }

  /// Convert Create Request to JSON
  static Map<String, dynamic> createToJson(CreateShippingProofRequest request) {
    return request.toJson();
  }

  /// Convert Update Request to JSON
  static Map<String, dynamic> updateToJson(UpdateShippingProofRequest request) {
    return request.toJson();
  }
}
