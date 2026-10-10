import 'dart:async';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/social/follow/data/follow_providers.dart';
import 'package:hishumi/domains/social/follow/domain/entities/follow_entity.dart';
import 'package:hishumi/domains/social/follow/domain/repositories/i_follow_repository.dart';
import 'package:hishumi/domains/social/follow/presentation/screens/follow_list_screen.dart';
import 'package:hishumi/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:hishumi/generated/app_localizations.dart';
import 'package:hishumi/shared/governance/content_lifecycle.dart';
import 'package:hishumi/shared/widgets/empty_state.dart';
import 'package:hishumi/shared/widgets/loading_indicator.dart';
import 'package:hishumi/shared/widgets/page_error_state.dart';

class _FakeAuthController extends AuthController {
  _FakeAuthController(this.stateValue);

  final AuthState stateValue;

  @override
  AuthState build() => stateValue;
}

class _RecordingNavigationHandler implements NavigationHandler {
  String? lastUserId;

  @override
  void navigateToAuction(String auctionId) {}

  @override
  void navigateToChat() {}

  @override
  void navigateToChatConversation(String conversationId) {}

  @override
  void navigateToCoinHistory() {}

  @override
  void navigateToContentDetail(String contentId) {}

  @override
  void navigateToCreateContent() {}

  @override
  void navigateToExternalProductDetail(String productId) {}

  @override
  void navigateToForgotPassword() {}

  @override
  void navigateToHome() {}

  @override
  void navigateToForSaleDetail(String fixedPriceSaleId) {}

  @override
  void navigateToSellerRenewal() {}

  @override
  void navigateToNotifications() {}

  @override
  void navigateToNotificationSettings() {}

  @override
  void navigateToOrderDetail(String orderId) {}

  @override
  void navigateToOrders() {}

  @override
  void navigateToProfile() {}

  @override
  void navigateToSavedItems() {}

  @override
  void navigateToSellerDashboard() {}

  @override
  void navigateToSellerEarnings() {}

  @override
  void navigateToSellerUpgrade() {}

  @override
  void navigateToSellerVerification() {}

  @override
  void navigateToSearch() {}

  @override
  void navigateToSearchResults(String query, {String? type}) {}

  @override
  void navigateToSettings() {}

  @override
  void navigateToSignIn() {}

  @override
  void navigateToSignUp() {}

  @override
  void navigateToUserProfile(String userId) {
    lastUserId = userId;
  }

  @override
  void navigateToWelcome() {}
}

/// Scripted repository: the screen runs the REAL stream providers, so every
/// watch subscription (initial, refresh, retry, post-mutation invalidation)
/// is observable per call.
class _ScriptedFollowRepository implements IFollowRepository {
  _ScriptedFollowRepository({this.onWatchFollowers, this.onWatchFollowing});

  Stream<List<FollowableUser>> Function(int call)? onWatchFollowers;
  Stream<List<FollowableUser>> Function(int call)? onWatchFollowing;

  int watchFollowersCalls = 0;
  int watchFollowingCalls = 0;
  final List<({String followerId, String followingId})> followCalls = [];
  final List<({String followerId, String followingId})> unfollowCalls = [];
  bool followSucceeds = true;
  bool unfollowSucceeds = true;
  String followErrorText = 'boom-follow';
  bool statusResult = false;

  @override
  Stream<List<FollowableUser>> watchFollowers(String userId) {
    watchFollowersCalls++;
    final handler = onWatchFollowers;
    if (handler != null) return handler(watchFollowersCalls);
    return Stream.value(const []);
  }

  @override
  Stream<List<FollowableUser>> watchFollowing(String userId) {
    watchFollowingCalls++;
    final handler = onWatchFollowing;
    if (handler != null) return handler(watchFollowingCalls);
    return Stream.value(const []);
  }

  @override
  Future<Result<bool>> followUser({
    required String followerId,
    required String followingId,
  }) async {
    followCalls.add((followerId: followerId, followingId: followingId));
    return followSucceeds
        ? Result.success(true)
        : Result.error(followErrorText);
  }

