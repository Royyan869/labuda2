/// App Entry State (formerly "Onboarding State")
///
/// CLARIFICATION: This is NOT an onboarding completion state.
/// This state is used ONLY by Splash/Welcome screens to manage
/// their own local UI state (loading, ready, error).
///
/// AUTH FLOW is controlled by AuthController, NOT by this state.
/// - Profile completion: AuthStateRequiresProfileCompletion
/// - Authenticated: AuthStateAuthenticated
///
/// This state is purely for splash/welcome UI transitions.
/// Free from Firebase/Flutter dependencies.
sealed class OnboardingState {
  const OnboardingState();
}

/// Initial state before any action
class OnboardingInitial extends OnboardingState {
  const OnboardingInitial();
}

/// Loading state (e.g., checking auth, loading resources)
class OnboardingLoading extends OnboardingState {
  const OnboardingLoading();
}

/// Ready to navigate (auth check complete)
class OnboardingReady extends OnboardingState {
  const OnboardingReady({required this.isAuthenticated});

  final bool isAuthenticated;
}

/// Error state
class OnboardingError extends OnboardingState {
  const OnboardingError({required this.message});

  final String message;
}
