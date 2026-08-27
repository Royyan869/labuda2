import 'package:labuda/core/navigation/navigation_handler.dart';
import 'package:labuda/domains/user/preference/onboarding/domain/repositories/onboarding_navigation_repository.dart';

/// Implementation of OnboardingNavigationRepository
///
/// Bridges domain interface with core NavigationHandler.
/// This is the data layer implementation.
class OnboardingNavigationRepositoryImpl
    implements OnboardingNavigationRepository {
  const OnboardingNavigationRepositoryImpl({
    required NavigationHandler navigationHandler,
  }) : _navigationHandler = navigationHandler;

  final NavigationHandler _navigationHandler;

  @override
  Future<void> navigateToHome() async {
    _navigationHandler.navigateToHome();
  }

  @override
  Future<void> navigateToWelcome() async {
    _navigationHandler.navigateToWelcome();
  }

  @override
  Future<void> navigateToSignUp() async {
    _navigationHandler.navigateToSignUp();
  }

  @override
  Future<void> navigateToSignIn() async {
    _navigationHandler.navigateToSignIn();
  }

  @override
  Future<void> navigateToGuestHome() async {
    // For guest browsing, we navigate to home without authentication
    _navigationHandler.navigateToHome();
  }
}