  @override
  Future<Result<bool>> unfollowUser({
    required String followerId,
    required String followingId,
  }) async {
    unfollowCalls.add((followerId: followerId, followingId: followingId));
    return unfollowSucceeds
        ? Result.success(true)
        : Result.error(followErrorText);
  }

  @override
  Future<Result<bool>> checkFollowStatus({
    required String followerId,
    required String followingId,
  }) async => Result.success(statusResult);

  @override
  Future<Result<bool>> blockUser({
    required String userId,
    required String targetUserId,
  }) async => Result.success(true);

  @override
  Future<Result<List<FollowableUser>>> getFollowers({
    required String userId,
    int limit = 20,
    String? lastFollowId,
  }) async => Result.success(const []);

  @override
  Future<Result<FollowStats>> getFollowStats({
    required String userId,
    String? currentUserId,
  }) async => Result.success(
    FollowStats(
      userId: userId,
      followersCount: 0,
      followingCount: 0,
      lastUpdated: DateTime.now(),
    ),
  );

  @override
  Future<Result<List<FollowableUser>>> getFollowing({
    required String userId,
    int limit = 20,
    String? lastFollowId,
  }) async => Result.success(const []);

  @override
  Future<Result<bool>> muteUser({
    required String userId,
    required String targetUserId,
  }) async => Result.success(true);

  @override
  Future<Result<List<FollowableUser>>> searchUsers({
    required String query,
    String? currentUserId,
    UserType? filterByType,
    int limit = 20,
  }) async => Result.success(const []);

  @override
  Future<Result<bool>> unblockUser({
    required String userId,
    required String targetUserId,
  }) async => Result.success(true);

  @override
  Future<Result<bool>> unmuteUser({
    required String userId,
    required String targetUserId,
  }) async => Result.success(true);

  @override
  Stream<List<FollowActivity>> watchFollowActivities(String userId) =>
      Stream.value(const []);

  @override
  Stream<FollowStats> watchFollowStats(String userId) => Stream.value(
    FollowStats(
      userId: userId,
      followersCount: 0,
      followingCount: 0,
      lastUpdated: DateTime.now(),
    ),
  );
}

AuthState _unauthenticated() => const AuthState.unauthenticated();

AuthUser _me() => AuthUser(
  id: 'me-1',
  createdAt: DateTime.utc(2026, 1, 1),
  updatedAt: DateTime.utc(2026, 1, 1),
  email: 'me@example.com',
  username: 'me',
  isEmailVerified: true,
  accountStatus: AccountStatus.active,
  roles: const [UserRole.user],
  provider: AuthProvider.email,
  lifecycle: ContentLifecycle.active,
);

Widget _wrap({
  required Widget child,
  required IFollowRepository repository,
  required _RecordingNavigationHandler navigation,
  AuthState? authState,
}) {
  return ProviderScope(
    // Retry is disabled so a failed stream does not schedule timers.
    retry: (retryCount, error) => null,
    overrides: [
      authControllerProvider.overrideWith(
        () => _FakeAuthController(authState ?? _unauthenticated()),
      ),
      followRepositoryProvider.overrideWithValue(repository),
      navigationHandlerProvider.overrideWithValue(navigation),
    ],
    child: MaterialApp(
      home: child,
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      locale: const Locale('id'),
    ),
  );
}

FollowableUser _user(String id, String username) => FollowableUser(
  id: id,
  username: username,
  avatar: null,
  userType: UserType.buyer,
  lifecycle: 'active',
  followersCount: 1,
  followingCount: 2,
);

List<FollowableUser> _followersFixture() {
  return [
    FollowableUser(
      id: 'user-bob',
      username: 'bob',
      avatar: null,
      userType: UserType.buyer,
      lifecycle: 'active',
      followersCount: 3,
      followingCount: 4,
    ),
    FollowableUser(
      id: 'user-empty',
      username: '',
      avatar: null,
      userType: UserType.buyer,
      lifecycle: 'active',
      followersCount: 0,
      followingCount: 0,
    ),
    FollowableUser(
      id: 'user-removed',
      username: 'ghost',
      avatar: 'https://cdn.example.com/ghost.jpg',
      userType: UserType.buyer,
      lifecycle: 'removed',
    ),
  ];
}

