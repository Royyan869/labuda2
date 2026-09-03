import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/features/home/presentation/handlers/main_screen_navigation_handler.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';

class _FakeAuthController extends AuthController {
  @override
  AuthState build() {
    final now = DateTime.parse('2026-06-03T00:00:00.000Z');
    final user = AuthUser(
      id: 'user-1',
      createdAt: now,
      updatedAt: now,
      email: 'yayan@example.com',
      username: 'yayan',
      avatarUrl: 'https://example.com/avatar.png',
      bio: null,
      isEmailVerified: true,
      accountStatus: AccountStatus.active,
      hasSellerProfile: false,
      sellerSubscriptionStatus: 'none',
      hasMarketAuthority: false,
      roles: const [UserRole.user],
      provider: AuthProvider.email,
      lifecycle: ContentLifecycle.active,
    );

    return AuthState.authenticated(user, emailVerified: true);
  }
}

class _RecordingAppRouter implements NavigationHandler {
  int profileCalls = 0;
  int userProfileCalls = 0;

  @override
  void navigateToProfile() {
    profileCalls += 1;
  }

  @override
  void navigateToUserProfile(String userId) {
    userProfileCalls += 1;
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

void main() {
  testWidgets('drawer profile action uses own-profile authority', (
    tester,
  ) async {
    final router = _RecordingAppRouter();
    var invoked = false;

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          authControllerProvider.overrideWith(_FakeAuthController.new),
        ],
        child: MaterialApp(
          home: Consumer(
            builder: (context, ref, _) {
              if (!invoked) {
                invoked = true;
                WidgetsBinding.instance.addPostFrameCallback((_) {
                  final handler = MainScreenNavigationHandler(
                    ref: ref,
                    context: context,
                    appRouter: router,
                  );
                  handler.handleProfile();
                });
              }
              return const SizedBox.shrink();
            },
          ),
        ),
      ),
    );

    await tester.pump();
    await tester.pumpAndSettle();

    expect(router.profileCalls, 1);
    expect(router.userProfileCalls, 0);
  });
}
