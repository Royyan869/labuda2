/// Shared Authentication UI Components
///
/// This library provides reusable UI components and state management
/// for authentication screens. Use these components to maintain consistency
/// and reduce code duplication.
///
/// ## Usage
///
/// ```dart
/// import 'package:labuda/domains/user/identity/authentication/presentation/shared/shared.dart';
///
/// // In your auth screen
/// final controller = AuthFormController.signIn();
///
/// // Use shared widgets
/// AuthHeader(
///   title: 'Welcome Back',
///   subtitle: 'Sign in to your account',
/// )
///
/// AuthTextField.email(
///   controller: _emailController,
///   validator: validateEmail,
/// )
///
/// AuthPasswordField(
///   controller: _passwordController,
///   isPasswordVisible: controller.isPasswordVisible,
///   onToggleVisibility: controller.togglePasswordVisibility,
/// )
///
/// AuthButton.primary(
///   text: 'Sign In',
///   isLoading: controller.isLoading,
///   isEnabled: controller.isFormValid,
///   onPressed: _handleSignIn,
/// )
/// ```
///
/// ## State Management
///
/// Use [AuthFormController] instead of scattered boolean state variables:
///
/// - `isLoading` -> `controller.isLoading`
/// - `_obscurePassword` -> `controller.isPasswordVisible`
/// - `_isFormValid` -> `controller.isFormValid`
/// - `setState(() => _isLoading = true)` -> `controller.setLoading(true)`
///
/// ## Available Components
///
/// ### Models
/// - AuthFormState - Unified form state model
/// - AuthFormType - Form type enum (signIn, signUp, forgotPassword)
/// - FormFieldValidation - Field validation status
/// - PasswordVisibility - Password visibility state
///
/// ### Controllers
/// - AuthFormController - Centralized state management
///
/// ### Widgets
/// - AuthTextField - Consistent text input field
/// - AuthPasswordField - Password field with visibility toggle
/// - AuthConfirmPasswordField - Confirm password with match indicator
/// - AuthButton - Primary/secondary/social buttons
/// - AuthDivider - "or" divider component
/// - AuthHeader - Screen header with logo and animations
/// - AuthStateView - Conditional state renderer
/// - AuthStateBanner - Top banner for messages

library;

// Models
export 'models/auth_form_state.dart';

// Controllers
export 'controllers/auth_form_controller.dart';

// Widgets
export 'widgets/auth_text_field.dart';
export 'widgets/auth_password_field.dart';
export 'widgets/auth_button.dart';
export 'widgets/auth_divider.dart';
export 'widgets/auth_header.dart';
export 'widgets/auth_state_view.dart';
