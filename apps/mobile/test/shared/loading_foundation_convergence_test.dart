// Loading Foundation convergence proof — first-load vs refresh.
//
// Business truth (owner-locked):
// - First load without data → loading as the main page state.
// - Refresh with data → old data stays + update indication (never full-page
//   loading/error).
// - First-load failure → PageErrorState, never EmptyState.
// - Refresh failure → old data stays + inline indication, never PageErrorState.
// - Successful zero-result → EmptyState. Loading/Idle is never Empty.
import 'dart:async';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/common/result.dart';
import 'package:hishumi/core/providers/core_providers.dart';
import 'package:hishumi/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:hishumi/domains/chat/chat/domain/repositories/chat_repository.dart';
import 'package:hishumi/domains/chat/chat/presentation/providers/chat_providers.dart';
import 'package:hishumi/domains/chat/chat/presentation/providers/chat_state.dart';
import 'package:hishumi/domains/chat/chat/presentation/screens/chat_list_screen.dart';
import 'package:hishumi/domains/chat/chat/presentation/widgets/chat_card.dart';
import 'package:hishumi/domains/finance/wallet/coins/coins_di.dart';
import 'package:hishumi/domains/finance/wallet/coins/presentation/screens/coin_history_screen.dart';
import 'package:hishumi/domains/user/identity/authentication/authentication.dart';
import 'package:hishumi/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:hishumi/features/home/domain/entities/feed_item.dart';
import 'package:hishumi/features/home/domain/entities/feed_page.dart';
import 'package:hishumi/features/home/domain/repositories/home_repository.dart';
import 'package:hishumi/features/home/presentation/providers/feed/feed_notifier.dart';
import 'package:hishumi/generated/app_localizations.dart';
import 'package:hishumi/shared/providers/authenticated_account_provider.dart';
import 'package:hishumi/shared/providers/auth_status_providers.dart'
    show currentUserIdProvider;
import 'package:hishumi/shared/services/logger_service.dart';
import 'package:hishumi/shared/widgets/empty_state.dart';
import 'package:hishumi/shared/widgets/loading_indicator.dart';
import 'package:hishumi/shared/widgets/page_error_state.dart';

String _source(String relativePath) {
  return File(relativePath).readAsStringSync().replaceAll('\r\n', '\n');
}

const _feedNotifier =
    'lib/features/home/presentation/providers/feed/feed_notifier.dart';
const _homeScreen = 'lib/features/home/presentation/screens/home_screen.dart';
const _chatNotifier =
    'lib/domains/chat/chat/presentation/providers/chat_notifier.dart';
const _chatListScreen =
    'lib/domains/chat/chat/presentation/screens/chat_list_screen.dart';
const _coinHistoryScreen =
    'lib/domains/finance/wallet/coins/presentation/screens/coin_history_screen.dart';
const _searchResultsScreen =
    'lib/features/search/search/presentation/screens/search_results_screen.dart';
const _marketplaceGrid =
    'lib/domains/commerce/catalog/shared/presentation/widgets/commerce_marketplace_primitives.dart';
const _emptyState = 'lib/shared/widgets/empty_state.dart';

// ============================================================================
// Feed fakes
// ============================================================================

class _FakeAuthController extends AuthController {
  @override
  AuthState build() => const AuthStateUnauthenticated();
}

const _authorId = '00000000-0000-0000-0000-000000000123';

FeedItem _feedItem({required String id, required String content}) {
  return FeedItem(
    id: id,
    content: content,
    authorId: _authorId,
    authorUsername: 'alice',
    authorAvatarUrl: 'https://example.com/avatar.jpg',
    type: FeedItemType.content,
    createdAt: DateTime.utc(2026, 7, 23, 10, 0),
    additionalData: const {'status': 'active'},
  );
}

/// Repository whose initial page can be held open with a [Completer] so the
/// in-flight refresh state is observable.
class _BlockingHomeRepository implements HomeRepository {
  _BlockingHomeRepository({required this.page});