List<FollowableUser> _followingFixture() {
  return [
    FollowableUser(
      id: 'user-charlie',
      username: 'charlie',
      avatar: null,
      userType: UserType.buyer,
      lifecycle: 'active',
      followersCount: 1,
      followingCount: 2,
    ),
  ];
}

_ScriptedFollowRepository _repoOf({
  List<FollowableUser>? followers,
  List<FollowableUser>? following,
}) => _ScriptedFollowRepository(
  onWatchFollowers: (_) => Stream.value(followers ?? const []),
  onWatchFollowing: (_) => Stream.value(following ?? const []),
);

Future<void> _pumpFollowers(
  WidgetTester tester, {
  required _ScriptedFollowRepository repository,
  required _RecordingNavigationHandler navigation,
  AuthState? authState,
  String userId = 'owner-1',
  String? username = 'Owner',
  bool settle = true,
}) async {
  await tester.pumpWidget(
    _wrap(
      repository: repository,
      navigation: navigation,
      authState: authState,
      child: FollowListScreen(
        userId: userId,
        type: FollowListType.followers,
        username: username,
      ),
    ),
  );
  if (settle) await tester.pumpAndSettle();
}

void main() {
  testWidgets(
    'followers mode preserves order, handles empty username, and navigates with stable ID',
    (tester) async {
      final navigation = _RecordingNavigationHandler();
      final repository = _ScriptedFollowRepository(
        onWatchFollowers: (_) => Stream.value(_followersFixture()),
        onWatchFollowing: (_) => Stream.value(_followingFixture()),
      );

      await tester.pumpWidget(
        _wrap(
          repository: repository,
          navigation: navigation,
          child: FollowListScreen(
            userId: 'owner-1',
            type: FollowListType.followers,
            username: 'Owner',
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text("Owner's Followers"), findsOneWidget);
      expect(find.text('@bob'), findsOneWidget);
      expect(find.text('@'), findsOneWidget);
      expect(find.text('Pengguna dihapus'), findsOneWidget);

      final bobTop = tester.getTopLeft(find.text('@bob')).dy;
      final emptyTop = tester.getTopLeft(find.text('@')).dy;
      final removedTop = tester.getTopLeft(find.text('Pengguna dihapus')).dy;

      expect(bobTop, lessThan(emptyTop));
      expect(emptyTop, lessThan(removedTop));

      await tester.tap(find.text('@bob'));
      await tester.pumpAndSettle();

      expect(navigation.lastUserId, 'user-bob');

      await tester.tap(find.text('Pengguna dihapus'));
      await tester.pumpAndSettle();

      expect(navigation.lastUserId, 'user-bob');
    },
  );

  testWidgets('search filters by username and empty username stays safe', (
    tester,
  ) async {
    final navigation = _RecordingNavigationHandler();
    final repository = _ScriptedFollowRepository(
      onWatchFollowers: (_) => Stream.value(_followersFixture()),
      onWatchFollowing: (_) => Stream.value(_followingFixture()),
    );

    await tester.pumpWidget(
      _wrap(
        repository: repository,
        navigation: navigation,
        child: FollowListScreen(
          userId: 'owner-1',
          type: FollowListType.followers,
          username: 'Owner',
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.enterText(find.byType(TextField), 'bob');
    await tester.pumpAndSettle();

    expect(find.text('@bob'), findsOneWidget);
    expect(find.text('@'), findsNothing);
    expect(find.text('Pengguna dihapus'), findsNothing);

    await tester.enterText(find.byType(TextField), '');
    await tester.pumpAndSettle();

    expect(find.text('@'), findsOneWidget);
  });

  testWidgets('following mode switches cleanly to following ownership', (
    tester,
  ) async {
    final navigation = _RecordingNavigationHandler();
    final repository = _ScriptedFollowRepository(
      onWatchFollowers: (_) => Stream.value(_followersFixture()),
      onWatchFollowing: (_) => Stream.value(_followingFixture()),
    );

    await tester.pumpWidget(
      _wrap(
        repository: repository,
        navigation: navigation,
        child: FollowListScreen(
          userId: 'owner-2',
          type: FollowListType.following,
          username: 'Owner',
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text("Owner's Following"), findsOneWidget);
    expect(find.text('@charlie'), findsOneWidget);
    expect(find.text('@bob'), findsNothing);
  });

  group('FollowListScreen — initial state', () {
    testWidgets('first request shows LoadingIndicator, never empty/error', (
      tester,
    ) async {
      final gate = StreamController<List<FollowableUser>>();
      final repository = _ScriptedFollowRepository(
        onWatchFollowers: (_) => gate.stream,
      );
      await _pumpFollowers(
        tester,
        repository: repository,
        navigation: _RecordingNavigationHandler(),
        settle: false,
      );
      await tester.pump();
      await tester.pump();

      expect(find.byType(LoadingIndicator), findsOneWidget);
      expect(find.byType(EmptyState), findsNothing);
      expect(find.byType(PageErrorState), findsNothing);

      gate.add(_followersFixture());
      await tester.pumpAndSettle();
      expect(find.text('@bob'), findsOneWidget);
      await gate.close();
    });

    testWidgets('successful zero-result shows EmptyState per type', (
      tester,
    ) async {
      final navigation = _RecordingNavigationHandler();
      await _pumpFollowers(
        tester,
        repository: _repoOf(followers: const []),
        navigation: navigation,
      );

      expect(find.byType(EmptyState), findsOneWidget);
      expect(find.text('Belum ada pengikut'), findsOneWidget);
      expect(find.byType(PageErrorState), findsNothing);
      expect(find.byType(LoadingIndicator), findsNothing);

      await tester.pumpWidget(
        _wrap(
          repository: _repoOf(following: const []),
          navigation: navigation,
          child: const FollowListScreen(
            userId: 'owner-2',
            type: FollowListType.following,
            username: 'Owner',
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.byType(EmptyState), findsOneWidget);
      expect(find.text('Belum mengikuti siapa pun'), findsOneWidget);
    });
  });

  group('FollowListScreen — initial failure', () {
    testWidgets('failure with no data shows PageErrorState, retry reloads', (
      tester,
    ) async {
      final repository = _ScriptedFollowRepository(
        onWatchFollowers: (call) => call == 1
            ? Stream<List<FollowableUser>>.error(Exception('boom-initial'))
            : Stream.value([_user('user-x', 'xena')]),
      );
      await _pumpFollowers(
        tester,
        repository: repository,
        navigation: _RecordingNavigationHandler(),
      );

      // CANONICAL error surface: safe localized copy only — the raw
      // backend text must never reach the screen.
      expect(find.byType(PageErrorState), findsOneWidget);
      expect(find.text('Terjadi Kesalahan'), findsOneWidget);
      expect(
        find.text('Data belum bisa dimuat. Silakan coba lagi.'),
        findsOneWidget,
      );
      expect(find.text('Coba Lagi'), findsOneWidget);
      expect(find.textContaining('boom-initial'), findsNothing);
      expect(find.byType(EmptyState), findsNothing);

      // Retry executes the actual initial-load operation.
      expect(repository.watchFollowersCalls, 1);
      await tester.tap(find.widgetWithText(ElevatedButton, 'Coba Lagi'));
      await tester.pumpAndSettle();

      expect(repository.watchFollowersCalls, 2);
      expect(find.byType(PageErrorState), findsNothing);
      expect(find.text('@xena'), findsOneWidget);
    });
  });

  group('FollowListScreen — refresh', () {
    testWidgets('existing rows stay visible with refresh indicator', (
      tester,
    ) async {
      final gate = StreamController<List<FollowableUser>>();
      final repository = _ScriptedFollowRepository(
        onWatchFollowers: (call) =>
            call == 1 ? Stream.value([_user('user-a', 'anna')]) : gate.stream,
      );
      await _pumpFollowers(
        tester,
        repository: repository,
        navigation: _RecordingNavigationHandler(),
      );
      expect(find.text('@anna'), findsOneWidget);

      await tester.fling(
        find.byType(CustomScrollView),
        const Offset(0, 300),
        1000,
      );
      // Allow the RefreshIndicator to fire onRefresh and the resubscribe to
      // start (bounded: the gate stays open, so never settle here).
      for (var i = 0; i < 50 && repository.watchFollowersCalls < 2; i++) {
        await tester.pump(const Duration(milliseconds: 50));
      }
      expect(repository.watchFollowersCalls, 2);

      // Refresh must not clear the list into full loading.
      expect(find.text('@anna'), findsOneWidget);
      expect(find.byType(LinearProgressIndicator), findsOneWidget);
      expect(find.byType(LoadingIndicator), findsNothing);
      expect(find.byType(PageErrorState), findsNothing);

      gate.add([_user('user-b', 'bima')]);
      await tester.pumpAndSettle();

      expect(find.text('@bima'), findsOneWidget);
      expect(find.text('@anna'), findsNothing);
      expect(find.byType(LinearProgressIndicator), findsNothing);
      await gate.close();
    });

    testWidgets(
      'refresh failure keeps rows with inline banner, retry recovers',
      (tester) async {
        final repository = _ScriptedFollowRepository(
          onWatchFollowers: (call) {
            if (call == 1) {
              return Stream.value([_user('user-a', 'anna')]);
            }
            if (call == 2) {
              return Stream<List<FollowableUser>>.error(
                Exception('boom-refresh'),
              );
            }
            return Stream.value([_user('user-b', 'bima')]);
          },
        );
        await _pumpFollowers(
          tester,
          repository: repository,
          navigation: _RecordingNavigationHandler(),
        );
        expect(find.text('@anna'), findsOneWidget);

        await tester.fling(
          find.byType(CustomScrollView),
          const Offset(0, 300),
          1000,
        );
        await tester.pumpAndSettle();

        // Valid data is preserved; failure renders inline, never full-page.
        expect(find.text('@anna'), findsOneWidget);
        expect(find.byType(PageErrorState), findsNothing);
        expect(
          find.text('Data belum bisa dimuat. Silakan coba lagi.'),
          findsOneWidget,
        );
        expect(find.widgetWithText(TextButton, 'Coba Lagi'), findsOneWidget);
        expect(find.textContaining('boom-refresh'), findsNothing);

        // Retry executes refresh; success replaces stale data and clears banner.
        await tester.tap(find.widgetWithText(TextButton, 'Coba Lagi'));
        await tester.pumpAndSettle();

        expect(repository.watchFollowersCalls, 3);
        expect(find.text('@bima'), findsOneWidget);
        expect(find.text('@anna'), findsNothing);
        expect(
          find.text('Data belum bisa dimuat. Silakan coba lagi.'),
          findsNothing,
        );
      },
    );
  });

  group('FollowListScreen — follow / unfollow mutation', () {
    testWidgets('follow reloads through the canonical stream path', (
      tester,
    ) async {
      final repository = _ScriptedFollowRepository(
        onWatchFollowers: (_) => Stream.value([_user('user-x', 'xena')]),
      );
      // Viewing the signed-in owner's own list: the mutation invalidates
      // this exact watched key.
      await _pumpFollowers(
        tester,
        repository: repository,
        navigation: _RecordingNavigationHandler(),
        authState: AuthState.authenticated(_me(), emailVerified: true),
        userId: 'me-1',
      );
      expect(find.text('@xena'), findsOneWidget);
      expect(find.text('Follow'), findsOneWidget);

      await tester.tap(find.text('Follow'));
      await tester.pumpAndSettle();

      expect(repository.followCalls, [
        (followerId: 'me-1', followingId: 'user-x'),
      ]);
      // The mutation invalidated the canonical stream key: one reload.
      expect(repository.watchFollowersCalls, 2);
      expect(find.text('Mulai mengikuti'), findsOneWidget);
      expect(find.text('@xena'), findsOneWidget);
      expect(find.byType(PageErrorState), findsNothing);
    });

    testWidgets('unfollow reloads through the canonical stream path', (
      tester,
    ) async {
      final repository = _ScriptedFollowRepository(
        onWatchFollowers: (_) => Stream.value([_user('user-x', 'xena')]),
      )..statusResult = true;
      await _pumpFollowers(
        tester,
        repository: repository,
        navigation: _RecordingNavigationHandler(),
        authState: AuthState.authenticated(_me(), emailVerified: true),
        userId: 'me-1',
      );
      expect(find.text('Following'), findsOneWidget);

      await tester.tap(find.text('Following'));
      await tester.pumpAndSettle();

      expect(repository.unfollowCalls, [
        (followerId: 'me-1', followingId: 'user-x'),
      ]);
      expect(repository.watchFollowersCalls, 2);
      expect(find.text('Berhenti mengikuti'), findsOneWidget);
      expect(find.text('@xena'), findsOneWidget);
    });

    testWidgets('failed mutation keeps rows and never reloads into empty', (
      tester,
    ) async {
      final repository = _ScriptedFollowRepository(
        onWatchFollowers: (_) => Stream.value([_user('user-x', 'xena')]),
      )..followSucceeds = false;
      await _pumpFollowers(
        tester,
        repository: repository,
        navigation: _RecordingNavigationHandler(),
        authState: AuthState.authenticated(_me(), emailVerified: true),
      );

      await tester.tap(find.text('Follow'));
      await tester.pumpAndSettle();

      expect(repository.followCalls, hasLength(1));
      // Failure invalidates nothing: no reload, no wipe, no full-page error.
      expect(repository.watchFollowersCalls, 1);
      expect(find.text('@xena'), findsOneWidget);
      expect(find.byType(PageErrorState), findsNothing);
      expect(find.byType(EmptyState), findsNothing);
    });
  });

  group('FollowListScreen — negative proof (static contract)', () {
    String screenSource() => File(
      'lib/domains/social/follow/presentation/screens/follow_list_screen.dart',
    ).readAsStringSync().replaceAll('\r\n', '\n');

    String statusSource() => File(
      'lib/domains/social/follow/presentation/providers/follow_status_provider.dart',
    ).readAsStringSync().replaceAll('\r\n', '\n');

    test('canonical renderers own every page state', () {
      final src = screenSource();
      expect(src.contains('LoadingIndicator('), isTrue);
      expect(src.contains('PageErrorState('), isTrue);
      expect(src.contains('EmptyState('), isTrue);
    });

    test('no raw first-load spinner or local error renderer remains', () {
      final src = screenSource();
      expect(src.contains('CircularProgressIndicator('), isFalse);
      expect(src.contains('_buildErrorState'), isFalse);
    });

    test('no raw technical error reaches the widget tree', () {
      final src = screenSource();
      expect(src.contains('error.toString()'), isFalse);
      expect(src.contains('Text(error'), isFalse);
    });

    test('single reload path: refresh, never invalidate-and-clear', () {
      final src = screenSource();
      expect('ref.refresh('.allMatches(src).length, 1);
      expect(src.contains('ref.invalidate(followersStreamProvider'), isFalse);
      expect(src.contains('ref.invalidate(followingStreamProvider'), isFalse);
    });

    test('no duplicate authority in the touched surface', () {
      final screen = screenSource();
      expect(screen.contains('NotifierProvider'), isFalse);
      expect(screen.contains('followListsProvider'), isFalse);
      expect(screen.contains('FollowListsNotifier'), isFalse);
      expect(statusSource().contains('followListsProvider'), isFalse);
      expect(
        File(
          'lib/domains/social/follow/presentation/providers/follow_lists_provider.dart',
        ).existsSync(),
        isFalse,
      );
      expect(
        File(
          'lib/domains/social/follow/presentation/providers/follow_lists_provider.g.dart',
        ).existsSync(),
        isFalse,
      );
    });

    test('mutations invalidate the canonical stream keys', () {
      final src = statusSource();
      expect(src.contains('ref.invalidate(followersStreamProvider('), isTrue);
      expect(src.contains('ref.invalidate(followingStreamProvider('), isTrue);
      expect(src.contains('ref.invalidate(followStatsStreamProvider('), isTrue);
    });
  });
}
