/// Order Data Providers - Riverpod providers for order data layer
///
/// This file provides all data dependencies for the order feature using pure Riverpod.
/// Replaces the GetIt-based OrderApiDI dependency injection.
///
/// MIGRATION STATUS: Migrated from order_api_di.dart (GetIt) to Riverpod
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/transaction/order/data/order_repository_impl.dart';
import 'package:labuda/domains/commerce/transaction/order/data/refund_repository_impl.dart';
import 'package:labuda/domains/commerce/transaction/order/data/remote/order_api_datasource_impl.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/repositories/order_repository.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/repositories/refund_repository.dart';

// =============================================================================
// DATASOURCE PROVIDERS
// =============================================================================

/// Order API Datasource Provider
final orderApiDatasourceProvider = Provider<OrderApiDatasourceImpl>((ref) {
  final apiClient = ref.watch(apiClientProvider);
  final logger = ref.watch(loggerServiceProvider);
  return OrderApiDatasourceImpl(apiClient, logger: logger);
});

// =============================================================================
// REPOSITORY PROVIDERS
// =============================================================================

/// Order Repository Provider
///
/// Provides the API implementation of OrderRepository.
/// This replaces the GetIt-based OrderApiDI.orderRepository.
///
/// MIGRATION: Previously accessed via `OrderApiDI.orderRepository` or `sl<OrderRepository>()`
final orderRepositoryProvider = Provider<OrderRepository>((ref) {
  final datasource = ref.watch(orderApiDatasourceProvider);
  final logger = ref.watch(loggerServiceProvider);
  return OrderRepositoryImpl(datasource, logger: logger);
});

/// Refund Repository Provider
///
/// Provides the API implementation of RefundRepository.
/// This replaces the GetIt-based OrderApiDI.refundRepository.
///
/// MIGRATION: Previously accessed via `OrderApiDI.refundRepository` or `sl<RefundRepository>()`
final refundRepositoryProvider = Provider<RefundRepository>((ref) {
  final datasource = ref.watch(orderApiDatasourceProvider);
  return RefundRepositoryImpl(remoteDatasource: datasource);
});
