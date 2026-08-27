import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/identity/authentication/presentation/shared/shared.dart';
import 'package:labuda/shared/helpers/canonical_email_validator.dart';

/// Forgot Password Screen - State-Driven Refactor
///
/// Features:
/// - Uses AuthFormController for state management (no local booleans)
/// - Uses AuthStateView for conditional rendering (no if/else blocks)
/// - Uses shared widgets (AuthTextField, AuthButton, AuthHeader)
/// - Clean async state handling with controller
///
/// ## State Flow
/// 1. Initial: Show email input form
/// 2. Loading: Show loading spinner during API call
/// 3. Success: Show success view with resend option
/// 4. Error: Show error message inline
class ForgotPasswordScreen extends ConsumerStatefulWidget {
  const ForgotPasswordScreen({super.key});

  @override
  ConsumerState<ForgotPasswordScreen> createState() =>
      _ForgotPasswordScreenState();
}

class _ForgotPasswordScreenState extends ConsumerState<ForgotPasswordScreen>
    with TickerProviderStateMixin {
  final _formKey = GlobalKey<FormState>();
  final _emailController = TextEditingController();

  late final AuthFormController _controller;
  late AnimationController _animationController;
  late Animation<double> _fadeAnimation;
  late Animation<Offset> _slideAnimation;

  @override
  void initState() {
    super.initState();

    // Initialize controller for forgot password form
    _controller = AuthFormController.forgotPassword();

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
    _animationController.dispose();
    _emailController.dispose();
    _controller.dispose();
    super.dispose();
  }

  Future<void> _handleForgotPassword() async {
    // Validate form at widget level
    if (!_formKey.currentState!.validate()) return;

    // Update loading state via controller
    _controller.setLoading(true);

    try {
      final success = await ref
          .read(authControllerProvider.notifier)
          .resetPassword(email: _emailController.text.trim());

      if (mounted) {
        if (success) {
          // Show success - this switches AuthStateView to success state
          _controller.showSuccess(
            'Password reset email has been sent!\nPlease check your inbox and spam folder.',
          );
        } else {
          _controller.showError('Failed to send password reset email.');
        }
      }
    } catch (e) {
      if (mounted) {
        _controller.showError('An error occurred. Please try again.');
      }
    } finally {
      if (mounted) {
        _controller.setLoading(false);
      }
    }
  }

  void _handleResend() {
    // Clear success to go back to form
    _controller.clearSuccess();
  }

  void _navigateToSignIn() {
    ref.navigation.navigateToSignIn();
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

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
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    const SizedBox(height: 20),

                    // Header - uses shared widget
                    AuthHeader.animated(
                      title: _controller.state.hasSuccess
                          ? 'Email Sent!'
                          : 'Forgot Password?',
                      subtitle: _controller.state.hasSuccess
                          ? 'Click the link in the email to reset your password.'
                          : 'Enter your email to receive a password reset link',
                      fadeAnimation: _fadeAnimation,
                      slideAnimation: _slideAnimation,
                      showLogo: true,
                    ),

                    // State-driven view - replaces if/else blocks
                    AuthStateView(
                      isLoading: _controller.isLoading,
                      error: _controller.errorMessage,
                      success: _controller.successMessage,
                      onErrorDismiss: _controller.clearError,
                      content: _buildForm(),
                      successWidget: _buildSuccessView(),
                    ),

                    const SizedBox(height: 24),

                    // Back to sign in link
                    _buildNavigation(),
                  ],
                ),
              );
            },
          ),
        ),
      ),
    );
  }

  /// Form content - shown when not loading/success/error
  Widget _buildForm() {
    return Form(
      key: _formKey,
      child: Column(
        children: [
          // Email field - uses shared widget
          AuthTextField.email(
            controller: _emailController,
            validator: (value) =>
                CanonicalEmailValidator.validationMessage(value),
          ),

          const SizedBox(height: 32),

          // Submit button - uses shared widget
          AuthButton.primary(
            text: 'Send Reset Email',
            isLoading: _controller.isLoading,
            onPressed: _handleForgotPassword,
          ),
        ],
      ),
    );
  }

  /// Success view - shown after email sent
  Widget _buildSuccessView() {
    return Column(
      children: [
        Icon(Icons.mark_email_read, size: 80, color: AppColors.success),
        const SizedBox(height: 32),
        AuthButton.secondary(text: 'Resend Email', onPressed: _handleResend),
      ],
    );
  }

  /// Navigation link - always shown
  Widget _buildNavigation() {
    return Row(
      mainAxisAlignment: MainAxisAlignment.center,
      children: [
        Text(
          'Back to ',
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
              color: AppColors.primaryBlue,
              fontWeight: FontWeight.w600,
            ),
          ),
        ),
      ],
    );
  }
}
