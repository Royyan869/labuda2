import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'providers.dart';
import 'search_history_state.dart';
import 'package:labuda/features/search/search/domain/entities/search_history.dart';
import 'package:labuda/features/search/search/domain/repositories/search_history_repository.dart';

part 'search_history_notifier.g.dart';

/// Search History Notifier - handles search history logic without UseCase classes
@riverpod
class SearchHistoryNotifier extends _$SearchHistoryNotifier {
  @override
  SearchHistoryState build() {
    // Get repository from provider
    final repository = ref.watch(searchHistoryRepositoryProvider);
    _repository = repository;
    return const SearchHistoryState();
  }

  SearchHistoryRepository? _repository;

  /// Load search history for user
  Future<void> loadHistory(String userId) async {
    if (_repository == null) {
      state = state.copyWith(error: 'Repository not initialized');
      return;
    }

    state = state.copyWith(isLoading: true);

    final result = await _repository!.getSearchHistory(userId, limit: 10);

    if (result.error != null) {
      state = state.copyWith(isLoading: false, error: result.error);
    } else {
      state = state.copyWith(
        history: result.data!,
        isLoading: false,
        error: null,
      );
    }
  }

  /// Save search to history
  Future<void> saveSearch(String userId, String query, int resultCount) async {
    if (_repository == null) {
      return;
    }

    final history = SearchHistory(
      id: DateTime.now().millisecondsSinceEpoch.toString(),
      userId: userId,
      query: query,
      searchedAt: DateTime.now(),
      resultCount: resultCount,
    );

    await _repository!.saveSearchHistory(history);
    // Reload history after saving
    await loadHistory(userId);
  }

  /// Clear all history
  Future<void> clearHistory(String userId) async {
    if (_repository == null) {
      return;
    }

    final result = await _repository!.clearSearchHistory(userId);

    if (result.error == null) {
      state = state.copyWith(history: []);
    } else {
      state = state.copyWith(error: result.error);
    }
  }

  /// Delete specific history item
  Future<void> deleteItem(String userId, String historyId) async {
    if (_repository == null) {
      return;
    }

    final result = await _repository!.deleteSearchHistoryItem(
      userId,
      historyId,
    );

    if (result.error == null) {
      // Remove from local state
      final updatedHistory = state.history
          .where((h) => h.id != historyId)
          .toList();
      state = state.copyWith(history: updatedHistory);
    }
  }
}
