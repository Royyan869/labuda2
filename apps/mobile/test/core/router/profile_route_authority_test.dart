import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/core/src/router/modules/profile_module.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart';
import 'package:labuda/domains/user/profile/data/services/avatar_upload_service.dart';
import 'package:labuda/domains/user/profile/data/services/cover_photo_upload_service.dart';
import 'package:labuda/domains/user/profile/domain/entities/profile_entity.dart';
import 'package:labuda/domains/user/profile/domain/repositories/i_profile_repository.dart';
import 'package:labuda/domains/user/profile/presentation/screens/personal_information_screen.dart';
import 'package:labuda/domains/user/profile/presentation/screens/unified_edit_profile_screen.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/generated/app_localizations.dart';
import 'package:labuda/domains/user/preference/seller/data/services/store_photo_upload_service.dart';
import 'package:labuda/domains/user/preference/seller/data/seller_providers.dart'
    show storePhotoUploadServiceProvider;
import 'package:labuda/shared/governance/content_lifecycle.dart';

class _FakeAuthController extends AuthController {
  _FakeAuthController(this._state);

  final AuthState _state;

  @override
  AuthState build() => _state;
}

class _FakeProfileRepository implements IProfileRepository {
  _FakeProfileRepository(this._profile);

  final ProfileEntity _profile;

  @override
  Stream<ProfileEntity?> watchProfile(String userId) =>
      Stream.value(userId == _profile.userId ? _profile : null);

