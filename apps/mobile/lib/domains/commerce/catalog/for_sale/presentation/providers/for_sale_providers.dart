/// ForSale Providers
///
/// Riverpod providers for forSale module.
/// Uses pure Riverpod (no get_it/ServiceLocator).
library;

import 'package:equatable/equatable.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/providers/core_providers.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/data/data.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/for_sale_controller.dart';

// =============================================================================
// DATA LAYER PROVIDERS (Dependency Injection)
// =============================================================================

/// Provider for ILoggerService (uses core provider)
final forSaleLoggerServiceProvider = Provider<ILoggerService>((ref) {
  return ref.watch(loggerServiceProvider);
});

/// Provider for ForSaleRemoteDatasource
/// Direct backend API integration - NO collection dependency
final forSaleDatasourceProvider = Provider<ForSaleRemoteDatasource>((ref) {
  final apiClient = ref.watch(apiClientProvider);
  final logger = ref.watch(forSaleLoggerServiceProvider);
  return ForSaleRemoteDatasource(apiClient, logger: logger);
});

/// Provider for ForSaleRepository implementation
/// Uses ForSaleRemoteDatasource - direct backend API integration
final forSaleRepositoryProvider = Provider<ForSaleRepository>((ref) {
  final datasource = ref.watch(forSaleDatasourceProvider);
  final logger = ref.watch(forSaleLoggerServiceProvider);
  return ForSaleRepositoryImpl(datasource: datasource, logger: logger);
});

// =============================================================================
// APPLICATION LAYER PROVIDERS
// =============================================================================

/// Provider for ForSaleController
final forSaleControllerProvider = Provider<ForSaleController>((ref) {
  final repository = ref.watch(forSaleRepositoryProvider);
  final logger = ref.watch(forSaleLoggerServiceProvider);
  return ForSaleController(repository: repository, logger: logger);
});

// =============================================================================
// READ-ONLY PROVIDERS (UI Consumption)
// =============================================================================

/// ForSales list provider with filter params
class ForSalesParams extends Equatable {
  final int page;
  final int limit;
  final ForSaleStatus? status;
  final String? searchQuery;
  final String? sellerId;
  final double? minPrice;
  final double? maxPrice;

  const ForSalesParams({
    this.page = 1,
    this.limit = 20,
    this.status,
    this.searchQuery,
    this.sellerId,
    this.minPrice,
    this.maxPrice,
  });

  GetForSalesParams toDomainParams() {
    return GetForSalesParams(
      page: page,
      limit: limit,
      status: status,
      searchQuery: searchQuery,
      sellerId: sellerId,
      minPrice: minPrice,
      maxPrice: maxPrice,
    );
  }

  ForSalesParams copyWith({
    int? page,
    int? limit,
    ForSaleStatus? status,
    String? searchQuery,
    String? sellerId,
    double? minPrice,
    double? maxPrice,
  }) {
    return ForSalesParams(
      page: page ?? this.page,
      limit: limit ?? this.limit,
      status: status ?? this.status,
      searchQuery: searchQuery ?? this.searchQuery,
      sellerId: sellerId ?? this.sellerId,
      minPrice: minPrice ?? this.minPrice,
      maxPrice: maxPrice ?? this.maxPrice,
    );
  }

  @override
  List<Object?> get props => [
    page,
    limit,
    status,
    searchQuery,
    sellerId,
    minPrice,
    maxPrice,
  ];
}

/// ForSales future provider
final forSalesProvider = FutureProvider.autoDispose
    .family<List<ForSale>, ForSalesParams>((ref, params) async {
      final controller = ref.watch(forSaleControllerProvider);
      final result = await controller.getForSales(params.toDomainParams());

      return result.fold(
        (error) => throw Exception(error),
        (forSales) => forSales,
      );
    });

/// ForSale detail provider
final forSaleDetailProvider = FutureProvider.autoDispose
    .family<ForSale?, String>((ref, forSaleId) async {
      ref.keepAlive(); // Keep alive to avoid refetching
      final controller = ref.watch(forSaleControllerProvider);
      final result = await controller.getForSaleById(forSaleId);

      return result.fold(
        (error) => throw Exception(error),
        (forSale) => forSale,
      );
    });

/// Seller forSales provider
final sellerForSalesProvider = FutureProvider.autoDispose
    .family<List<ForSale>, SellerForSalesParams>((ref, params) async {
      final controller = ref.watch(forSaleControllerProvider);
      final result = await controller.getSellerForSales(
        params.sellerId,
        page: params.page,
        pageSize: params.pageSize,
      );

      return result.fold(
        (error) => throw Exception(error),
        (forSales) => forSales,
      );
    });

class SellerForSalesParams extends Equatable {
  final String sellerId;
  final int page;
  final int pageSize;

  const SellerForSalesParams({
    required this.sellerId,
    this.page = 1,
    this.pageSize = 20,
  });

  @override
  List<Object?> get props => [sellerId, page, pageSize];
}
