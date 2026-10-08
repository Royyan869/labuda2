import 'package:go_router/go_router.dart';
import 'package:labuda/core/src/router/route_paths.dart';
import 'package:labuda/domains/user/profile/profile.dart';
import 'package:labuda/domains/system/notification/notification.dart';
import 'package:labuda/domains/system/report/presentation/screens/my_reports_screen.dart'
    show MyReportsScreen;
import 'package:labuda/domains/user/identity/authentication/presentation/screens/login_sessions_screen.dart'
    show LoginSessionsScreen;
import 'package:labuda/domains/social/follow/presentation/screens/follow_list_screen.dart'
    show FollowListScreen, FollowListType;
import 'package:labuda/domains/user/identity/authentication/presentation/providers/auth_controller.dart';
import 'package:labuda/domains/user/identity/authentication/presentation/providers/auth_state.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'base_module.dart';

/// Profile Module - Profile detail and settings
class ProfileModule extends BaseModule {
  @override
  String get moduleName => 'ProfileModule';

  @override
  List<GoRoute> get routes => [
    // Route for viewing own profile (/profile)
    // Uses current authenticated user's ID
    GoRoute(
      path: RoutePaths.profile,
      name: RouteNames.profile,
      builder: (context, state) {
        // Get current userId from auth state
        final container = ProviderScope.containerOf(context, listen: false);
        final authState = container.read(authControllerProvider);

        if (authState is AuthStateAuthenticated) {
          return ProfileScreen(userId: authState.user.id);
        }

        // Not authenticated - show empty profile or redirect
        return const ProfileScreen(userId: '');
      },
    ),
    // Route for viewing other user's profile (/user/:userId)
    GoRoute(
      path: RoutePaths.userProfile,
      name: RouteNames.userProfile,
      builder: (context, state) {
        final userId = state.pathParameters['userId'] ?? '';
        return ProfileScreen(userId: userId);
      },
    ),
    // Route for editing the current user's own profile. Always resolves the
    // caller from auth state (same pattern as RoutePaths.profile above) —
    // there is no "edit someone else's profile" flow.
    // `extra` optionally carries a UnifiedEditProfileSection to scroll to on
    // open (defaults to personal).
    GoRoute(
      path: RoutePaths.editProfile,
      name: RouteNames.editProfile,
      builder: (context, state) {
        final container = ProviderScope.containerOf(context, listen: false);
        final authState = container.read(authControllerProvider);
        final section = state.extra is UnifiedEditProfileSection
            ? state.extra as UnifiedEditProfileSection
            : UnifiedEditProfileSection.personal;

        if (authState is AuthStateAuthenticated) {
          return UnifiedEditProfileScreen(
            userId: authState.user.id,
            initialSection: section,
          );
        }
        return const ProfileScreen(userId: '');
      },
    ),
    // Route for editing personal information (email/phone verification, DOB).
    GoRoute(
      path: RoutePaths.personalInformation,
      name: RouteNames.personalInformation,
      builder: (context, state) => const PersonalInformationScreen(),
    ),
    GoRoute(
      path: RoutePaths.settings,
      name: RouteNames.settings,
      builder: (context, state) => const SettingsScreen(),
    ),
    GoRoute(
      path: '/settings/notifications',
      name: 'notificationSettings',
      builder: (context, state) => const NotificationSettingsScreen(),
    ),
    GoRoute(
      path: RoutePaths.notifications,
      name: RouteNames.notifications,
      builder: (context, state) {
        // Get current userId from auth state
        final container = ProviderScope.containerOf(context, listen: false);
        final authState = container.read(authControllerProvider);

        if (authState is AuthStateAuthenticated) {
          return NotificationListScreen(userId: authState.user.id);
        }

        // Fallback: show empty or redirect - using empty string will show loading state
        return const NotificationListScreen(userId: '');
      },
    ),
    GoRoute(
      path: RoutePaths.addresses,
      name: RouteNames.addresses,
      builder: (context, state) => const AddressListScreen(),
    ),

    // Follow graph of a user profile (`?type=followers|following`).
    GoRoute(
      path: RoutePaths.followList,
      name: RouteNames.followList,
      builder: (context, state) {
        final userId = state.pathParameters['userId'] ?? '';
        final type = state.uri.queryParameters['type'] == 'following'
            ? FollowListType.following
            : FollowListType.followers;
        final username = state.extra is String ? state.extra as String : null;
        return FollowListScreen(
          userId: userId,
          type: type,
          username: username,
        );
      },
    ),

    // ── Account & safety surfaces ──────────────────────────────────────────
    GoRoute(
      path: RoutePaths.security,
      name: RouteNames.security,
      builder: (context, state) => const SecurityScreen(),
    ),
    GoRoute(
      path: RoutePaths.loginSessions,
      name: RouteNames.loginSessions,
      builder: (context, state) => const LoginSessionsScreen(),
    ),
    GoRoute(
      path: RoutePaths.blockedUsers,
      name: RouteNames.blockedUsers,
      builder: (context, state) => const BlockedUsersScreen(),
    ),
    GoRoute(
      path: RoutePaths.myReports,
      name: RouteNames.myReports,
      builder: (context, state) => const MyReportsScreen(),
    ),
    GoRoute(
      path: RoutePaths.termsOfService,
      name: RouteNames.termsOfService,
      builder: (context, state) => const TermsOfServiceScreen(),
    ),
    GoRoute(
      path: RoutePaths.privacyPolicy,
      name: RouteNames.privacyPolicy,
      builder: (context, state) => const PrivacyPolicyScreen(),
    ),
  ];

  @override
  Future<void> initialize() async {}

  @override
  void registerRoutes(List<GoRoute> mainRoutes) {
    mainRoutes.addAll(routes);
  }

  @override
  void dispose() {}
}