  @override
  noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _FakeAvatarUploadService implements AvatarUploadService {
  @override
  noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _FakeCoverPhotoUploadService implements CoverPhotoUploadService {
  @override
  noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _FakeStorePhotoUploadService implements StorePhotoUploadService {
  @override
  noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

AuthUser _authUser({required String id, required bool seller}) {
  return AuthUser(
    id: id,
    createdAt: DateTime.utc(2026, 1, 1),
    updatedAt: DateTime.utc(2026, 1, 1),
    email: '$id@example.com',
    username: id,
    avatarUrl: null,
    bio: 'Profile bio',
    isEmailVerified: true,
    phoneNumber: '+628123456789',
    dateOfBirth: DateTime.utc(1994, 1, 1),
    hasSellerProfile: seller,
    sellerSubscriptionStatus: seller ? 'active' : 'none',
    hasMarketAuthority: seller,
    accountStatus: AccountStatus.active,
    roles: const [UserRole.user],
    provider: ShonaAuthProvider.email,
    lifecycle: ContentLifecycle.active,
  );
}

AuthState _authState({required String id, required bool seller}) {
  return AuthState.authenticated(
    _authUser(id: id, seller: seller),
    emailVerified: true,
  );
}

ProfileEntity _sellerProfile(String userId) {
  return ProfileEntity(
    id: 'profile-$userId',
    userId: userId,
    location: 'Depok, West Java',
    coverPhotoUrl: null,
    joinedAt: DateTime.utc(2026, 1, 1),
    lastActiveAt: DateTime.utc(2026, 6, 1),
    stats: const ProfileStats(followersCount: 12, followingCount: 7),
    verification: const UserVerificationInfo(
      isPhoneVerified: true,
      isEmailVerified: true,
      isIdVerified: false,
      isFarmVerified: false,
      badges: <ProfileBadge>[],
    ),
    contactInfo: const ContactInfo(
      maskedPhone: '0812***6789',
      maskedEmail: 'owner***@example.com',
      isPhonePublic: true,
      isEmailPublic: true,
      instagramHandle: 'labuda_farm',
      facebookHandle: 'Labuda Farm',
      tiktokHandle: 'labuda_farm',
      twitterHandle: 'labuda_farm',
      isSocialMediaPublic: true,
    ),
    farmInfo: FarmInfo(
      farmName: 'Labuda Farm',
      farmWebsite: 'https://example.com',
      specialties: <String>['organic'],
      establishedDate: DateTime.utc(2020, 1, 1),
    ),
  );
}

Future<void> _pumpRouterApp(
  WidgetTester tester, {
  required AuthState authState,
  required String initialLocation,
  NavigatorObserver? observer,
  ProfileEntity? profile,
}) async {
  final overrides = [
    authControllerProvider.overrideWith(() => _FakeAuthController(authState)),
  ];

  if (profile != null) {
    overrides.addAll([
      profileRepositoryProvider.overrideWithValue(
        _FakeProfileRepository(profile),
      ),
      avatarUploadServiceProvider.overrideWithValue(_FakeAvatarUploadService()),
      coverPhotoUploadServiceProvider.overrideWithValue(
        _FakeCoverPhotoUploadService(),
      ),
      storePhotoUploadServiceProvider.overrideWithValue(
        _FakeStorePhotoUploadService(),
      ),
    ]);
  }

  final router = GoRouter(
    routes: ProfileModule().routes,
    initialLocation: initialLocation,
    observers: observer == null ? const [] : [observer],
  );

  await tester.pumpWidget(
    ProviderScope(
      overrides: overrides,
      child: MaterialApp.router(
        routerConfig: router,
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        locale: const Locale('en'),
      ),
    ),
  );

  await tester.pumpAndSettle();
}

void main() {
  test('profile module exposes canonical profile routes', () {
    final routes = ProfileModule().routes;
    final paths = routes.map((route) => route.path).toList();

    expect(paths, contains(RoutePaths.profile));
    expect(paths, contains(RoutePaths.userProfile));
    expect(paths, contains(RoutePaths.editProfile));
    expect(paths, contains(RoutePaths.personalInformation));
    expect(paths, contains(RoutePaths.settings));
    expect(
      routes.singleWhere((route) => route.path == RoutePaths.userProfile).name,
      RouteNames.userProfile,
    );
    expect(
      routes.singleWhere((route) => route.path == RoutePaths.editProfile).name,
      RouteNames.editProfile,
    );
    expect(
      routes
          .singleWhere((route) => route.path == RoutePaths.personalInformation)
          .name,
      RouteNames.personalInformation,
    );
  });

  testWidgets('settings edit profile tile opens the unified editor route', (
    tester,
  ) async {
    final observer = _RecordingNavigatorObserver();
    await _pumpRouterApp(
      tester,
      authState: _authState(id: 'owner-1', seller: false),
      initialLocation: RoutePaths.settings,
      observer: observer,
    );

    await tester.tap(find.text('Edit Profile'));
    await tester.pumpAndSettle();

    expect(observer.pushCount, greaterThan(0));
    expect(find.byType(UnifiedEditProfileScreen), findsOneWidget);
    expect(find.text('Informasi Profile'), findsOneWidget);
  });

  testWidgets('settings personal information tile opens the personal route', (
    tester,
  ) async {
    final observer = _RecordingNavigatorObserver();
    await _pumpRouterApp(
      tester,
      authState: _authState(id: 'owner-1', seller: false),
      initialLocation: RoutePaths.settings,
      observer: observer,
    );

    await tester.tap(find.text('Personal Information'));
    await tester.pumpAndSettle();

    expect(observer.pushCount, greaterThan(0));
    expect(find.byType(PersonalInformationScreen), findsOneWidget);
  });

  testWidgets(
    'unified edit profile scrolls to the business section when requested',
    (tester) async {
      final userId = 'seller-1';

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            authControllerProvider.overrideWith(
              () => _FakeAuthController(_authState(id: userId, seller: true)),
            ),
            profileRepositoryProvider.overrideWithValue(
              _FakeProfileRepository(_sellerProfile(userId)),
            ),
            avatarUploadServiceProvider.overrideWithValue(
              _FakeAvatarUploadService(),
            ),
            coverPhotoUploadServiceProvider.overrideWithValue(
              _FakeCoverPhotoUploadService(),
            ),
            storePhotoUploadServiceProvider.overrideWithValue(
              _FakeStorePhotoUploadService(),
            ),
          ],
          child: MaterialApp(
            home: UnifiedEditProfileScreen(
              userId: userId,
              initialSection: UnifiedEditProfileSection.business,
            ),
          ),
        ),
      );

      await tester.pumpAndSettle();

      expect(find.byType(UnifiedEditProfileScreen), findsOneWidget);
      expect(find.text('Farm Information'), findsOneWidget);

      final scrollable = find.descendant(
        of: find.byType(UnifiedEditProfileScreen),
        matching: find.byWidgetPredicate(
          (widget) =>
              widget is Scrollable &&
              widget.axisDirection == AxisDirection.down,
        ),
      );
      final scrolledState = scrollable
          .evaluate()
          .map((element) {
            return tester.state<ScrollableState>(find.byWidget(element.widget));
          })
          .firstWhere(
            (state) => state.position.pixels > 0,
            orElse: () =>
                throw TestFailure('No vertical scrollable moved down'),
          );

      expect(scrolledState.position.pixels, greaterThan(0.0));
    },
  );
}

class _RecordingNavigatorObserver extends NavigatorObserver {
  int pushCount = 0;

  @override
  void didPush(Route<dynamic> route, Route<dynamic>? previousRoute) {
    pushCount += 1;
    super.didPush(route, previousRoute);
  }
}
