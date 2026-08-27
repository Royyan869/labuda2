import 'dart:async';

import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/profile/data/datasources/address_api_datasource.dart';
import 'package:labuda/domains/user/profile/data/mappers/address_api_mapper.dart';
import 'package:labuda/domains/user/profile/domain/entities/address_entity.dart';
import 'package:labuda/domains/user/profile/domain/repositories/i_address_repository.dart';

/// API implementation of address repository
class AddressRepositoryApi implements IAddressRepository {
  final AddressApiDatasource _datasource;

  AddressRepositoryApi(this._datasource);

  @override
  Future<Result<List<AddressEntity>>> getAddressesByUserId(
    String userId,
  ) async {
    final result = await _datasource.getAddresses();

    return result.fold((error) => Result.error(error), (response) {
      final addresses = response.data.map(AddressApiMapper.toDomain).toList();
      return Result.success(addresses);
    });
  }

  @override
  Future<Result<List<AddressEntity>>> getAddressesByPurpose(
    String userId,
    AddressPurpose purpose,
  ) async {
    final purposeStr = purpose == AddressPurpose.shipping
        ? 'shipping'
        : 'sender';
    final result = await _datasource.getAddresses(purpose: purposeStr);

    return result.fold((error) => Result.error(error), (response) {
      final addresses = response.data.map(AddressApiMapper.toDomain).toList();
      return Result.success(addresses);
    });
  }

  @override
  Future<Result<AddressEntity>> getAddressById(String addressId) async {
    final result = await _datasource.getAddressById(addressId);

    return result.fold((error) => Result.error(error), (response) {
      final address = AddressApiMapper.toDomain(response);
      return Result.success(address);
    });
  }

  @override
  Future<Result<AddressEntity?>> getPrimaryAddress(
    String userId, {
    AddressPurpose? purpose,
  }) async {
    final purposeStr = purpose != null
        ? (purpose == AddressPurpose.shipping ? 'shipping' : 'sender')
        : null;
    final result = await _datasource.getPrimaryAddress(purpose: purposeStr);

    return result.fold(
      (error) {
        // 404 means no primary address found - return null instead of error
        if (error.contains('404') || error.contains('not found')) {
          return Result.success(null);
        }
        return Result.error(error);
      },
      (response) {
        final address = AddressApiMapper.toDomain(response);
        return Result.success(address);
      },
    );
  }

  @override
  Future<Result<void>> addAddress(AddressEntity address) async {
    final request = AddressApiMapper.toCreateRequest(address);
    final result = await _datasource.createAddress(request);

    return result.fold(
      (error) => Result.error(error),
      (_) => Result.success(null),
    );
  }

  @override
  Future<Result<void>> updateAddress(AddressEntity address) async {
    // Build update request from entity
    final updates = <String, dynamic>{
      'nickname': address.nickname,
      'recipientName': address.recipientName,
      'phone': address.phone,
      'provinceId': address.province.id,
      'provinceName': address.province.name,
      'cityId': address.city.id,
      'cityName': address.city.name,
      'districtId': address.district.id,
      'districtName': address.district.name,
      'villageId': address.village.id,
      'villageName': address.village.name,
      'streetAddress': address.streetAddress,
      'postalCode': address.postalCode,
      'notes': address.notes,
      'latitude': address.latitude,
      'longitude': address.longitude,
    };

    final request = AddressApiMapper.toUpdateRequest(updates);
    final result = await _datasource.updateAddress(address.id, request);

    return result.fold(
      (error) => Result.error(error),
      (_) => Result.success(null),
    );
  }

  @override
  Future<Result<void>> deleteAddress(String addressId) async {
    final result = await _datasource.deleteAddress(addressId);

    return result.fold(
      (error) => Result.error(error),
      (_) => Result.success(null),
    );
  }

  @override
  Future<Result<void>> setPrimaryAddress(
    String addressId,
    String userId,
  ) async {
    final result = await _datasource.setPrimaryAddress(addressId);

    return result.fold(
      (error) => Result.error(error),
      (_) => Result.success(null),
    );
  }

  @override
  Stream<Result<List<AddressEntity>>> watchAddresses(String userId) {
    // For API implementation, we use polling
    return Stream.periodic(const Duration(seconds: 30)).asyncMap((_) async {
      final result = await _datasource.getAddresses();

      return result.fold((error) => Result.error(error), (response) {
        final addresses = response.data.map(AddressApiMapper.toDomain).toList();
        return Result.success(addresses);
      });
    });
  }

  @override
  Stream<Result<List<AddressEntity>>> watchAddressesByPurpose(
    String userId,
    AddressPurpose purpose,
  ) {
    // For API implementation, we use polling
    return Stream.periodic(const Duration(seconds: 30)).asyncMap((_) async {
      final purposeStr = purpose == AddressPurpose.shipping
          ? 'shipping'
          : 'sender';
      final result = await _datasource.getAddresses(purpose: purposeStr);

      return result.fold((error) => Result.error(error), (response) {
        final addresses = response.data.map(AddressApiMapper.toDomain).toList();
        return Result.success(addresses);
      });
    });
  }

  @override
  Future<Result<int>> countAddresses(
    String userId, {
    AddressPurpose? purpose,
  }) async {
    final result = await _datasource.getAddressCount();

    return result.fold((error) => Result.error(error), (response) {
      // Return count based on purpose
      if (purpose == null) {
        return Result.success(response.total);
      } else if (purpose == AddressPurpose.shipping) {
        return Result.success(response.shippingCount);
      } else {
        return Result.success(response.senderCount);
      }
    });
  }
}
