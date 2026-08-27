import 'dart:io';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/rating/domain/entities/rating_entity.dart';
import 'package:labuda/domains/user/profile/domain/entities/profile_entity.dart';
import 'package:labuda/domains/user/profile/domain/repositories/i_profile_repository.dart';
import 'package:labuda/domains/user/profile/presentation/helpers/profile_rating_summary_state.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart'
    show profileRepositoryProvider;
import 'package:labuda/domains/user/profile/presentation/providers/profile_providers.dart'
    show currentUserProfileProvider;

class _MutableAuthController extends AuthController {
  _MutableAuthController(this._state);

  AuthState _state;

  @override
  AuthState build() => _state;

  void setState(AuthState next) {
    _state = next;
    state = next;
  }
}

class _FakeProfileRepository implements IProfileRepository {
  _FakeProfileRepository(this.profileForUser);

  final ProfileEntity? Function(String userId) profileForUser;

  @override
  Future<Result<ProfileEntity?>> getProfile(String userId) async =>
      Result.success(profileForUser(userId));

  @override
  Future<Result<bool>> profileExists(String userId) async =>
      Result.success(profileForUser(userId) != null);

  @override
  Stream<ProfileEntity?> watchProfile(String userId) =>
      Stream.value(profileForUser(userId));

  @override
  Future<Result<ProfileStats>> getProfileStats(String userId) async =>
      Result.success(const ProfileStats(followersCount: 0, followingCount: 0));

  @override
  Future<Result<List<ProfileEntity>>> getMultipleProfiles(
    List<String> userIds,
  ) async => Result.success(const <ProfileEntity>[]);

  @override
  Future<Result<List<ProfileEntity>>> getProfilesByType(
    UserRole userRole, {
    int limit = 20,
    String? lastDocumentId,
  }) async => Result.success(const <ProfileEntity>[]);

  @override
  Future<Result<List<ProfileEntity>>> getTrendingProfiles({
    int limit = 10,
  }) async => Result.success(const <ProfileEntity>[]);

  @override
  Future<Result<List<ProfileEntity>>> searchProfiles(
    String query, {
    int limit = 20,
    String? lastDocumentId,
  }) async => Result.success(const <ProfileEntity>[]);

  @override
  Future<Result<ProfileEntity>> createProfile(ProfileEntity profile) =>
      throw UnimplementedError('Not used');

  @override
  Future<Result<ProfileEntity>> updateProfile(ProfileEntity profile) =>
      throw UnimplementedError('Not used');

  @override
  Future<Result<List<ProfileEntity>>> getVerifiedSellers({
    int limit = 20,
    String? lastDocumentId,
  }) => throw UnimplementedError('Not used');

