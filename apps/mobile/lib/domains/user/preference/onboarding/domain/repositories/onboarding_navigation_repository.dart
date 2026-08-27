/// Onboarding Navigation Repository Interface
///
/// Abstracts navigation decisions for onboarding flow.
/// Domain layer - bebas dari Flutter/Firebase implementation.
abstract class OnboardingNavigationRepository {
  /// Navigate to home screen (for authenticated users)
  Future<void> navigateToHome();

  /// Navigate to welcome screen (for unauthenticated users)
  Future<void> navigateToWelcome();

  /// Navigate to sign up screen
  Future<void> navigateToSignUp();

  /// Navigate to sign in screen
  Future<void> navigateToSignIn();

  /// Navigate to guest home (browse without login)
  Future<void> navigateToGuestHome();
}
