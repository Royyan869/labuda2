/// ForSale Controller
///
/// Application layer - orchestrates business logic.
/// Wraps repository calls with error handling.
library;

import 'package:labuda/core/common/result.dart';
import 'package:labuda/core/src/interfaces/services/i_logger_service.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/domain.dart';
import 'package:labuda/domains/user/identity/authentication/authentication.dart';

/// ForSale Controller
class ForSaleController {
  final ForSaleRepository _repository;
  final ILoggerService _logger;

  ForSaleController({
    required ForSaleRepository repository,
    required ILoggerService logger,
  }) : _repository = repository,
       _logger = logger;

  /// Returns true only when the current authenticated principal can create a
  /// fixed-price forSale.
  bool canCreateForSale(AuthState authState) {
    if (authState is! AuthStateAuthenticated) {
      return false;
    }

    final user = authState.user;
    return user.hasSellerProfile == true && user.hasMarketAuthority == true;
  }

  /// Get list of forSales
  Future<Result<List<ForSale>>> getForSales(GetForSalesParams params) async {
    try {
      _logger.info('Fetching forSales', extra: {'params': params.toString()});
      final result = await _repository.getForSales(params);

      result.fold(
        (error) =>
            _logger.error('Failed to fetch forSales', extra: {'error': error}),
        (forSales) => _logger.info(
          'Successfully fetched forSales',
          extra: {'count': forSales.length},
        ),
      );

      return result;
    } catch (e, stackTrace) {
      _logger.error(
        'Unexpected error in getForSales',
        extra: {'error': e.toString(), 'stackTrace': stackTrace.toString()},
      );
      return Result.error('Unexpected error: ${e.toString()}');
    }
  }

  /// Get fixed-price sale by ID
  Future<Result<ForSale?>> getForSaleById(String forSaleId) async {
    try {
      _logger.info('Fetching fixed-price sale by ID', extra: {'id': forSaleId});
      final result = await _repository.getForSaleById(forSaleId);

      result.fold(
        (error) => _logger.error(
          'Failed to fetch fixed-price sale',
          extra: {'error': error},
        ),
        (forSale) => _logger.info(
          'Successfully fetched fixed-price sale',
          extra: {'found': forSale != null},
        ),
      );

      return result;
    } catch (e, stackTrace) {
      _logger.error(
        'Unexpected error in getForSaleById',
        extra: {'error': e.toString(), 'stackTrace': stackTrace.toString()},
      );
      return Result.error('Unexpected error: ${e.toString()}');
    }
  }

  /// Get seller forSales
  Future<Result<List<ForSale>>> getSellerForSales(
    String sellerId, {
    int page = 1,
    int pageSize = 20,
  }) async {
    try {
      _logger.info(
        'Fetching seller forSales',
        extra: {'sellerId': sellerId, 'page': page},
      );
      final result = await _repository.getSellerForSales(
        sellerId,
        page: page,
        pageSize: pageSize,
      );

      result.fold(
        (error) => _logger.error(
          'Failed to fetch seller forSales',
          extra: {'error': error},
        ),
        (forSales) => _logger.info(
          'Successfully fetched seller forSales',
          extra: {'count': forSales.length},
        ),
      );

      return result;
    } catch (e, stackTrace) {
      _logger.error(
        'Unexpected error in getSellerForSales',
        extra: {'error': e.toString(), 'stackTrace': stackTrace.toString()},
      );
      return Result.error('Unexpected error: ${e.toString()}');
    }
  }

