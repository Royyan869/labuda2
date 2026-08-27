import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/shared/helpers/canonical_email_validator.dart';
import 'package:firebase_auth/firebase_auth.dart';
import '../../data/auth_persistence_service.dart';
import '../shared/shared.dart';

/// Sign In Screen - Golden Sample Refactor
///
/// Uses AuthFormController for UI state + shared widgets for consistency.
/// Screen only handles: controller init, submit logic, layout orchestration.
///
/// ## Architecture
/// - UI state → AuthFormController (no local booleans)
/// - Fields → AuthTextField, AuthPasswordField (shared)
/// - Buttons → AuthButton (shared)
/// - Validation → Widget/Form level (controller only knows valid/invalid)
class SignInScreen extends ConsumerStatefulWidget {
  const SignInScreen({super.key});

  @override
  ConsumerState<SignInScreen> createState() => _SignInScreenState();
}

class _SignInScreenState extends ConsumerState<SignInScreen>
    with TickerProviderStateMixin {
  final _formKey = GlobalKey<FormState>();
  final _emailController = TextEditingController();
  final _passwordController = TextEditingController();

  late AnimationController _animationController;
  late Animation<double> _fadeAnimation;
  late Animation<Offset> _slideAnimation;

  // ✅ UI state via controller (no local booleans)
  late final AuthFormController _controller;

  @override
  void initState() {
    super.initState();
    _controller = AuthFormController.signIn();

    // Add listeners for form validation
    _emailController.addListener(_validateForm);
    _passwordController.addListener(_validateForm);

    _loadRememberMeState();
    _setupAnimations();
  }

  @override
  void dispose() {
    _emailController.removeListener(_validateForm);
    _passwordController.removeListener(_validateForm);
    _animationController.dispose();
    _emailController.dispose();
    _passwordController.dispose();
    _controller.dispose();
    super.dispose();
  }

  /// Validate form and update controller state
  ///
  /// Login does NOT apply the registration password policy: the user is
  /// entering an existing credential, which may legitimately predate the
  /// min-8 policy. The gate is only "canonically valid email" + "password
  /// non-empty"; Firebase is the acceptance authority for existing
  /// credentials. The email gate delegates to the canonical email authority
  /// (Stage 4D), not a weak `contains('@')` check.
  void _validateForm() {
    final hasEmail = CanonicalEmailValidator.isValid(
      _emailController.text.trim(),
    );
    final hasPassword = _passwordController.text.trim().isNotEmpty;
    _controller.setFormValid(hasEmail && hasPassword);
  }

  void _setupAnimations() {
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

  Future<void> _loadRememberMeState() async {
    final rememberMe = await AuthPersistenceService.getRememberMe();
    final lastEmail = await AuthPersistenceService.getLastLoginEmail();

    if (mounted) {
      if (lastEmail != null) _emailController.text = lastEmail;
      if (rememberMe) _controller.setRememberMe(true);
    }
  }

  /// Email/Password Sign-In - Deterministic Flow
  ///
  /// UI hanya memanggil signInWithEmail() dan menunggu.
  /// Router akan menangani redirect secara otomatis ketika state berubah.
  /// Tidak ada check authState manual, tidak ada navigasi manual.
  Future<void> _handleSignIn() async {
    if (!_formKey.currentState!.validate()) return;

    try {
      await ref
          .read(authControllerProvider.notifier)
          .signInWithEmail(
            email: _emailController.text.trim(),
            password: _passwordController.text.trim(),
          );

      // Jangan baca authState di sini - race condition!
      // Router akan menangani redirect ketika AuthStateAuthenticated tercapai
    } on FirebaseAuthException catch (e) {
      _showFirebaseError(e);
    }
    // _controller.setLoading(false) dihapus - UI loading berdasarkan AuthState
  }

  /// Google Sign-In - Deterministic Flow
  ///
  /// UI hanya memanggil signInWithGoogle() dan menunggu.
  /// Router akan menangani redirect secara otomatis ketika state berubah.
  /// Tidak ada check authState manual, tidak ada navigasi manual.
  Future<void> _handleGoogleSignIn() async {
    try {
      await ref.read(authControllerProvider.notifier).signInWithGoogle();

      // Jangan baca authState di sini - race condition!
      // Router akan menangani redirect ketika AuthStateAuthenticated tercapai
    } catch (e) {
      // Error sudah ditangani di repository dan controller
      // Ini hanya fallback untuk unexpected errors
      if (mounted) {
        AppSnackBar.showError(context, 'Terjadi kesalahan. Coba lagi.');
      }
    }
    // _controller.setLoading(false) dihapus - UI loading berdasarkan AuthState
  }

  void _showFirebaseError(FirebaseAuthException e) {
    String message = 'Invalid Email or Password!';
    switch (e.code) {
      case 'user-not-found':
        message = 'Email not registered. Please sign up first.';
        break;
      case 'wrong-password':
        message = 'Wrong password. Please check your password again.';
        break;
      case 'invalid-email':
        message = 'Invalid email format. Example: name@email.com';
        break;
      case 'user-disabled':
        message = 'Your account has been disabled. Please contact support.';
        break;
      case 'too-many-requests':
        message = 'Too many login attempts. Please try again later.';
        break;
      case 'invalid-credential':
        message = 'Invalid Email or Password!';
        break;
    }
    if (mounted) AppSnackBar.showError(context, message);
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
                padding: const EdgeInsets.symmetric(horizontal: 24),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    // Header (shared widget)
                    AuthHeader.animated(
                      title: 'Welcome Back',
                      subtitle: 'Sign in to your LABUDA account',
                      fadeAnimation: _fadeAnimation,
                      slideAnimation: _slideAnimation,
                    ),

                    // Form
                    Form(
                      key: _formKey,
                      child: Column(
                        children: [
                          // Email field (shared widget)
                          AuthTextField.email(
                            controller: _emailController,
                            validator: (value) =>
                                CanonicalEmailValidator.validationMessage(value),
                          ),

                          const SizedBox(height: 16),

                          // Password field (shared widget)
                          AuthPasswordField(
                            controller: _passwordController,
                            isPasswordVisible: _controller.isPasswordVisible,
                            onToggleVisibility:
                                _controller.togglePasswordVisibility,
                            validator: (value) {
                              if (value == null || value.isEmpty) {
                                return 'Password is required';
                              }
                              return null;
                            },
                          ),

                          const SizedBox(height: 12),

                          // Remember me + Forgot password
                          Row(
                            mainAxisAlignment: MainAxisAlignment.spaceBetween,
                            children: [
                              Row(
                                mainAxisSize: MainAxisSize.min,
                                children: [
                                  Checkbox(
                                    value: _controller.rememberMe,
                                    onChanged: (value) {
                                      _controller.setRememberMe(value ?? false);
                                    },
                                    activeColor: AppColors.primaryRed,
                                    materialTapTargetSize:
                                        MaterialTapTargetSize.shrinkWrap,
                                  ),
                                  Text(
                                    'Remember me',
                                    style: Theme.of(context).textTheme.bodySmall
                                        ?.copyWith(
                                          color: isDark
                                              ? AppColors.neutralGray400
                                              : AppColors.neutralGray600,
                                        ),
                                  ),
                                ],
                              ),
                              TextButton(
                                onPressed: () => ref
                                    .read(navigationHandlerProvider)
                                    .navigateToForgotPassword(),
                                child: const Text(
                                  'Forgot password?',
                                  style: TextStyle(
                                    color: AppColors.primaryRed,
                                    fontWeight: FontWeight.w500,
                                  ),
                                ),
                              ),
                            ],
                          ),
                        ],
                      ),
                    ),

                    const SizedBox(height: 32),

                    // Sign in button (shared widget)
                    AuthButton.primary(
                      text: 'Sign In',
                      isLoading: isAuthLoading || _controller.isLoading,
                      isEnabled: _controller.isFormValid,
                      onPressed: _handleSignIn,
                    ),

                    const SizedBox(height: 24),

                    // Divider (shared widget)
                    const AuthDivider(text: 'or'),

                    const SizedBox(height: 24),

                    // Google button (shared widget)
                    AuthButton.social(
                      icon: Icons.account_circle,
                      text: 'Sign in with Google',
                      isEnabled: !isAuthLoading,
                      onPressed: _handleGoogleSignIn,
                    ),

                    const SizedBox(height: 48),

                    // Navigation to sign up
                    Row(
                      mainAxisAlignment: MainAxisAlignment.center,
                      children: [
                        Text(
                          "Don't have an account? ",
                          style: Theme.of(context).textTheme.bodyMedium
                              ?.copyWith(
                                color: isDark
                                    ? AppColors.neutralGray400
                                    : AppColors.neutralGray600,
                              ),
                        ),
                        TextButton(
                          onPressed: () => ref
                              .read(navigationHandlerProvider)
                              .navigateToSignUp(),
                          child: const Text(
                            'Sign Up',
                            style: TextStyle(
                              color: AppColors.primaryRed,
                              fontWeight: FontWeight.w600,
                            ),
                          ),
                        ),
                      ],
                    ),

                    const SizedBox(height: 24),
                  ],
                ),
              );
            },
          ),
        ),
      ),
    );
  }
}
