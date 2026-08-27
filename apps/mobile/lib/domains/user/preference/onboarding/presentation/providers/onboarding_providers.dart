import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/preference/onboarding/domain/repositories/onboarding_navigation_repository.dart';
import 'package:labuda/domains/user/preference/onboarding/data/repositories/onboarding_navigation_repository_impl.dart';

// =============================================================================
// DATA LAYER PROVIDERS (Dependency Injection)
// =============================================================================

/// App Entry Navigation Repository Provider
///
/// CLARIFICATION: This is NOT an auth flow or onboarding completion controller.
/// This ONLY provides navigation helpers for splash/welcome screens.
///
/// Auth flow is controlled by AuthController + goRouterProvider redirect logic.
final onboardingNavigationRepositoryProvider =
    Provider<OnboardingNavigationRepository>((ref) {
      final navigationHandler = ref.watch(navigationHandlerProvider);
      return OnboardingNavigationRepositoryImpl(
        navigationHandler: navigationHandler,
      );
    });
