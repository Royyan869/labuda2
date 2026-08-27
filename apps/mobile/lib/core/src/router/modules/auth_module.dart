import 'package:go_router/go_router.dart';

import 'package:labuda/domains/user/identity/authentication/authentication.dart';
import 'package:labuda/core/src/router/route_paths.dart';
import 'base_module.dart';

/// Authentication Module
///
/// Mengelola semua routes yang berkaitan dengan authentication:
/// - Sign In Page
/// - Sign Up Page
/// - Forgot Password Page
///
/// Module ini juga bertanggung jawab untuk register
/// authentication-related services ke dependency injection container.
class AuthModule extends BaseModule {
  @override
  List<GoRoute> get routes => [
    // Sign In Route
    GoRoute(
      path: RoutePaths.signIn,
      name: RouteNames.signIn,
      builder: (context, state) => const SignInScreen(),
    ),

    // Sign Up Route
    GoRoute(
      path: RoutePaths.signUp,
      name: RouteNames.signUp,
      builder: (context, state) => const SignUpScreen(),
    ),

    // Forgot Password Route
    GoRoute(
      path: RoutePaths.forgotPassword,
      name: RouteNames.forgotPassword,
      builder: (context, state) => const ForgotPasswordScreen(),
    ),

    // Complete Profile Route - Required for new Google users
    GoRoute(
      path: RoutePaths.completeProfile,
      name: RouteNames.completeProfile,
      builder: (context, state) => const CompleteProfileScreen(),
    ),
  ];

  @override
  Future<void> initialize() async {
    // Register Authentication Service
    // Note: FirebaseAuthenticationService akan diregister di main.dart
    // karena memerlukan Firebase initialization terlebih dahulu

    // Register authentication-related providers atau services lain
    // yang specific untuk auth module bisa ditambahkan disini

    // Contoh:
    // sl.registerLazySingleton<IAuthValidationService>(
    //   () => AuthValidationService(),
    // );

    // sl.registerLazySingleton<IAuthNotificationService>(
    //   () => AuthNotificationService(),
    // );
  }

  @override
  void registerRoutes(List<GoRoute> mainRoutes) {
    mainRoutes.addAll(routes);
  }

  @override
  void dispose() {
    // Cleanup authentication module resources
    // Unregister services if needed
  }

  @override
  String get moduleName => 'AuthModule';
}