  FeedPage page;
  Completer<void>? gate;
  int refreshCalls = 0;

  @override
  Future<FeedPage> getFeedPage({
    int limit = 20,
    String? currentUserId,
    bool loadMore = false,
  }) async {
    final g = gate;
    if (g != null) await g.future;
    return page;
  }

  @override
  Stream<List<FeedItem>> watchFeedItems({
    int limit = 20,
    String? currentUserId,
  }) {
    return const Stream<List<FeedItem>>.empty();
  }

  @override
  Future<void> refreshFeedItems() async {
    refreshCalls += 1;
  }
}

class _ToggleHomeRepository implements HomeRepository {
  _ToggleHomeRepository({required this.page});

  final FeedPage page;
  Object? nextError;

  @override
  Future<FeedPage> getFeedPage({
    int limit = 20,
    String? currentUserId,
    bool loadMore = false,
  }) async {
    final error = nextError;
    nextError = null;
    if (error != null) throw error;
    return page;
  }

  @override
  Stream<List<FeedItem>> watchFeedItems({
    int limit = 20,
    String? currentUserId,
  }) {
    return const Stream<List<FeedItem>>.empty();
  }

  @override
  Future<void> refreshFeedItems() async {}
}

ProviderContainer _feedContainer(HomeRepository repo) {
  final container = ProviderContainer(
    overrides: [
      homeRepositoryProvider.overrideWithValue(repo),
      loggerServiceProvider.overrideWithValue(LoggerService.instance),
      authControllerProvider.overrideWith(_FakeAuthController.new),
    ],
  );
  addTearDown(container.dispose);
  // Instantiate the notifier (triggers the initial load).
  container.listen(feedProvider, (_, _) {});
  return container;
}

Future<void> _settle() async {
  await Future<void>.delayed(Duration.zero);
  await Future<void>.delayed(Duration.zero);
}

// ============================================================================
// Chat fakes
// ============================================================================

Chat _chat(String id) => Chat(
  id: id,
  participantIds: const ['me', 'other'],
  participantNames: const {'me': 'Me', 'other': 'Other User'},
  participantAvatars: const <String, String?>{},
  createdAt: DateTime.utc(2026, 1, 1),
);

class _ToggleChatRepository implements ChatRepository {
  _ToggleChatRepository(this.chats);

  List<Chat> chats;
  Object? nextError;

