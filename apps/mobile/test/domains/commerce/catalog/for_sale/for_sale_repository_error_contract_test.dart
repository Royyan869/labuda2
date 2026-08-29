import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/core/src/interfaces/services/i_logger_service.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/data/dto/for_sale_dto.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/data/repositories/for_sale_repository_impl.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/data/remote/for_sale_remote_datasource.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/domain.dart';

class _NoOpLogger implements ILoggerService {
  @override
  Future<Result<void>> info(String message, {Map<String, dynamic>? extra}) {
    return Future.value(Result.success(null));
  }

  @override
  Future<Result<void>> error(
    String message, {
    Map<String, dynamic>? extra,
    StackTrace? stackTrace,
  }) {
    return Future.value(Result.success(null));
  }

  @override
  Future<void> log(String message, {LogLevel level = LogLevel.debug}) async {}

  @override
  dynamic noSuchMethod(Invocation invocation) => null;
}

class _ErrorListingDatasource extends ForSaleRemoteDatasource {
  _ErrorListingDatasource() : super(ApiClient());

  @override
  Future<Result<ForSaleResponseDto>> createForSale(
    CreateForSaleRequestDto request,
  ) async {
    return Result.error(
      'Pilih minimal 1 opsi pengiriman',
      code: 'SHIPPING_OPTION_REQUIRED',
      statusCode: 400,
      details: <String, dynamic>{'field': 'shipping_option_ids'},
    );
  }
}

void main() {
  test(
    'createListing preserves backend error code and message',
    () async {
      final repository = ForSaleRepositoryImpl(
        datasource: _ErrorListingDatasource(),
        logger: _NoOpLogger(),
      );

      final result = await repository.createForSale(
        const CreateForSaleRequest(
          title: 'Kohaku',
          description: 'Listing tanpa shipping',
          price: 1500000,
          quantity: 1,
        ),
      );

      expect(result.isError, isTrue);
      expect(result.errorCode, 'SHIPPING_OPTION_REQUIRED');
      expect(result.error, 'Pilih minimal 1 opsi pengiriman');
    },
  );
}
