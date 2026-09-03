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
      createdAt: dto.createdAt,
      updatedAt: dto.updatedAt,
    );
  }

  /// Convert Entity to DTO (for create/update requests)
  static Map<String, dynamic> toCreateJson(
    CreateShippingSetupRequest request,
  ) {
    return {
      'name': request.name,
      'transport_type': request.type.name,
    };
  }

  /// Convert Entity to DTO (for update requests)
  static Map<String, dynamic> toUpdateJson(
    UpdateShippingSetupRequest request,
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
  /// Convert DTO to Entity
  static ShippingCoverage toEntity(ShippingCoverageDto dto) {
    return ShippingCoverage(
      provinceId: dto.provinceCode,
      provinceName: dto.provinceName,
      provinceRate: dto.rate,
      isAvailable: dto.isAvailable,
    );
  }

  /// Convert Entity to Add Coverage Request JSON
  static Map<String, dynamic> toAddCoverageJson(AddCoverageRequest request) {
    return request.toJson();
  }

  /// Convert Entity to Update Coverage Request JSON
  static Map<String, dynamic> toUpdateCoverageJson(
    UpdateCoverageRequest request,
  ) {
    return request.toJson();
  }

  /// Convert list of DTOs to Entities
  static List<ShippingCoverage> toEntityList(List<ShippingCoverageDto> dtos) {
    return dtos.map((dto) => toEntity(dto)).toList();
  }
}

/// Mapper untuk Delivery Option DTO → Entity
class DeliveryOptionMapper {
  /// Convert DTO to Entity
  static DeliveryOption toEntity(DeliveryOptionDto dto) {
    return DeliveryOption(
      shippingSetupId: dto.shippingSetupId,
      displayName: dto.displayName,
      type: dto.type,
      rate: dto.rate,
      notes: dto.notes,
      source: dto.source,
    );
  }

  /// Convert Check Delivery Request to JSON
  static Map<String, dynamic> checkDeliveryToJson(
    CheckDeliveryRequest request,
  ) {
    return {
      'product_id': request.productId,
      'province_id': request.provinceId,
      'city_id': request.cityId,
      'city_name': request.cityName,
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