  @override
  noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

AuthUser _authUser({
  required String id,
  required String username,
  bool hasSellerProfile = true,
}) {
  return AuthUser(
    id: id,
    createdAt: DateTime.utc(2026, 1, 1),
    updatedAt: DateTime.utc(2026, 1, 1),
    email: '$username@example.com',
    username: username,
    isEmailVerified: true,
    hasSellerProfile: hasSellerProfile,
    hasMarketAuthority: hasSellerProfile,
    roles: const [UserRole.user],
    provider: ShonaAuthProvider.email,
  );
}

ProfileEntity _profile(String userId) {
  return ProfileEntity(
    id: 'profile-$userId',
    userId: userId,
    joinedAt: DateTime.utc(2026, 1, 1),
    stats: const ProfileStats(followersCount: 0, followingCount: 0),
    verification: const UserVerificationInfo(
      isPhoneVerified: false,
      isEmailVerified: false,
      isIdVerified: false,
      isFarmVerified: false,
      badges: <ProfileBadge>[],
    ),
  );
}

String _readSource(String relativePath) {
  return File(relativePath).readAsStringSync();
}

void main() {
  group('Profile rating summary authority', () {
    test('loading state never collapses into a zero rating', () {
      const state = ProfileRatingSummaryState.loading();

      expect(state.isLoading, isTrue);
      expect(state.averageText, isNot('0.0'));
      expect(state.averageText, '...');
      expect(state.totalText, '...');
    });

    test('successful empty summary keeps zero values and empty semantics', () {
      final state = ProfileRatingSummaryState.fromAsyncValue(
        AsyncData(
          Result.success(
            const RatingSummary(
              totalRatings: 0,
              averageRating: 0.0,
              oneStarCount: 0,
              twoStarCount: 0,
              threeStarCount: 0,
              fourStarCount: 0,
              fiveStarCount: 0,
            ),
          ),
        ),
      );

      expect(state.isEmpty, isTrue);
      expect(state.averageText, '0.0');
      expect(state.totalText, '0');
      expect(state.averageLabel, 'Belum ada ulasan');
    });

    test('successful populated summary uses backend values', () {
      final state = ProfileRatingSummaryState.fromAsyncValue(
        AsyncData(
          Result.success(
            const RatingSummary(
              totalRatings: 12,
              averageRating: 4.75,
              oneStarCount: 0,
              twoStarCount: 0,
              threeStarCount: 1,
              fourStarCount: 3,
              fiveStarCount: 8,
            ),
          ),
        ),
      );

      expect(state.isPopulated, isTrue);
      expect(state.averageText, '4.8');
      expect(state.totalText, '12');
      expect(state.averageLabel, 'Rating (12)');
    });

    test('error summary stays unavailable and does not fake a zero', () {
      final state = ProfileRatingSummaryState.fromAsyncValue(
        AsyncError<Result<RatingSummary>>(
          Exception('timeout'),
          StackTrace.empty,
        ),
      );

      expect(state.isUnavailable, isTrue);
      expect(state.averageText, '\u2014');
      expect(state.totalText, '\u2014');
      expect(state.averageLabel, 'Rating unavailable');
    });

    test('only one currentUserProfileProvider declaration remains', () {
      final coreSource = _readSource(
        'lib/domains/user/profile/presentation/providers/profile_core_provider.dart',
      );
      final presentationSource = _readSource(
        'lib/domains/user/profile/presentation/providers/profile_providers.dart',
      );
      final declaration = RegExp(r'final\s+currentUserProfileProvider\s*=');
      final count =
          declaration.allMatches(coreSource).length +
          declaration.allMatches(presentationSource).length;

      expect(count, 1);
    });

    test(
      'currentUserProfileProvider follows the active principal and clears on logout',
      () async {
        final controller = _MutableAuthController(
          AuthState.authenticated(
            _authUser(id: 'user-a', username: 'alice'),
            emailVerified: true,
          ),
        );
        final repository = _FakeProfileRepository((userId) {
          if (userId == 'user-a') return _profile('user-a');
          if (userId == 'user-b') return _profile('user-b');
          return null;
        });
        final container = ProviderContainer(
          overrides: [
            authControllerProvider.overrideWith(() => controller),
            profileRepositoryProvider.overrideWithValue(repository),
          ],
        );
        addTearDown(container.dispose);

        final first = await container.read(currentUserProfileProvider.future);
        expect(first?.userId, 'user-a');

        controller.setState(
          AuthState.authenticated(
            _authUser(id: 'user-b', username: 'bob'),
            emailVerified: true,
          ),
        );
        final second = await container.read(currentUserProfileProvider.future);
        expect(second?.userId, 'user-b');

        controller.setState(const AuthState.unauthenticated());
        final loggedOut = await container.read(
          currentUserProfileProvider.future,
        );
        expect(loggedOut, isNull);
      },
    );

    test(
      'profile screen uses viewed-user authority, not current-user fallback',
      () {
        final source = _readSource(
          'lib/domains/user/profile/presentation/screens/profile_screen.dart',
        );

        expect(
          source.contains('profileViewDataProvider(actualUserId)'),
          isTrue,
        );
        expect(source.contains('currentUserProfileProvider'), isFalse);
      },
    );

    test('seller identity gate in About tab is keyed to hasSellerProfile', () {
      final source = _readSource(
        'lib/domains/user/profile/presentation/screens/profile_screen/profile_about_tab.dart',
      );

      expect(source.contains('sellerState?.hasSellerProfile ?? false'), isTrue);
      expect(source.contains('sellerState?.isSeller ?? false'), isFalse);
    });
  });
}
