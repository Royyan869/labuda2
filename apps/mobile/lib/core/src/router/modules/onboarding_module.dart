import 'package:go_router/go_router.dart';
import 'package:labuda/domains/user/preference/onboarding/onboarding.dart';
import 'package:labuda/domains/user/identity/authentication/presentation/screens/account_restricted_screen.dart';
import 'package:labuda/core/src/router/route_paths.dart';
import 'base_module.dart';

/// App Entry Module untuk splash dan welcome screens
///
/// CLARIFICATION: "Onboarding" here is a legacy name - this module does NOT
/// implement an onboarding domain or completion flow. It only registers routes
/// for app entry point screens:
/// - Splash Screen (loading/initialization)
/// - Welcome Screen (login/signup entry point)
///
/// AUTH FLOW OWNERSHIP:
/// - Auth state & session: AuthController (authentication module)
/// - Route decisions: goRouterProvider redirect logic (core/router)
/// - This module: ONLY provides route registration for entry UI screens
///
/// Actual auth flow is controlled by AuthController state machine which
/// determines whether user needs email verification, profile completion,
/// or can proceed to home.
class OnboardingModule extends BaseModule {
  @override
  List<GoRoute> get routes => [
    // Splash Screen Route
    GoRoute(
      path: RoutePaths.splash,
      name: RouteNames.splash,
      builder: (context, state) => const SplashScreen(),
    ),

    // Welcome Screen Route
    GoRoute(
      path: RoutePaths.welcome,
      name: RouteNames.welcome,
      builder: (context, state) => const WelcomeScreen(),
    ),

    // Account Restricted Screen - ID1D: shown for suspended/banned accounts
    GoRoute(
      path: RoutePaths.accountRestricted,
      name: RouteNames.accountRestricted,
      builder: (context, state) => const AccountRestrictedScreen(),
    ),
  ];

  @override
  Future<void> initialize() async {
    // Register onboarding-related services if needed
    // Untuk saat ini tidak ada services khusus yang diperlukan
  }

  @override
  void registerRoutes(List<GoRoute> mainRoutes) {
    mainRoutes.addAll(routes);
  }

  @override
  void dispose() {
    // Cleanup onboarding module resources
  }

  @override
  String get moduleName => 'OnboardingModule';
}
