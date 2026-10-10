import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/features/search/search/domain/entities/search_history.dart';
import 'package:hishumi/features/search/search/domain/repositories/search_history_repository.dart';
import 'package:hishumi/features/search/search/presentation/providers/providers.dart';
import 'package:hishumi/features/search/search/presentation/screens/search_screen.dart';
import 'package:hishumi/shared/providers/auth_status_providers.dart';

class _FakeSearchHistoryRepository implements SearchHistoryRepository {
  _FakeSearchHistoryRepository(this._history);

  final List<SearchHistory> _history;
  final List<SearchHistory> savedItems = [];

  @override
  Future<Result<void>> clearSearchHistory(String userId) async {
    return Result.success(null);
  }

  @override
  Future<Result<void>> deleteSearchHistoryItem(
    String userId,
    String historyId,
  ) async {
    return Result.success(null);
  }

  @override
  Future<Result<List<SearchHistory>>> getSearchHistory(
    String userId, {
    int limit = 10,
  }) async {
    return Result.success(List<SearchHistory>.from(_history));
  }

  @override
  Future<Result<void>> saveSearchHistory(SearchHistory history) async {
    savedItems.add(history);
    _history.add(history);
    return Result.success(null);
  }
}

class _RecordingNavigationHandler extends Fake implements NavigationHandler {
  String? searchedQuery;

  @override
  void navigateToSearchResults(String query, {String? type}) {
    searchedQuery = query;
  }
}

void main() {
  testWidgets('SearchScreen submit saves history exactly once', (tester) async {
    const userId = 'user-1';
    const query = 'Kohaku';

    final repository = _FakeSearchHistoryRepository(<SearchHistory>[]);
    final navigationHandler = _RecordingNavigationHandler();

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          currentUserIdProvider.overrideWith((ref) => userId),
          searchHistoryRepositoryProvider.overrideWithValue(repository),
          navigationHandlerProvider.overrideWithValue(navigationHandler),
        ],
        child: const MaterialApp(home: SearchScreen()),
      ),
    );

    await tester.pumpAndSettle();

    final searchField = find.byType(TextField);
    expect(searchField, findsOneWidget);

    await tester.enterText(searchField, query);
    await tester.testTextInput.receiveAction(TextInputAction.search);
    await tester.pumpAndSettle();

    expect(repository.savedItems, hasLength(1));
    final savedHistory = repository.savedItems.single;
    expect(savedHistory.query, query);
    expect(savedHistory.userId, userId);
    expect(savedHistory.id, isNotEmpty);
    expect(navigationHandler.searchedQuery, query);
  });
}
