import 'package:hishumi/domains/user/profile/data/models/api/address_api_models.dart';
import 'package:hishumi/domains/user/profile/domain/entities/address_entity.dart';
import 'package:hishumi/shared/shared.dart';

/// Mapper for converting between Address API models and domain entities
class AddressApiMapper {
  /// Convert API response to domain entity
  static AddressEntity toDomain(AddressResponseApi api) {
    return AddressEntity(
      id: api.id,
      userId: api.userId,
      nickname: api.nickname,
      recipientName: api.recipientName,
      phone: api.phone,
      province: Province(id: api.provinceId, name: api.provinceName),
      city: City(
        id: api.cityId,
        name: api.cityName,
        provinceId: api.provinceId,
      ),
      district: District(
        id: api.districtId,
        name: api.districtName,
        cityId: api.cityId,
      ),
      village: Village(
        id: api.villageId,
        name: api.villageName,
        districtId: api.districtId,
      ),
      streetAddress: api.streetAddress,
      postalCode: api.postalCode,
      notes: api.notes,
      isPrimary: api.isPrimary,
      latitude: api.latitude,
      longitude: api.longitude,
      createdAt: api.createdAt,
      updatedAt: api.updatedAt,
    );
  }

  /// Convert domain entity to create request API model
  static CreateAddressRequestApi toCreateRequest(AddressEntity entity) {
    return CreateAddressRequestApi(
      nickname: entity.nickname,
      recipientName: entity.recipientName,
      phone: entity.phone,
      provinceId: entity.province.id,
      provinceName: entity.province.name,
      cityId: entity.city.id,
      cityName: entity.city.name,
      districtId: entity.district.id,
      districtName: entity.district.name,
      villageId: entity.village.id,
      villageName: entity.village.name,
      streetAddress: entity.streetAddress,
      postalCode: entity.postalCode,
      notes: entity.notes,
      isPrimary: entity.isPrimary,
      latitude: entity.latitude,
      longitude: entity.longitude,
    );
  }

  /// Convert partial domain entity updates to update request API model
  static UpdateAddressRequestApi toUpdateRequest(Map<String, dynamic> updates) {
    return UpdateAddressRequestApi(
      nickname: updates['nickname'] as String?,
      recipientName: updates['recipientName'] as String?,
      phone: updates['phone'] as String?,
      provinceId: updates['provinceId'] as String?,
      provinceName: updates['provinceName'] as String?,
      cityId: updates['cityId'] as String?,
      cityName: updates['cityName'] as String?,
      districtId: updates['districtId'] as String?,
      districtName: updates['districtName'] as String?,
      villageId: updates['villageId'] as String?,
      villageName: updates['villageName'] as String?,
      streetAddress: updates['streetAddress'] as String?,
      postalCode: updates['postalCode'] as String?,
      notes: updates['notes'] as String?,
      latitude: updates['latitude'] as double?,
      longitude: updates['longitude'] as double?,
    );
  }
}
