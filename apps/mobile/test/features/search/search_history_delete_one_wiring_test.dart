import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/features/search/search/domain/entities/search_history.dart';
import 'package:labuda/features/search/search/domain/repositories/search_history_repository.dart';
import 'package:labuda/features/search/search/presentation/providers/providers.dart';
import 'package:labuda/features/search/search/presentation/screens/search_screen.dart';
import 'package:labuda/features/search/search/presentation/widgets/search_history_list.dart';
import 'package:labuda/shared/providers/auth_status_providers.dart';

class _FakeSearchHistoryRepository implements SearchHistoryRepository {
  _FakeSearchHistoryRepository(this._history);

  final List<SearchHistory> _history;
  final List<({String userId, String historyId})> deleteCalls = [];

  @override
  Future<ApiResult<void>> clearSearchHistory(String userId) async {
    return (data: null, error: null);
  }

  @override
  Future<ApiResult<void>> deleteSearchHistoryItem(
    String userId,
    String historyId,
  ) async {
    deleteCalls.add((userId: userId, historyId: historyId));
    _history.removeWhere(
      (item) => item.userId == userId && item.id == historyId,
    );
    return (data: null, error: null);
  }

  @override
  Future<ApiResult<List<SearchHistory>>> getSearchHistory(
    String userId, {
    int limit = 10,
  }) async {
    return (data: List<SearchHistory>.from(_history), error: null);
  }

  @override
  Future<ApiResult<void>> saveSearchHistory(SearchHistory history) async {
    _history.add(history);
    return (data: null, error: null);
  }
}

SearchHistory _searchHistoryFixture({
  required String id,
  required String userId,
  required String query,
}) {
  return SearchHistory(
    id: id,
    userId: userId,
    query: query,
    searchedAt: DateTime.parse('2026-08-12T00:00:00.000Z'),
  );
}

void main() {
  testWidgets(
    'tapping delete on a search history row deletes that row by id',
    (tester) async {
      const userId = 'user-1';
      const historyId = 'history-1';
      const query = 'Kohaku';

      final repository = _FakeSearchHistoryRepository([
        _searchHistoryFixture(id: historyId, userId: userId, query: query),
      ]);

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            currentUserIdProvider.overrideWith((ref) => userId),
            searchHistoryRepositoryProvider.overrideWithValue(repository),
          ],
          child: const MaterialApp(
            home: SearchScreen(),
          ),
        ),
      );

      await tester.pumpAndSettle();

      final historyListFinder = find.byType(SearchHistoryList);
      final historyTitleFinder = find.descendant(
        of: historyListFinder,
        matching: find.text(query),
      );
      final deleteButtonFinder = find.descendant(
        of: historyListFinder,
        matching: find.byIcon(Icons.close),
      );

      expect(historyTitleFinder, findsOneWidget);

      await tester.tap(deleteButtonFinder);
      await tester.pumpAndSettle();

      expect(
        repository.deleteCalls,
        [(
          userId: userId,
          historyId: historyId,
        )],
      );
      expect(historyTitleFinder, findsNothing);
    },
  );
}
