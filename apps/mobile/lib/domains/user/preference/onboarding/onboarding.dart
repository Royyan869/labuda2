/// App Entry Module Public API
///
/// CLARIFICATION: This is NOT an "onboarding domain" or completion flow.
/// This module only provides UI screens for app entry points:
/// - Splash Screen (initial loading/brand display)
/// - Welcome Screen (login/signup entry point)
///
/// AUTH FLOW OWNERSHIP:
/// - Auth state: AuthController (authentication module)
/// - Route decisions: goRouterProvider (core/router)
/// - This module: ONLY provides splash/welcome UI widgets
///
/// Mengikuti clean architecture guidelines.
library;

// Screens
export 'presentation/screens/splash_screen.dart';
export 'presentation/screens/welcome_screen.dart';

// Providers (for DI if needed)
export 'presentation/providers/onboarding_providers.dart';
