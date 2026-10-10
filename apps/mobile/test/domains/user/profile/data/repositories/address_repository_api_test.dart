import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/api/api_client.dart';
import 'package:hishumi/core/common/result.dart';
import 'package:hishumi/domains/user/profile/data/datasources/address_api_datasource.dart';
import 'package:hishumi/domains/user/profile/data/models/api/address_api_models.dart';
import 'package:hishumi/domains/user/profile/data/repositories/address_repository_api.dart';

class _FakeAddressDatasource extends AddressApiDatasource {
  _FakeAddressDatasource({
    required this.addressesResult,
    required this.primaryResult,
  }) : super(ApiClient(logger: null));

  final Result<AddressListResponseApi> addressesResult;
  final Result<AddressResponseApi> primaryResult;

  @override
  Future<Result<AddressListResponseApi>> getAddresses() async {
    return addressesResult;
  }

  @override
  Future<Result<AddressResponseApi>> getPrimaryAddress() async {
    return primaryResult;
  }
}

void main() {
  test('getAddressesByUserId resolves immediately (no polling)', () async {
    final repository = AddressRepositoryApi(
      _FakeAddressDatasource(
        addressesResult: Result.success(
          const AddressListResponseApi(data: [], total: 0),
        ),
        primaryResult: Result.error('not used'),
      ),
    );

    // One-shot read — the Settings page must not wait for a polling interval.
    final result = await repository.getAddressesByUserId('user-1');

    expect(result.isSuccess, isTrue);
    expect(result.data, isEmpty);
  });

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

      final result = await repository.getPrimaryAddress('user-1');

      expect(result.isSuccess, isTrue);
      expect(result.data, isNull);
    },
  );
}
