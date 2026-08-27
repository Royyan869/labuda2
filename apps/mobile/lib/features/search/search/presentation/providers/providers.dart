import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/features/search/search/data/search_history_repository_impl.dart';
import 'package:labuda/features/search/search/data/search_repository_impl.dart';
import 'package:labuda/features/search/search/data/remote/search_api_service.dart';
import 'package:labuda/features/search/search/domain/repositories/search_history_repository.dart';
import 'package:labuda/features/search/search/domain/repositories/search_repository.dart';
part 'providers.g.dart';

// =====================
// Data Layer Providers
// =====================

/// Provides SearchApiService
@riverpod
SearchApiService searchApiService(Ref ref) {
  final apiClient = ref.watch(apiClientProvider);
  final logger = ref.watch(loggerServiceProvider);

  return SearchApiService(apiClient, logger: logger);
}

/// Provides SearchRepository
@riverpod
SearchRepository searchRepository(Ref ref) {
  final apiService = ref.watch(searchApiServiceProvider);

  return SearchRepositoryImpl(apiService);
}

/// Provides SearchHistoryRepository
@riverpod
SearchHistoryRepository searchHistoryRepository(Ref ref) {
  final apiService = ref.watch(searchApiServiceProvider);

  return SearchHistoryRepositoryImpl(apiService);
}