  /// Create a new forSale
  ///
  /// The caller should use [canCreateForSale] to gate the UI, but this method
  /// also rechecks the current auth state before touching the repository.
  Future<Result<ForSale>> createForSaleIfAuthorized(
    CreateForSaleRequest request,
    AuthState authState,
  ) async {
    if (authState is! AuthStateAuthenticated) {
      return Result.error(
        'Sesi autentikasi belum siap untuk membuat forSale.',
        code: 'AUTH_NOT_READY',
      );
    }

    final user = authState.user;
    if (user.hasSellerProfile != true) {
      return Result.error(
        'Buat seller profile dulu untuk membuat forSale.',
        code: 'SELLER_PROFILE_REQUIRED',
      );
    }

    if (user.hasMarketAuthority != true) {
      return Result.error(
        'Langganan seller Anda sudah berakhir. Perpanjang dulu untuk membuat forSale.',
        code: 'MARKET_AUTHORITY_REQUIRED',
      );
    }

    return createForSale(request);
  }

  /// Create a new forSale
  Future<Result<ForSale>> createForSale(CreateForSaleRequest request) async {
    try {
      _logger.info('Creating forSale', extra: {'title': request.title});
      final result = await _repository.createForSale(request);

      result.fold(
        (error) =>
            _logger.error('Failed to create forSale', extra: {'error': error}),
        (forSale) => _logger.info(
          'Successfully created forSale',
          extra: {'id': forSale.forSaleId},
        ),
      );

      return result;
    } catch (e, stackTrace) {
      _logger.error(
        'Unexpected error in createForSale',
        extra: {'error': e.toString(), 'stackTrace': stackTrace.toString()},
      );
      return Result.error('Unexpected error: ${e.toString()}');
    }
  }

  /// Update a fixed-price sale
  Future<Result<ForSale>> updateForSale(
    String forSaleId,
    UpdateForSaleRequest request,
  ) async {
    try {
      _logger.info('Updating fixed-price sale', extra: {'id': forSaleId});
      final result = await _repository.updateForSale(forSaleId, request);

      result.fold(
        (error) => _logger.error(
          'Failed to update fixed-price sale',
          extra: {'error': error},
        ),
        (forSale) => _logger.info(
          'Successfully updated fixed-price sale',
          extra: {'id': forSale.forSaleId},
        ),
      );

      return result;
    } catch (e, stackTrace) {
      _logger.error(
        'Unexpected error in updateForSale',
        extra: {'error': e.toString(), 'stackTrace': stackTrace.toString()},
      );
      return Result.error('Unexpected error: ${e.toString()}');
    }
  }

  /// Delete a fixed-price sale
  Future<Result<void>> deleteForSale(String forSaleId) async {
    try {
      _logger.info('Deleting fixed-price sale', extra: {'id': forSaleId});
      final result = await _repository.deleteForSale(forSaleId);

      result.fold(
        (error) => _logger.error(
          'Failed to delete fixed-price sale',
          extra: {'error': error},
        ),
        (_) => _logger.info(
          'Successfully deleted fixed-price sale',
          extra: {'id': forSaleId},
        ),
      );

      return result;
    } catch (e, stackTrace) {
      _logger.error(
        'Unexpected error in deleteForSale',
        extra: {'error': e.toString(), 'stackTrace': stackTrace.toString()},
      );
      return Result.error('Unexpected error: ${e.toString()}');
    }
  }

  /// Update fixed-price sale status
  Future<Result<ForSale>> updateForSaleStatus(
    String forSaleId,
    ForSaleStatus status,
  ) async {
    try {
      _logger.info(
        'Updating fixed-price sale status',
        extra: {'id': forSaleId, 'status': status.name},
      );
      final result = await _repository.updateForSaleStatus(forSaleId, status);

      result.fold(
        (error) => _logger.error(
          'Failed to update fixed-price sale status',
          extra: {'error': error},
        ),
        (forSale) => _logger.info(
          'Successfully updated fixed-price sale status',
          extra: {'id': forSale.forSaleId, 'status': status.name},
        ),
      );

      return result;
    } catch (e, stackTrace) {
      _logger.error(
        'Unexpected error in updateForSaleStatus',
        extra: {'error': e.toString(), 'stackTrace': stackTrace.toString()},
      );
      return Result.error('Unexpected error: ${e.toString()}');
    }
  }
}
