import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/domains/user/profile/data/datasources/address_api_datasource.dart';
import 'package:labuda/domains/user/profile/data/models/api/address_api_models.dart';
import 'package:labuda/domains/user/profile/data/repositories/address_repository_api.dart';
import 'package:labuda/domains/user/profile/domain/entities/address_entity.dart';

class _FakeAddressDatasource extends AddressApiDatasource {
  _FakeAddressDatasource({
    required this.addressesResult,
    required this.primaryResult,
  }) : super(ApiClient(logger: null));

  final Result<AddressListResponseApi> addressesResult;
  final Result<AddressResponseApi> primaryResult;

  @override
  Future<Result<AddressListResponseApi>> getAddresses({String? purpose}) async {
    return addressesResult;
  }

  @override
  Future<Result<AddressResponseApi>> getPrimaryAddress({
    String? purpose,
  }) async {
    return primaryResult;
  }
}

void main() {
  test(
    'watchAddresses polls and maps addresses correctly',
    () async {
      final repository = AddressRepositoryApi(
        _FakeAddressDatasource(
          addressesResult: Result.success(
            const AddressListResponseApi(data: [], total: 0),
          ),
          primaryResult: Result.error('not used'),
        ),
      );

      // watchAddresses uses Stream.periodic(30s) — first emission
      // arrives after the period, not immediately.
      final first = await repository
          .watchAddresses('user-1')
          .first
          .timeout(const Duration(seconds: 35));

      expect(first.isSuccess, isTrue);
      expect(first.data, isEmpty);
    },
    timeout: const Timeout(Duration(seconds: 40)),
  );

  test(
    'getPrimaryAddress returns typed null for address-not-configured',
    () async {
      final repository = AddressRepositoryApi(
        _FakeAddressDatasource(
          addressesResult: Result.success(
            const AddressListResponseApi(data: [], total: 0),
          ),
          primaryResult: Result.error(
            '404 Not Found',
            code: 'ADDRESS_NOT_CONFIGURED',
            statusCode: 404,
          ),
        ),
      );

      final result = await repository.getPrimaryAddress(
        'user-1',
        purpose: AddressPurpose.sender,
      );

      expect(result.isSuccess, isTrue);
      expect(result.data, isNull);
    },
  );
}
