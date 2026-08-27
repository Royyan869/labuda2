import 'package:flutter/foundation.dart';
import '../models/auth_form_state.dart';

/// Controller for authentication form state
///
/// Centralized state management for all auth forms.
/// Use this controller to manage UI state instead of scattered booleans.
///
/// ## 🚨 ARCHITECTURE LOCK - READ BEFORE MODIFYING 🚨
///
/// This controller manages SCREEN-LEVEL UI state ONLY.
/// Field-level validation MUST be handled at widget/Form level.
///
/// ✅ ALLOWED in this controller:
/// - `isLoading` - screen-level loading state
/// - `errorMessage` / `successMessage` - screen-level messages
/// - `isFormValid` - boolean aggregation ONLY (true/false, no field details)
/// - `passwordVisibility` - UI state for password toggle
/// - `rememberMe` / `agreeToTerms` - checkbox states
///
/// ❌ FORBIDDEN in this controller:
/// - Field-specific validation state (e.g., `emailValidation`, `usernameError`)
/// - Field focus state (e.g., `isEmailFocused`, `isPasswordFocused`)
/// - Async validation status (e.g., `isCheckingUsername`, `usernameAvailability`)
/// - Individual field error messages
///
/// ## Why This Separation?
///
/// 1. **Widget-level validation** requires immediate UI feedback
/// 2. **Async validation** needs complex state (checking/valid/invalid/error)
/// 3. **Controller** only needs to know "can I submit?" (true/false)
///
/// If you need field-level state:
/// - Create a separate widget (e.g., `UsernameField`)
/// - Let that widget manage its own state
/// - Callback aggregated boolean to parent (isValid, isAvailable)
///
/// Example usage:
/// ```dart
/// final controller = AuthFormController.signIn();
///
/// // Update loading state
/// controller.setLoading(true);
///
/// // Toggle password visibility
/// controller.togglePasswordVisibility();
///
/// // Update form validity (call this from form field changes)
/// controller.setFormValid(_formKey.currentState?.validate() ?? false);
/// ```
///
/// @see REFACTOR_UI.md section 10.7 for full architecture guidelines
class AuthFormController extends ChangeNotifier {
  AuthFormState _state;

  AuthFormController(AuthFormType formType)
    : _state = AuthFormState(formType: formType);

  /// Create controller for sign in form
  AuthFormController.signIn() : _state = const AuthFormState.signIn();

  /// Create controller for sign up form
  AuthFormController.signUp() : _state = const AuthFormState.signUp();

  /// Create controller for forgot password form
  AuthFormController.forgotPassword()
    : _state = const AuthFormState.forgotPassword();

  /// Current state
  AuthFormState get state => _state;

  /// Form type
  AuthFormType get formType => _state.formType;

  /// Loading state
  bool get isLoading => _state.isLoading;

  /// Form validity (true/false only, no field details)
  bool get isFormValid => _state.isFormValid;

  /// Remember me checkbox state
  bool get rememberMe => _state.rememberMe;

  /// Agree to terms checkbox state (sign up only)
  bool get agreeToTerms => _state.agreeToTerms;

  /// Password visibility
  bool get isPasswordVisible => _state.passwordVisibility.isPasswordVisible;

  bool get isConfirmPasswordVisible =>
      _state.passwordVisibility.isConfirmPasswordVisible;

  /// Error/Success messages (screen-level only)
  String? get errorMessage => _state.errorMessage;
  String? get successMessage => _state.successMessage;

  /// Update loading state
  void setLoading(bool loading) {
    if (_state.isLoading != loading) {
      _state = _state.copyWith(isLoading: loading);
      notifyListeners();
    }
  }

  /// Update form validity
  ///
  /// Call this from form field changes:
  /// ```dart
  /// void _validateForm() {
  ///   controller.setFormValid(_formKey.currentState?.validate() ?? false);
  /// }
  /// ```
  void setFormValid(bool isValid) {
    if (_state.isFormValid != isValid) {
      _state = _state.copyWith(isFormValid: isValid);
      notifyListeners();
    }
  }

  /// Toggle remember me
  void toggleRememberMe() {
    _state = _state.copyWith(rememberMe: !_state.rememberMe);
    notifyListeners();
  }

  /// Set remember me value
  void setRememberMe(bool value) {
    if (_state.rememberMe != value) {
      _state = _state.copyWith(rememberMe: value);
      notifyListeners();
    }
  }

  /// Toggle agree to terms
  void toggleAgreeToTerms() {
    _state = _state.copyWith(agreeToTerms: !_state.agreeToTerms);
    notifyListeners();
  }

  /// Set agree to terms value
  void setAgreeToTerms(bool value) {
    if (_state.agreeToTerms != value) {
      _state = _state.copyWith(agreeToTerms: value);
      notifyListeners();
    }
  }

  /// Toggle password visibility
  void togglePasswordVisibility() {
    final newVisibility = _state.passwordVisibility.copyWith(
      isPasswordVisible: !_state.passwordVisibility.isPasswordVisible,
    );
    _state = _state.copyWith(passwordVisibility: newVisibility);
    notifyListeners();
  }

  /// Set password visibility
  void setPasswordVisible(bool visible) {
    if (_state.passwordVisibility.isPasswordVisible != visible) {
      final newVisibility = _state.passwordVisibility.copyWith(
        isPasswordVisible: visible,
      );
      _state = _state.copyWith(passwordVisibility: newVisibility);
      notifyListeners();
    }
  }

  /// Toggle confirm password visibility
  void toggleConfirmPasswordVisibility() {
    final newVisibility = _state.passwordVisibility.copyWith(
      isConfirmPasswordVisible:
          !_state.passwordVisibility.isConfirmPasswordVisible,
    );
    _state = _state.copyWith(passwordVisibility: newVisibility);
    notifyListeners();
  }

  /// Set confirm password visibility
  void setConfirmPasswordVisible(bool visible) {
    if (_state.passwordVisibility.isConfirmPasswordVisible != visible) {
      final newVisibility = _state.passwordVisibility.copyWith(
        isConfirmPasswordVisible: visible,
      );
      _state = _state.copyWith(passwordVisibility: newVisibility);
      notifyListeners();
    }
  }

  /// Show screen-level error message
  void showError(String message) {
    _state = _state.copyWith(errorMessage: message, clearSuccess: true);
    notifyListeners();
  }

  /// Show screen-level success message
  void showSuccess(String message) {
    _state = _state.copyWith(successMessage: message, clearError: true);
    notifyListeners();
  }

  /// Clear error message
  void clearError() {
    if (_state.errorMessage != null) {
      _state = _state.copyWith(clearError: true);
      notifyListeners();
    }
  }

  /// Clear success message
  void clearSuccess() {
    if (_state.successMessage != null) {
      _state = _state.copyWith(clearSuccess: true);
      notifyListeners();
    }
  }

  /// Reset form to initial state (keeps form type)
  void reset() {
    _state = AuthFormState(formType: _state.formType);
    notifyListeners();
  }
}
