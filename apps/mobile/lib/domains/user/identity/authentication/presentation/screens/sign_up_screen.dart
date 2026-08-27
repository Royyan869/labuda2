import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/shared/helpers/canonical_email_validator.dart';
import 'package:labuda/shared/helpers/canonical_password_policy.dart';
import 'package:labuda/domains/user/identity/authentication/presentation/shared/shared.dart';
import '../widgets/username_field.dart';

/// Sign Up Screen - State-Driven Refactor
///
/// Features:
/// - Uses AuthFormController for state management (no local booleans)
/// - Uses shared widgets (AuthTextField, AuthPasswordField, etc.)
/// - Async validation stays at widget level (UsernameField)
/// - Password strength indicator as inline widget
/// - Clean separation: UI state in controller, field validation in widgets
///
/// ## State Flow
/// 1. User fills form → fields validate at widget level
/// 2. Form validated via Form.validate()
/// 3. Loading state during API call
/// 4. Success → navigate to home
/// 5. Error → show inline error via AuthStateView
class SignUpScreen extends ConsumerStatefulWidget {
  const SignUpScreen({super.key});

  @override
  ConsumerState<SignUpScreen> createState() => _SignUpScreenState();
}

class _SignUpScreenState extends ConsumerState<SignUpScreen>
    with TickerProviderStateMixin {
  final _formKey = GlobalKey<FormState>();
  final _usernameController = TextEditingController();
  final _emailController = TextEditingController();
  final _passwordController = TextEditingController();
  final _confirmPasswordController = TextEditingController();

  late final AuthFormController _controller;
  late AnimationController _animationController;
  late Animation<double> _fadeAnimation;
  late Animation<Offset> _slideAnimation;

  // Username validation state (widget-level, not in controller).
  // Availability (taken/reserved) is backend authority and is surfaced at
  // submission; the form only gates on LOCAL format validity.
  bool _isUsernameValid = false;

  @override
  void initState() {
    super.initState();

    // Initialize controller for sign up form
    _controller = AuthFormController.signUp();

    // Add listener to password controller for strength indicator updates
    _passwordController.addListener(_onPasswordChanged);

    // Setup animations
    _animationController = AnimationController(
      duration: const Duration(milliseconds: 1200),
      vsync: this,
    );

    _fadeAnimation = Tween<double>(begin: 0.0, end: 1.0).animate(
      CurvedAnimation(parent: _animationController, curve: Curves.easeInOut),
    );

    _slideAnimation =
        Tween<Offset>(begin: const Offset(0, 0.1), end: Offset.zero).animate(
          CurvedAnimation(
            parent: _animationController,
            curve: Curves.easeOutCubic,
          ),
        );

    _animationController.forward();
  }

  @override
  void dispose() {
    _passwordController.removeListener(_onPasswordChanged);
    _animationController.dispose();
    _usernameController.dispose();
    _emailController.dispose();
    _passwordController.dispose();
    _confirmPasswordController.dispose();
    _controller.dispose();
    super.dispose();
  }

  /// Password changed callback - updates strength indicator
  void _onPasswordChanged() {
    setState(() {});
  }

  /// Username validation callback from UsernameField (widget-level)
  void _onUsernameValidationChanged(bool isValid, bool isAvailable) {
    setState(() {
      _isUsernameValid = isValid;
    });
  }

  /// Check if form is valid for enabling submit button
  ///
  /// Local validation establishes only username FORMAT validity. Availability
  /// (reserved / taken / final acceptance) is backend authority and is
  /// surfaced when the authenticated exchange responds — so the submit is
  /// enabled on valid format, never gated by a locally-fabricated "available".
  bool get _isFormValid {
    final hasValidUsername = _isUsernameValid;
    final hasEmail = CanonicalEmailValidator.isValid(
      _emailController.text.trim(),
    );
    final hasPassword = CanonicalPasswordPolicy.isValid(
      _passwordController.text.trim(),
    );
    final passwordsMatch =
        _confirmPasswordController.text.trim() ==
        _passwordController.text.trim();
    final agreedToTerms = _controller.agreeToTerms;

    return hasValidUsername &&
        hasEmail &&
        hasPassword &&
        passwordsMatch &&
        agreedToTerms;
  }

  /// Email/Password Sign-Up - Deterministic Flow
  ///
  /// UI hanya memanggil signUpWithEmail() dan menunggu.
  /// Router akan menangani redirect secara otomatis ketika state berubah.
  /// Tidak ada check authState manual, tidak ada navigasi manual.
  Future<void> _handleSignUp() async {
    // Validate form at widget level
    if (!_formKey.currentState!.validate()) return;

    try {
      final controller = ref.read(authControllerProvider.notifier);
      final username = _usernameController.text.trim();

      // Stage 1C recovery: if the Firebase account already exists and the
      // backend rejected the first username (USERNAME_TAKEN / RESERVED /
      // INVALID_FORMAT), retry ONLY the authenticated exchange with the
      // corrected username. Re-calling signUpWithEmail would recreate the
      // account and hit EMAIL_ALREADY_IN_USE.
      if (controller.hasPendingRegistration) {
        await controller.retryRegistrationUsername(username);
      } else {
        await controller.signUpWithEmail(
          email: _emailController.text.trim(),
          password: _passwordController.text.trim(),
          username: username,
        );
      }

      if (mounted) {
        AppSnackBar.showSuccess(
          context,
          'Account created! Verification email sent.',
        );
      }
      // Router akan menangani redirect ketika AuthStateAuthenticated tercapai
    } catch (e) {
      if (mounted) {
        _controller.showError('Registration failed. Please try again.');
      }
    }
    // _controller.setLoading(false) dihapus - UI loading berdasarkan AuthState
  }

  /// Google Sign-Up - Deterministic Flow
  ///
  /// UI hanya memanggil signUpWithGoogle() dan menunggu.
  /// Router akan menangani redirect secara otomatis ketika state berubah.
  /// Tidak ada check authState manual, tidak ada navigasi manual.
  Future<void> _handleGoogleSignUp() async {
    try {
      await ref.read(authControllerProvider.notifier).signUpWithGoogle();

      if (mounted) {
        AppSnackBar.showSuccess(context, 'Signed up with Google!');
      }
      // Router akan menangani redirect ketika AuthStateAuthenticated tercapai
    } catch (e) {
      if (mounted) {
        _controller.showError('Google sign up failed. Please try again.');
      }
    }
    // _controller.setLoading(false) dihapus - UI loading berdasarkan AuthState
  }

  void _navigateToSignIn() {
    ref.navigation.navigateToSignIn();
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    // 🔒 DETERMINISTIC: Watch auth state untuk loading, bukan local controller
    // Ini memastikan UI sinkron dengan actual auth flow
    final authState = ref.watch(authControllerProvider);
    final isAuthLoading =
        authState is AuthStateLoading ||
        authState is AuthStateSyncingWithBackend;

    // Surface backend-rejected registration username errors (USERNAME_TAKEN,
    // USERNAME_RESERVED, USERNAME_INVALID_FORMAT, USERNAME_IMMUTABLE) inline on
    // the form so the user can correct the username. AppAuthStatus for these
    // resolves to degraded (no redirect), keeping the user on this screen.
    String? authBackendError;
    if (authState is AuthStateBackendFailure) {
      authBackendError = authState.message;
    }

    return Scaffold(
      body: Container(
        decoration: BoxDecoration(
          gradient: isDark
              ? const LinearGradient(
                  begin: Alignment.topCenter,
                  end: Alignment.bottomCenter,
                  colors: [AppColors.darkGray900, AppColors.darkGray800],
                )
              : const LinearGradient(
                  begin: Alignment.topCenter,
                  end: Alignment.bottomCenter,
                  colors: [AppColors.neutralGray50, AppColors.neutralWhite],
                ),
        ),
        child: SafeArea(
          child: ListenableBuilder(
            listenable: _controller,
            builder: (context, child) {
              return SingleChildScrollView(
                padding: const EdgeInsets.all(24),
                child: Form(
                  key: _formKey,
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      const SizedBox(height: 20),

                      // Header - uses shared widget
                      AuthHeader.animated(
                        title: 'Create Account',
                        subtitle: 'Join our community today',
                        fadeAnimation: _fadeAnimation,
                        slideAnimation: _slideAnimation,
                      ),

                      // Inline banner for backend-rejected registration username
                      // (USERNAME_TAKEN / USERNAME_RESERVED / USERNAME_INVALID_FORMAT /
                      // USERNAME_IMMUTABLE). Keeps the form visible so the user can
                      // correct the username and resubmit.
                      if (authBackendError != null) ...[
                        const SizedBox(height: 16),
                        AuthStateBanner(error: authBackendError),
                      ],

                      // State-driven view for error/loading
                      AuthStateView(
                        isLoading: isAuthLoading || _controller.isLoading,
                        error: _controller.errorMessage,
                        content: _buildForm(isDark),
                      ),

                      const SizedBox(height: 24),

                      // Navigation to sign in
                      _buildNavigation(),
                    ],
                  ),
                ),
              );
            },
          ),
        ),
      ),
    );
  }

  /// Form content - shown when not loading/error
  Widget _buildForm(bool isDark) {
    return Column(
      children: [
        // Username field - has async validation at widget level
        UsernameField(
          controller: _usernameController,
          isDark: isDark,
          onValidationChanged: _onUsernameValidationChanged,
        ),

        const SizedBox(height: 16),

        // Email field
        AuthTextField.email(
          controller: _emailController,
          validator: (value) =>
              CanonicalEmailValidator.validationMessage(value),
        ),

        const SizedBox(height: 16),

        // Password field with strength indicator
        AuthPasswordField(
          controller: _passwordController,
          labelText: 'Password',
          hintText: 'Create a strong password',
          isPasswordVisible: _controller.isPasswordVisible,
          onToggleVisibility: _controller.togglePasswordVisibility,
          strengthIndicator: PasswordStrengthIndicator(
            password: _passwordController.text,
            isDark: isDark,
          ),
          textInputAction: TextInputAction.next,
          validator: (value) {
            if (value == null || value.isEmpty) {
              return 'Password is required';
            }
            // Canonical Labuda password policy (min 8 + upper + lower + digit).
            return CanonicalPasswordPolicy.validationMessage(value);
          },
        ),

        const SizedBox(height: 16),

        // Confirm password field with match indicator
        AuthConfirmPasswordField(
          controller: _confirmPasswordController,
          passwordController: _passwordController,
          isVisible: _controller.isConfirmPasswordVisible,
          onToggleVisibility: _controller.toggleConfirmPasswordVisibility,
          validator: (value) {
            if (value == null || value.isEmpty) {
              return 'Please confirm your password';
            }
            if (value != _passwordController.text) {
              return 'Passwords do not match';
            }
            return null;
          },
        ),

        const SizedBox(height: 24),

        // Terms checkbox
        _buildTermsCheckbox(isDark),

        const SizedBox(height: 32),

        // Sign up button
        AuthButton.primary(
          text: 'Create Account',
          isLoading: _controller.isLoading,
          isEnabled: _isFormValid,
          onPressed: _handleSignUp,
        ),

        const SizedBox(height: 24),

        // Divider
        const AuthDivider(),

        // Google sign up button
        AuthButton.social(
          icon: Icons.account_circle,
          text: 'Sign up with Google',
          onPressed: _handleGoogleSignUp,
        ),
      ],
    );
  }

  /// Terms and conditions checkbox
  Widget _buildTermsCheckbox(bool isDark) {
    return FormField<bool>(
      initialValue: _controller.agreeToTerms,
      validator: (value) {
        if (!_controller.agreeToTerms) {
          return 'You must agree to Terms and Conditions';
        }
        return null;
      },
      builder: (formFieldState) {
        return Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Checkbox(
                  value: _controller.agreeToTerms,
                  onChanged: (value) {
                    _controller.setAgreeToTerms(value ?? false);
                    formFieldState.didChange(value);
                  },
                  activeColor: AppColors.primaryRed,
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(4),
                  ),
                ),
                Expanded(
                  child: Padding(
                    padding: const EdgeInsets.only(top: 12),
                    child: Text.rich(
                      TextSpan(
                        text: 'I agree with ',
                        style: TextStyle(
                          fontSize: 13,
                          color: isDark
                              ? AppColors.neutralGray300
                              : AppColors.neutralGray700,
                        ),
                        children: [
                          TextSpan(
                            text: 'Terms and Conditions',
                            style: TextStyle(
                              color: AppColors.primaryRed,
                              fontWeight: FontWeight.w600,
                              decoration: TextDecoration.underline,
                            ),
                          ),
                        ],
                      ),
                    ),
                  ),
                ),
              ],
            ),
            if (formFieldState.hasError)
              Padding(
                padding: const EdgeInsets.only(left: 12, top: 8),
                child: Text(
                  formFieldState.errorText!,
                  style: TextStyle(color: AppColors.error, fontSize: 12),
                ),
              ),
          ],
        );
      },
    );
  }

  /// Navigation to sign in link
  Widget _buildNavigation() {
    return Row(
      mainAxisAlignment: MainAxisAlignment.center,
      children: [
        Text(
          'Already have an account? ',
          style: Theme.of(context).textTheme.bodyMedium?.copyWith(
            color: Theme.of(context).brightness == Brightness.dark
                ? AppColors.neutralGray400
                : AppColors.neutralGray600,
          ),
        ),
        TextButton(
          onPressed: _navigateToSignIn,
          child: const Text(
            'Sign In',
            style: TextStyle(
              color: AppColors.primaryRed,
              fontWeight: FontWeight.w600,
            ),
          ),
        ),
      ],
    );
  }
}