  @override
  Future<Result<List<Chat>>> getUserChats({
    required String userId,
    int page = 1,
    int limit = 20,
  }) async {
    final error = nextError;
    nextError = null;
    if (error != null) return Result.error('load failed');
    return Result.success(chats);
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _FakeChatList extends ChatList {
  _FakeChatList(this._initial);

  final ChatListState _initial;

  @override
  ChatListState build() => _initial;

  @override
  Future<void> loadChats(String userId, {bool isRefresh = false}) async {}
}

Widget _chatApp(ChatListState initial) => ProviderScope(
  overrides: [
    currentUserIdProvider.overrideWithValue('me'),
    chatListProvider.overrideWith(() => _FakeChatList(initial)),
  ],
  child: MaterialApp(
    localizationsDelegates: AppLocalizations.localizationsDelegates,
    supportedLocales: AppLocalizations.supportedLocales,
    locale: const Locale('id'),
    home: const ChatListScreen(),
  ),
);

// ============================================================================
// Coin fakes
// ============================================================================

AuthUser _coinUser() => AuthUser(
  id: 'user-1',
  createdAt: DateTime.utc(2026, 8, 1),
  updatedAt: DateTime.utc(2026, 8, 1),
  email: 'user@example.com',
  username: 'user',
  isEmailVerified: true,
  accountStatus: AccountStatus.active,
  roles: const [],
  provider: AuthProvider.email,
);

CoinTransaction _tx(String id) => CoinTransaction(
  id: id,
  userId: 'user-1',
  type: CoinTransactionType.earn,
  sourceType: CoinSourceType.orderReward,
  amount: 10,
  balanceAfter: 100,
  description: 'Order reward',
  createdAt: DateTime.utc(2026, 8, 2),
);

class _ScriptedCoinRepository implements CoinRepository {
  _ScriptedCoinRepository(this._script);

  final List<Result<List<CoinTransaction>>> _script;
  Completer<void>? gate;
  int calls = 0;

  @override
  Future<Result<List<CoinTransaction>>> getTransactions({
    required String userId,
    int limit = 50,
    int offset = 0,
  }) async {
    final g = gate;
    if (g != null) await g.future;
    final i = calls++;
    return i < _script.length ? _script[i] : _script.last;
  }

  @override
  Future<Result<CoinBalance>> getCoinBalance(String userId) async {
    return Result.error('not used');
  }

  @override
  Future<Result<bool>> hasEnoughCoins({
    required String userId,
    required int requiredAmount,
  }) async {
    return Result.error('not used');
  }

  @override
  Future<Result<int>> getTotalEarnedFromSource({
    required String userId,
    required CoinSourceType sourceType,
  }) async {
    return Result.error('not used');
  }

  @override
  Future<Result<int>> getTotalSpentOnSource({
    required String userId,
    required CoinSourceType sourceType,
  }) async {
    return Result.error('not used');
  }
}

Widget _coinApp(_ScriptedCoinRepository repo) {
  final user = _coinUser();
  return ProviderScope(
    overrides: [
      authenticatedUserProvider.overrideWith((ref) => user),
      coinRepositoryProvider.overrideWithValue(repo),
    ],
    child: MaterialApp(
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      locale: const Locale('id'),
      home: const CoinHistoryScreen(userId: 'user-1'),
    ),
  );
}

void main() {
  // ==========================================================================
  // FEED — first-load vs refresh distinction (unit)
  // ==========================================================================
  group('feed loading foundation (unit)', () {
    test(
      'in-flight refresh keeps items with isRefreshing (no full loading)',
      () async {
        final repo = _BlockingHomeRepository(
          page: FeedPage(
            items: [_feedItem(id: 'v1', content: 'v1')],
            hasMore: false,
          ),
        );
        final container = _feedContainer(repo);
        await _settle();
        expect(container.read(feedProvider).items, hasLength(1));

        // Hold the next fetch open and refresh again.
        repo.page = FeedPage(
          items: [_feedItem(id: 'v2', content: 'v2')],
          hasMore: false,
        );
        repo.gate = Completer<void>();
        final pending = container.read(feedProvider.notifier).refresh();
        await _settle();

        final during = container.read(feedProvider);
        expect(during.items, hasLength(1));
        expect(during.items[0].id, 'v1');
        expect(during.isRefreshing, isTrue);
        expect(during.isLoading, isFalse);
        expect(during.errorMessage, isNull);

        repo.gate!.complete();
        await pending;
        await _settle();
        final after = container.read(feedProvider);
        expect(after.isRefreshing, isFalse);
        expect(after.refreshError, isNull);
        expect(after.items[0].id, 'v2');
      },
    );

    test('refresh failure preserves items with refreshError only', () async {
      final repo = _ToggleHomeRepository(
        page: FeedPage(
          items: [_feedItem(id: 'a-1', content: 'A')],
          hasMore: false,
        ),
      );
      final container = _feedContainer(repo);
      await _settle();
      await container.read(feedProvider.notifier).refresh();
      expect(container.read(feedProvider).items, hasLength(1));

      repo.nextError = Exception('boom');
      await container.read(feedProvider.notifier).refresh();

      final state = container.read(feedProvider);
      expect(state.items, hasLength(1));
      expect(state.items[0].id, 'a-1');
      // First-load error stays null: a refresh failure must never read as a
      // full-page error.
      expect(state.errorMessage, isNull);
      expect(state.refreshError, isNotNull);
      expect(state.isLoading, isFalse);
      expect(state.isRefreshing, isFalse);
    });

    test('first-load failure sets errorMessage (PageErrorState path)', () async {
      final repo = _ToggleHomeRepository(
        page: const FeedPage(items: [], hasMore: false),
      )..nextError = Exception('boom');
      // Fail the very first load by failing refreshFeedItems-independent path:
      // initial build calls loadFeed directly.
      final container = _feedContainer(repo);
      await _settle();

      final state = container.read(feedProvider);
      expect(state.items, isEmpty);
      expect(state.errorMessage, isNotNull);
      expect(state.refreshError, isNull);
      expect(state.isLoading, isFalse);
    });

    test('loadFeed(isRefresh) with no items behaves as first load', () async {
      final repo = _BlockingHomeRepository(
        page: const FeedPage(items: [], hasMore: false),
      );
      final container = _feedContainer(repo);
      await _settle();
      expect(container.read(feedProvider).items, isEmpty);

      // Next fetch held open: with no cached items this is a first load,
      // even when requested as a refresh.
      repo.gate = Completer<void>();
      final pending = container
          .read(feedProvider.notifier)
          .loadFeed(isRefresh: true);
      await _settle();

      final during = container.read(feedProvider);
      expect(during.items, isEmpty);
      expect(during.isLoading, isTrue);
      expect(during.isRefreshing, isFalse);

      repo.gate!.complete();
      await pending;
    });
  });

  // ==========================================================================
  // CHAT — first-load vs refresh distinction (unit)
  // ==========================================================================
  group('chat list loading foundation (unit)', () {
    test('refresh failure preserves chats with refreshError only', () async {
      final repo = _ToggleChatRepository([_chat('c1')]);
      final container = ProviderContainer(
        overrides: [chatRepositoryProvider.overrideWithValue(repo)],
      );
      addTearDown(container.dispose);
      container.listen(chatListProvider, (_, _) {});

      final notifier = container.read(chatListProvider.notifier);
      await notifier.loadChats('me');
      expect(container.read(chatListProvider).chats, hasLength(1));

      repo.nextError = Exception('boom');
      await notifier.loadChats('me', isRefresh: true);

      final state = container.read(chatListProvider);
      expect(state.chats, hasLength(1));
      expect(state.error, isNull);
      expect(state.refreshError, isNotNull);
      expect(state.isLoading, isFalse);
      expect(state.isRefreshing, isFalse);
    });

    test('refresh success replaces chats and clears refreshError', () async {
      final repo = _ToggleChatRepository([_chat('c1')]);
      final container = ProviderContainer(
        overrides: [chatRepositoryProvider.overrideWithValue(repo)],
      );
      addTearDown(container.dispose);
      container.listen(chatListProvider, (_, _) {});

      final notifier = container.read(chatListProvider.notifier);
      await notifier.loadChats('me');

      repo.nextError = Exception('boom');
      await notifier.loadChats('me', isRefresh: true);
      expect(container.read(chatListProvider).refreshError, isNotNull);

      repo.chats = [_chat('c2')];
      await notifier.loadChats('me', isRefresh: true);
      final state = container.read(chatListProvider);
      expect(state.chats.map((c) => c.id), ['c2']);
      expect(state.refreshError, isNull);
      expect(state.error, isNull);
    });

    test('first-load failure sets error (PageErrorState path)', () async {
      final repo = _ToggleChatRepository(const [])..nextError = Exception('x');
      final container = ProviderContainer(
        overrides: [chatRepositoryProvider.overrideWithValue(repo)],
      );
      addTearDown(container.dispose);
      container.listen(chatListProvider, (_, _) {});

      await container.read(chatListProvider.notifier).loadChats('me');

      final state = container.read(chatListProvider);
      expect(state.chats, isEmpty);
      expect(state.error, isNotNull);
      expect(state.refreshError, isNull);
    });

    test('loading() clears a stale error (copyWith clear contract)', () {
      const failed = ChatListState(error: 'stale');
      final loading = failed.loading();
      expect(loading.isLoading, isTrue);
      expect(loading.error, isNull);
    });

    test('clearError clears both error channels', () async {
      final repo = _ToggleChatRepository(const []);
      final container = ProviderContainer(
        overrides: [chatRepositoryProvider.overrideWithValue(repo)],
      );
      addTearDown(container.dispose);
      container.listen(chatListProvider, (_, _) {});

      final notifier = container.read(chatListProvider.notifier);
      notifier.state = const ChatListState(
        chats: [],
        error: 'e',
        refreshError: 'r',
      );
      notifier.clearError();
      expect(container.read(chatListProvider).error, isNull);
      expect(container.read(chatListProvider).refreshError, isNull);
    });
  });

  // ==========================================================================
  // CHAT LIST SCREEN — data stays + inline indication (widget)
  // ==========================================================================
  group('chat list screen (widget)', () {
    testWidgets('cached chats stay with inline banner on refresh failure', (
      tester,
    ) async {
      await tester.pumpWidget(
        _chatApp(ChatListState(chats: [_chat('c1')], refreshError: 'x')),
      );
      await tester.pumpAndSettle();

      expect(find.byType(ChatCard), findsOneWidget);
      expect(find.byType(PageErrorState), findsNothing);
      // Inline refresh-failure copy (safe localized, never raw).
      expect(
        find.text('Data belum bisa dimuat. Silakan coba lagi.'),
        findsOneWidget,
      );
      expect(find.text('Coba Lagi'), findsOneWidget);
    });

    testWidgets('first load shows LoadingIndicator, never EmptyState', (
      tester,
    ) async {
      await tester.pumpWidget(_chatApp(const ChatListState(isLoading: true)));
      await tester.pump();

      expect(find.byType(LoadingIndicator), findsOneWidget);
      expect(find.byType(EmptyState), findsNothing);
      expect(find.byType(PageErrorState), findsNothing);
    });

    testWidgets('first-load error shows PageErrorState, never EmptyState', (
      tester,
    ) async {
      await tester.pumpWidget(
        _chatApp(const ChatListState(error: 'load failed')),
      );
      await tester.pumpAndSettle();

      expect(find.byType(PageErrorState), findsOneWidget);
      expect(find.byType(EmptyState), findsNothing);
      expect(find.byType(LoadingIndicator), findsNothing);
    });
  });

  // ==========================================================================
  // COIN HISTORY SCREEN (widget)
  // ==========================================================================
  group('coin history screen (widget)', () {
    testWidgets('idle/initial shows loading, never empty', (tester) async {
      final repo = _ScriptedCoinRepository([Result.success(const [])])
        ..gate = Completer<void>();
      await tester.pumpWidget(_coinApp(repo));
      await tester.pump();

      // Fetch held open: idle/not-yet-loaded renders loading, never Empty.
      expect(find.byType(LoadingIndicator), findsWidgets);
      expect(find.byType(EmptyState), findsNothing);
      expect(find.byType(PageErrorState), findsNothing);

      repo.gate!.complete();
      await tester.pumpAndSettle();
    });

    testWidgets('successful zero-result shows EmptyState', (tester) async {
      final repo = _ScriptedCoinRepository([Result.success(const [])]);
      await tester.pumpWidget(_coinApp(repo));
      await tester.pumpAndSettle();

      expect(find.byType(EmptyState), findsOneWidget);
      expect(find.byType(PageErrorState), findsNothing);
      expect(find.byType(LoadingIndicator), findsNothing);
    });

    testWidgets('first-load failure shows PageErrorState without raw error', (
      tester,
    ) async {
      final repo = _ScriptedCoinRepository([
        Result.error('INTERNAL_SERVER_ERROR: sql: no rows in result set'),
      ]);
      await tester.pumpWidget(_coinApp(repo));
      await tester.pumpAndSettle();

      expect(find.byType(PageErrorState), findsOneWidget);
      expect(find.byType(EmptyState), findsNothing);
      expect(find.textContaining('INTERNAL_SERVER_ERROR'), findsNothing);
      expect(find.textContaining('sql:'), findsNothing);
    });

    testWidgets('refresh failure keeps cached list with inline banner', (
      tester,
    ) async {
      final repo = _ScriptedCoinRepository([
        Result.success([_tx('tx-1')]),
        Result.error('HTTP 500: boom'),
      ]);
      await tester.pumpWidget(_coinApp(repo));
      await tester.pumpAndSettle();
      expect(find.text('Order reward'), findsOneWidget);

      // Pull-to-refresh triggers a reload (page 1) which fails.
      await tester.fling(find.byType(ListView), const Offset(0, 300), 1000);
      await tester.pumpAndSettle();

      expect(find.text('Order reward'), findsOneWidget);
      expect(find.byType(PageErrorState), findsNothing);
      expect(
        find.text('Data belum bisa dimuat. Silakan coba lagi.'),
        findsOneWidget,
      );
      expect(find.textContaining('HTTP 500'), findsNothing);
    });
  });

  // ==========================================================================
  // NEGATIVE PROOF — obsolete loading behavior cannot return (static)
  // ==========================================================================
  group('negative proof (static contract)', () {
    test('no loading-as-empty semantics anywhere in scope', () {
      for (final path in [
        _feedNotifier,
        _homeScreen,
        _chatNotifier,
        _chatListScreen,
        _coinHistoryScreen,
        _searchResultsScreen,
        _marketplaceGrid,
        _emptyState,
      ]) {
        expect(
          _source(path),
          isNot(contains('EmptyState.loading')),
          reason: path,
        );
      }
      expect(_source(_emptyState), isNot(contains('loading')));
    });

    test('refresh never clears data to start a request', () {
      expect(_source(_feedNotifier), isNot(contains('items: const []')));
    });

    test('in-scope screens use canonical renderers', () {
      for (final path in [
        _homeScreen,
        _chatListScreen,
        _coinHistoryScreen,
        _searchResultsScreen,
      ]) {
        final src = _source(path);
        expect(src, contains('LoadingIndicator('), reason: path);
        expect(src, contains('PageErrorState('), reason: path);
        expect(src, contains('EmptyState('), reason: path);
        expect(
          src,
          isNot(contains('Center(child: CircularProgressIndicator())')),
          reason: path,
        );
      }
      // Marketplace grid default loading is the canonical indicator.
      expect(_source(_marketplaceGrid), contains('LoadingIndicator()'));
      expect(
        _source(_marketplaceGrid),
        isNot(contains('CircularProgressIndicator')),
      );
    });

    test('no raw technical error reaches the screen in scope', () {
      for (final path in [
        _homeScreen,
        _chatListScreen,
        _coinHistoryScreen,
        _searchResultsScreen,
      ]) {
        final src = _source(path);
        // Raw exception text must never be rendered: no toString of the
        // failure, no Text() built from a state error field, no force-unwrap
        // of an error field into the widget tree. (Plain null-checks such as
        // `state.errorMessage != null` stay provider-side and are allowed.)
        expect(src, isNot(contains('error.toString()')), reason: path);
        expect(src, isNot(contains('Text(state.')), reason: path);
        expect(src, isNot(contains('Text(message')), reason: path);
        expect(src, isNot(contains('state.error!')), reason: path);
        expect(src, isNot(contains('state.refreshError!')), reason: path);
        expect(src, isNot(contains('errorMessage!')), reason: path);
      }
      expect(_source(_coinHistoryScreen), isNot(contains('_ErrorState(')));
      expect(_source(_coinHistoryScreen), isNot(contains('_EmptyState(')));
    });

    test('first-load error is gated on empty data in list screens', () {
      // Home: full-page error only when there is nothing to preserve.
      expect(_source(_homeScreen), contains('if (feedItems.isEmpty)'));
      // Chat: error view only when the collection is empty.
      expect(_source(_chatListScreen), contains('if (state.chats.isEmpty)'));
      // Search: null result set without error is still loading, never empty.
      expect(_source(_searchResultsScreen), contains('results == null'));
    });
  });
}
