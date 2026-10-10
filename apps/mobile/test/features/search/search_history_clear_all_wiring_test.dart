import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/common/result.dart';
import 'package:hishumi/features/search/search/domain/entities/search_history.dart';
import 'package:hishumi/features/search/search/domain/repositories/search_history_repository.dart';
import 'package:hishumi/features/search/search/presentation/providers/providers.dart';
import 'package:hishumi/features/search/search/presentation/screens/search_screen.dart';
import 'package:hishumi/shared/providers/auth_status_providers.dart';

class _FakeSearchHistoryRepository implements SearchHistoryRepository {
  _FakeSearchHistoryRepository(this._history);

  final List<SearchHistory> _history;
  final List<String> clearCalls = [];

  @override
  Future<Result<void>> clearSearchHistory(String userId) async {
    clearCalls.add(userId);
    _history.removeWhere((item) => item.userId == userId);
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
    _history.add(history);
    return Result.success(null);
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
    'tapping clear all clears search history through the canonical repository path',
    (tester) async {
      const userId = 'user-1';
      const query = 'Benigoi Deluxe';

      final repository = _FakeSearchHistoryRepository([
        _searchHistoryFixture(id: 'history-1', userId: userId, query: query),
      ]);

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            currentUserIdProvider.overrideWith((ref) => userId),
            searchHistoryRepositoryProvider.overrideWithValue(repository),
          ],
          child: const MaterialApp(home: SearchScreen()),
        ),
      );

      await tester.pumpAndSettle();

      expect(find.text(query), findsOneWidget);
      expect(find.text('Clear All'), findsOneWidget);

      await tester.tap(find.text('Clear All'));
      await tester.pumpAndSettle();

      expect(repository.clearCalls, [userId]);
      expect(find.text(query), findsNothing);
      expect(find.text('Clear All'), findsNothing);
    },
  );
}
