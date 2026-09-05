/// ForSale Repository Implementation
///
/// Uses ForSaleRemoteDatasource for direct backend API integration.
/// No collection dependency.
library;

import 'package:labuda/core/common/result.dart';
import 'package:labuda/core/src/interfaces/services/i_logger_service.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/data/dto/for_sale_dto.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/data/mappers/for_sale_dto_mapper.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/data/remote/for_sale_remote_datasource.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/domain.dart';

/// ForSale repository implementation
///
/// Direct integration with backend fixed-price forSale API.
class ForSaleRepositoryImpl implements ForSaleRepository {
  final ForSaleRemoteDatasource _datasource;
  final ILoggerService _logger;

  ForSaleRepositoryImpl({
    required ForSaleRemoteDatasource datasource,
    required ILoggerService logger,
  }) : _datasource = datasource,
       _logger = logger;

  @override
  Future<Result<List<ForSale>>> getForSales(GetForSalesParams params) async {
    try {
      _logger.info(
        'Getting forSales from backend API',
        extra: {
          'page': params.page,
          'limit': params.limit,
          'search': params.searchQuery,
        },
      );

      final result = await _datasource.listForSales(
        page: params.page,
        limit: params.limit,
        search: params.searchQuery,
        sellerId: params.sellerId,
        priceMin: params.minPrice?.toInt(),
        priceMax: params.maxPrice?.toInt(),
        sortBy: _buildSortParam(params.status),
      );

      return result.fold((error) => Result.error(error), (response) {
        final forSales = ForSaleDtoMapper.toEntityList(response.forSales);
        return Result.success(forSales);
      });
    } catch (e, stackTrace) {
      _logger.error(
        'Failed to get forSales',
        extra: {'error': e.toString(), 'stackTrace': stackTrace.toString()},
      );
      return Result.error('Failed to get forSales: ${e.toString()}');
    }
  }

  @override
  Future<Result<ForSale?>> getForSaleById(String forSaleId) async {
    try {
      _logger.info(
        'Getting fixed-price sale by ID from backend API',
        extra: {'id': forSaleId},
      );

      final result = await _datasource.getForSale(forSaleId);

      return result.fold(
        (error) {
          if (error.toString().contains('not found')) {
            return Result.success(null);
          }
          return Result.error(error);
        },
        (dto) {
          final forSale = ForSaleDtoMapper.toEntity(dto);
          return Result.success(forSale);
        },
      );
    } catch (e, stackTrace) {
      _logger.error(
        'Failed to get fixed-price sale',
        extra: {
          'id': forSaleId,
          'error': e.toString(),
          'stackTrace': stackTrace.toString(),
        },
      );
      return Result.error('Failed to get forSale: ${e.toString()}');
    }
  }

  @override
  Future<Result<List<ForSale>>> getForSalesByIds(
    List<String> forSaleIds,
  ) async {
    try {
      if (forSaleIds.isEmpty) {
        return Result.success([]);
      }

      _logger.info(
        'Getting forSales by IDs from backend API',
        extra: {'count': forSaleIds.length},
      );

      final result = await _datasource.getForSalesByIds(forSaleIds);

      return result.fold((error) => Result.error(error), (dtos) {
        final forSales = ForSaleDtoMapper.toEntityList(dtos);
        return Result.success(forSales);
      });
    } catch (e, stackTrace) {
      _logger.error(
        'Failed to get forSales by IDs',
        extra: {
          'count': forSaleIds.length,
          'error': e.toString(),
          'stackTrace': stackTrace.toString(),
        },
      );
      return Result.error('Failed to get forSales by IDs: ${e.toString()}');
    }
  }

  @override
  Future<Result<List<ForSale>>> getSellerForSales(
    String sellerId, {
    int page = 1,
    int pageSize = 20,
  }) async {
    try {
      _logger.info(
        'Getting seller forSales from backend API',
        extra: {'sellerId': sellerId, 'page': page},
      );

      final result = await _datasource.getSellerForSales(
        sellerId,
        page: page,
        limit: pageSize,
      );

      return result.fold((error) => Result.error(error), (response) {
        final forSales = ForSaleDtoMapper.toEntityList(response.forSales);
        return Result.success(forSales);
      });
    } catch (e, stackTrace) {
      _logger.error(
        'Failed to get seller forSales',
        extra: {
          'sellerId': sellerId,
          'error': e.toString(),
          'stackTrace': stackTrace.toString(),
        },
      );
      return Result.error('Failed to get seller forSales: ${e.toString()}');
    }
  }

  String? _buildSortParam(ForSaleStatus? status) {
    // Default sort by newest
    if (status == null) {
      return 'created_at_desc';
    }
    return null;
  }

