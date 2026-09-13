/// Auction Data Providers - Riverpod providers for auction data layer
///
/// This file provides all data dependencies for the auction feature using pure Riverpod.
/// Replaces the GetIt-based AuctionDI dependency injection.
///
/// MIGRATION STATUS: Migrated from auction_di.dart (GetIt) to Riverpod
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/remote/auction_remote_datasource.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/remote/bidding_remote_datasource.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/repositories/auction_repository_impl.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/repositories/bidding_repository_impl.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/domain.dart';

// =============================================================================
// DATASOURCE PROVIDERS
// =============================================================================

/// Auction Remote Datasource Provider
final auctionRemoteDatasourceProvider = Provider<AuctionRemoteDatasource>((
  ref,
) {
  final apiClient = ref.watch(apiClientProvider);
  final logger = ref.watch(loggerServiceProvider);
  return AuctionRemoteDatasource(apiClient, logger: logger);
});

/// Bidding Remote Datasource Provider
final biddingRemoteDatasourceProvider = Provider<BiddingRemoteDatasource>((
  ref,
) {
  final apiClient = ref.watch(apiClientProvider);
  final logger = ref.watch(loggerServiceProvider);
  return BiddingRemoteDatasource(apiClient, logger: logger);
});

// =============================================================================
// REPOSITORY PROVIDERS
// =============================================================================

/// Auction Repository Provider
///
/// Provides the API implementation of AuctionRepository.
/// This replaces the GetIt-based AuctionDI.auctionRepository.
///
/// MIGRATION: Previously accessed via AuctionDI.auctionRepository or the generic `sl` accessor
final auctionRepositoryProvider = Provider<AuctionRepository>((ref) {
  final datasource = ref.watch(auctionRemoteDatasourceProvider);
  final logger = ref.watch(loggerServiceProvider);
  return AuctionRepositoryImpl(datasource: datasource, logger: logger);
});

/// Bidding Repository Provider
///
/// Provides the API implementation of BiddingRepository.
final biddingRepositoryProvider = Provider<BiddingRepository>((ref) {
  final datasource = ref.watch(biddingRemoteDatasourceProvider);
  final logger = ref.watch(loggerServiceProvider);
  return BiddingRepositoryImpl(datasource: datasource, logger: logger);
});
