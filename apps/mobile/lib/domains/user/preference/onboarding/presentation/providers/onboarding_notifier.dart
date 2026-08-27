import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:labuda/domains/user/preference/onboarding/domain/repositories/onboarding_navigation_repository.dart';
import 'onboarding_state.dart';

part 'onboarding_notifier.g.dart';

/// App Entry Notifier (formerly "Onboarding Notifier")
///
/// CLARIFICATION: This does NOT control auth flow or onboarding completion.
/// This notifier ONLY handles splash/welcome screen UI state and local navigation.
///
/// AUTH FLOW OWNERSHIP:
/// - Auth state management: AuthController (authentication module)
/// - Route redirects: goRouterProvider (core/router)
/// - This notifier: ONLY splash/welcome UI state and local navigation actions
///
/// This replaces UseCase classes - logic lives here.
@riverpod
class OnboardingNotifier extends _$OnboardingNotifier {
  OnboardingNavigationRepository? _navigationRepository;

  @override
  OnboardingState build() {
    return const OnboardingInitial();
  }

  /// Set the navigation repository (called by DI)
  void setNavigationRepository(OnboardingNavigationRepository repository) {
    _navigationRepository = repository;
  }

  /// Check authentication state and return navigation decision
  /// Returns true if authenticated, false otherwise
  Future<bool> checkAuthState() async {
    state = const OnboardingLoading();

    try {
      // This will be implemented by checking auth state via injected dependency
      // For now, return default (unauthenticated) - will be overridden by impl
      state = const OnboardingReady(isAuthenticated: false);
      return false;
    } catch (e) {
      state = OnboardingError(message: e.toString());
      return false;
    }
  }

  /// Navigate based on auth state
  Future<void> navigateBasedOnAuth(bool isAuthenticated) async {
    final repository = _navigationRepository;
    if (repository == null) {
      state = const OnboardingError(
        message: 'Navigation repository not initialized',
      );
      return;
    }

    try {
      if (isAuthenticated) {
        await repository.navigateToHome();
      } else {
        await repository.navigateToWelcome();
      }
    } catch (e) {
      state = OnboardingError(message: e.toString());
    }
  }

  /// Navigate to welcome screen
  Future<void> navigateToWelcome() async {
    final repository = _navigationRepository;
    if (repository == null) return;

    try {
      await repository.navigateToWelcome();
    } catch (e) {
      state = OnboardingError(message: e.toString());
    }
  }

  /// Navigate to home screen
  Future<void> navigateToHome() async {
    final repository = _navigationRepository;
    if (repository == null) return;

    try {
      await repository.navigateToHome();
    } catch (e) {
      state = OnboardingError(message: e.toString());
    }
  }

  /// Navigate to sign up screen
  Future<void> navigateToSignUp() async {
    final repository = _navigationRepository;
    if (repository == null) return;

    try {
      await repository.navigateToSignUp();
    } catch (e) {
      state = OnboardingError(message: e.toString());
    }
  }

  /// Navigate to sign in screen
  Future<void> navigateToSignIn() async {
    final repository = _navigationRepository;
    if (repository == null) return;

    try {
      await repository.navigateToSignIn();
    } catch (e) {
      state = OnboardingError(message: e.toString());
    }
  }

  /// Navigate to guest home (browse without login)
  Future<void> navigateToGuestHome() async {
    final repository = _navigationRepository;
    if (repository == null) return;

    try {
      await repository.navigateToGuestHome();
    } catch (e) {
      state = OnboardingError(message: e.toString());
    }
  }

  /// Reset state to initial
  void reset() {
    state = const OnboardingInitial();
  }
}