  @override
  Future<Result<ForSale>> createForSale(CreateForSaleRequest request) async {
    try {
      _logger.info(
        'Creating forSale via backend API',
        extra: {'title': request.title},
      );

      // Convert domain request to DTO
      final dto = CreateForSaleRequestDto(
        title: request.title,
        description: request.description,
        price: request.price.toInt(),
        quantity: request.quantity,
        negotiationEnabled: request.negotiationEnabled,
        visibility: request.visibility,
        mediaUrls: request.mediaUrls,
        variety: request.variety,
        sizeCm: request.sizeCm?.toInt(),
        ageMonths: request.ageMonths,
        gender: request.gender,
        breeder: request.breeder,
        bloodline: request.bloodline,
        certificates: request.certificates,
        farmAddressId: request.farmAddressId,
        preparationTime: request.preparationTime?.toJson(),
        preparationNote: request.preparationNote,
      );

      final result = await _datasource.createForSale(dto);

      if (result.isError) {
        // Preserve the API code (e.g. EMAIL_VERIFICATION_REQUIRED) so the
        // call site can react via Result.errorCode.
        return Result.error(
          result.error ?? 'Unknown error',
          code: result.errorCode,
        );
      }
      final forSale = ForSaleDtoMapper.toEntity(result.data!);
      _logger.info(
        'Successfully created forSale',
        extra: {'id': forSale.forSaleId},
      );
      return Result.success(forSale);
    } catch (e, stackTrace) {
      _logger.error(
        'Failed to create forSale',
        extra: {'error': e.toString(), 'stackTrace': stackTrace.toString()},
      );
      return Result.error('Failed to create forSale: ${e.toString()}');
    }
  }

  @override
  Future<Result<ForSale>> updateForSale(
    String forSaleId,
    UpdateForSaleRequest request,
  ) async {
    try {
      _logger.info(
        'Updating fixed-price sale via backend API',
        extra: {'id': forSaleId},
      );

      // Convert domain request to DTO (F01C: no quantity — stock not editable via PUT)
      final dto = UpdateForSaleRequestDto(
        title: request.title,
        description: request.description,
        price: request.price?.toInt(),
        negotiationEnabled: request.negotiationEnabled,
        status: request.status?.name, // Convert enum to string for backend
        mediaUrls: request.mediaUrls,
        variety: request.variety,
        sizeCm: request.sizeCm?.toInt(),
        ageMonths: request.ageMonths,
        gender: request.gender,
        breeder: request.breeder,
        bloodline: request.bloodline,
        certificates: request.certificates,
        preparationTime: request.preparationTime?.toJson(),
        preparationNote: request.preparationNote,
      );

      final result = await _datasource.updateForSale(forSaleId, dto);

      return result.fold((error) => Result.error(error), (responseDto) {
        final forSale = ForSaleDtoMapper.toEntity(responseDto);
        _logger.info(
          'Successfully updated fixed-price sale',
          extra: {'id': forSale.forSaleId},
        );
        return Result.success(forSale);
      });
    } catch (e, stackTrace) {
      _logger.error(
        'Failed to update fixed-price sale',
        extra: {
          'id': forSaleId,
          'error': e.toString(),
          'stackTrace': stackTrace.toString(),
        },
      );
      return Result.error('Failed to update forSale: ${e.toString()}');
    }
  }

  @override
  Future<Result<void>> deleteForSale(String forSaleId) async {
    try {
      _logger.info(
        'Deleting fixed-price sale via backend API',
        extra: {'id': forSaleId},
      );

      final result = await _datasource.deleteForSale(forSaleId);

      return result.fold(
        (error) {
          _logger.error(
            'Failed to delete fixed-price sale',
            extra: {'error': error},
          );
          return Result.error(error);
        },
        (_) {
          _logger.info(
            'Successfully deleted fixed-price sale',
            extra: {'id': forSaleId},
          );
          return Result.success(null);
        },
      );
    } catch (e, stackTrace) {
      _logger.error(
        'Failed to delete fixed-price sale',
        extra: {
          'id': forSaleId,
          'error': e.toString(),
          'stackTrace': stackTrace.toString(),
        },
      );
      return Result.error('Failed to delete forSale: ${e.toString()}');
    }
  }

  @override
  Future<Result<ForSale>> updateForSaleStatus(
    String forSaleId,
    ForSaleStatus status,
  ) async {
    try {
      _logger.info(
        'Updating fixed-price sale status via backend API',
        extra: {'id': forSaleId, 'status': status.name},
      );

      // Convert status to string for backend
      final statusStr = switch (status) {
        ForSaleStatus.draft => 'draft',
        ForSaleStatus.active => 'active',
        ForSaleStatus.withdrawn => 'withdrawn',
        ForSaleStatus.sold => 'sold',
      };

      final dto = UpdateForSaleRequestDto(
        status: statusStr,
      );

      final result = await _datasource.updateForSale(forSaleId, dto);

      if (result.isError) {
        // Preserve the machine-readable code (e.g. SHIPPING_NOT_CONFIGURED)
        // so the publish call site can branch via Result.errorCode instead of
        // matching error message substrings.
        return Result.error(
          result.error ?? 'Failed to update forSale status',
          code: result.errorCode,
          statusCode: result.statusCode,
        );
      }
      final forSale = ForSaleDtoMapper.toEntity(result.data!);
      _logger.info(
        'Successfully updated fixed-price sale status',
        extra: {'id': forSale.forSaleId, 'status': status.name},
      );
      return Result.success(forSale);
    } catch (e, stackTrace) {
      _logger.error(
        'Failed to update fixed-price sale status',
        extra: {
          'id': forSaleId,
          'error': e.toString(),
          'stackTrace': stackTrace.toString(),
        },
      );
      return Result.error('Failed to update forSale status: ${e.toString()}');
    }
  }
}
