import 'package:labuda/core/api/api.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/domains/user/profile/data/models/api/address_api_models.dart';

/// Address API datasource for HTTP operations
class AddressApiDatasource extends BaseApiRepository {
  AddressApiDatasource(super.apiClient, {super.logger});

  /// Create a new address
  Future<Result<AddressResponseApi>> createAddress(
    CreateAddressRequestApi request,
  ) async {
    return executeRequest(
      () => apiClient.post('/addresses', data: request.toJson()),
      parser: (data) =>
          AddressResponseApi.fromJson(data as Map<String, dynamic>),
    );
  }

  /// Get all addresses for current user
  /// Optional [purpose] filter: 'shipping' or 'sender'
  Future<Result<AddressListResponseApi>> getAddresses({String? purpose}) async {
    final queryParams = <String, dynamic>{};
    if (purpose != null) {
      queryParams['purpose'] = purpose;
    }

    return executeRequest(
      () => apiClient.get('/addresses', queryParameters: queryParams),
      parser: (data) =>
          AddressListResponseApi.fromJson(data as Map<String, dynamic>),
    );
  }

  /// Get address by ID
  Future<Result<AddressResponseApi>> getAddressById(String addressId) async {
    return executeRequest(
      () => apiClient.get('/addresses/$addressId'),
      parser: (data) =>
          AddressResponseApi.fromJson(data as Map<String, dynamic>),
    );
  }

  /// Get primary address
  /// Optional [purpose] filter: 'shipping' or 'sender'
  Future<Result<AddressResponseApi>> getPrimaryAddress({
    String? purpose,
  }) async {
    final queryParams = <String, dynamic>{};
    if (purpose != null) {
      queryParams['purpose'] = purpose;
    }

    return executeRequest(
      () => apiClient.get('/addresses/primary', queryParameters: queryParams),
      parser: (data) =>
          AddressResponseApi.fromJson(data as Map<String, dynamic>),
    );
  }

  /// Update address
  Future<Result<AddressResponseApi>> updateAddress(
    String addressId,
    UpdateAddressRequestApi request,
  ) async {
    return executeRequest(
      () => apiClient.put('/addresses/$addressId', data: request.toJson()),
      parser: (data) =>
          AddressResponseApi.fromJson(data as Map<String, dynamic>),
    );
  }

  /// Delete address
  Future<Result<void>> deleteAddress(String addressId) async {
    return executeRequest(
      () => apiClient.delete('/addresses/$addressId'),
      parser: (_) {},
    );
  }

  /// Set address as primary
  Future<Result<void>> setPrimaryAddress(String addressId) async {
    return executeRequest(
      () => apiClient.post('/addresses/$addressId/primary'),
      parser: (_) {},
    );
  }

  /// Get address count
  Future<Result<AddressCountResponseApi>> getAddressCount() async {
    return executeRequest(
      () => apiClient.get('/addresses/count'),
      parser: (data) =>
          AddressCountResponseApi.fromJson(data as Map<String, dynamic>),
    );
  }
}
