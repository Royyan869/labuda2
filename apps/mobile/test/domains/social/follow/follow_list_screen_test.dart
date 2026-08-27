import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/follow/data/follow_providers.dart';
import 'package:labuda/domains/social/follow/domain/entities/follow_entity.dart';
import 'package:labuda/domains/social/follow/domain/repositories/i_follow_repository.dart';
import 'package:labuda/domains/social/follow/presentation/screens/follow_list_screen.dart';

class _FakeAuthController extends AuthController {
  _FakeAuthController(this.stateValue);

  final AuthState stateValue;

  @override
  AuthState build() => stateValue;
}

class _RecordingNavigationHandler implements NavigationHandler {
  String? lastUserId;

  @override
  void navigateBack() {}

  @override
  void navigateToAddressPayment() {}

  @override
  void navigateToAuction(String auctionId) {}

  @override
  void navigateToBusinessDocuments() {}

  @override
  void navigateToChat() {}

  @override
  void navigateToChatConversation(String conversationId) {}

  @override
  void navigateToCheckout() {}

  @override
  void navigateToCoinBalance() {}

  @override
  void navigateToCoinHistory() {}

  @override
  void navigateToContentDetail(String contentId) {}

  @override
  void navigateToCreateAuction() {}

  @override
  void navigateToCreateContent() {}

  @override
  void navigateToCreateListing() {}

  @override
  void navigateToEditProfile() {}

  @override
  void navigateToExternalProductDetail(String productId) {}

  @override
  void navigateToForgotPassword() {}

  @override
  void navigateToHome() {}

  @override
  void navigateToKycVerification({String? userId}) {}

  @override
  void navigateToListingDetail(String fixedPriceSaleId) {}

  @override
  void navigateToLogin() {}

  @override
  void navigateToNotifications() {}

  @override
  void navigateToNotificationSettings() {}

  @override
  void navigateToOrderDetail(String orderId) {}

  @override
  void navigateToOrderHistory() {}

  @override
  void navigateToOrders() {}

  @override
  void navigateToPayment(paymentRequest) {}

  @override
  void navigateToPrivacySettings() {}

  @override
  void navigateToBlockedUsers() {}

  @override
  void navigateToProfile() {}

  @override
  void navigateToRegister() {}

  @override
  void navigateToSavedItems() {}

  @override
  void navigateToSecurity() {}

  @override
  void navigateToSellerDashboard() {}

  @override
  void navigateToSellerEarnings() {}

  @override
  void navigateToSellerForSales() {}

  @override
  void navigateToSellerRefundList() {}

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

  @override
  void showBottomSheet<T>(Widget Function(BuildContext p1) builder) {}

  @override
  void showModalDialog<T>(Widget Function(BuildContext p1) builder) {}

  @override
  void showSnackBar(String message, {bool isError = false}) {}
}

class _FakeFollowRepository implements IFollowRepository {
  _FakeFollowRepository({required this.followers, required this.following});

  final Stream<List<FollowableUser>> followers;
  final Stream<List<FollowableUser>> following;

  @override
  Future<Result<bool>> blockUser({
    required String userId,
    required String targetUserId,
  }) async => Result.success(true);

  @override
  Future<Result<bool>> checkFollowStatus({
    required String followerId,
    required String followingId,
  }) async => Result.success(false);

  @override
  Future<Result<bool>> followUser({
    required String followerId,
    required String followingId,
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
  Future<Result<bool>> unfollowUser({
    required String followerId,
    required String followingId,
  }) async => Result.success(true);

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

  @override
  Stream<List<FollowableUser>> watchFollowers(String userId) => followers;

  @override
  Stream<List<FollowableUser>> watchFollowing(String userId) => following;
}

AuthState _unauthenticated() => const AuthState.unauthenticated();

Widget _wrap({
  required Widget child,
  required IFollowRepository repository,
  required _RecordingNavigationHandler navigation,
}) {
  return ProviderScope(
    overrides: [
      authControllerProvider.overrideWith(
        () => _FakeAuthController(_unauthenticated()),
      ),
      followRepositoryProvider.overrideWithValue(repository),
      navigationHandlerProvider.overrideWithValue(navigation),
    ],
    child: MaterialApp(home: child),
  );
}

List<FollowableUser> _followersFixture() {
  return [
    FollowableUser(
      id: 'user-bob',
      username: 'bob',
      displayName: 'Bob',
      avatar: null,
      userType: UserType.buyer,
      lifecycle: 'active',
      followersCount: 3,
      followingCount: 4,
    ),
    FollowableUser(
      id: 'user-empty',
      username: '',
      displayName: 'User',
      avatar: null,
      userType: UserType.buyer,
      lifecycle: 'active',
      followersCount: 0,
      followingCount: 0,
    ),
    FollowableUser(
      id: 'user-removed',
      username: 'ghost',
      displayName: 'Ghost',
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
      displayName: 'Charlie',
      avatar: null,
      userType: UserType.buyer,
      lifecycle: 'active',
      followersCount: 1,
      followingCount: 2,
    ),
  ];
}

void main() {
  testWidgets(
    'followers mode preserves order, handles empty username, and navigates with stable ID',
    (tester) async {
      final navigation = _RecordingNavigationHandler();
      final repository = _FakeFollowRepository(
        followers: Stream.value(_followersFixture()),
        following: Stream.value(_followingFixture()),
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
    final repository = _FakeFollowRepository(
      followers: Stream.value(_followersFixture()),
      following: Stream.value(_followingFixture()),
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
    final repository = _FakeFollowRepository(
      followers: Stream.value(_followersFixture()),
      following: Stream.value(_followingFixture()),
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
}
