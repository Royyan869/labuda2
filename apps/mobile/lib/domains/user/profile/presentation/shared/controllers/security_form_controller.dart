import 'package:flutter/foundation.dart';
import '../models/profile_form_state.dart';

/// Controller for security form state
///
/// Manages screen-level UI state for security forms.
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
///
/// ❌ FORBIDDEN in this controller:
/// - Field-specific validation state (e.g., `passwordValidation`, `currentPasswordError`)
/// - Field focus state (e.g., `isCurrentPasswordFocused`)
/// - Async verification status (e.g., `isCheckingPassword`)
/// - Toggle settings state (e.g., `biometricEnabled`, `twoFactorEnabled`) - these stay in screen
///
/// Example usage:
/// ```dart
/// final controller = SecurityFormController();
///
/// // Update loading state
/// controller.setLoading(true);
///
/// // Show error message
/// controller.showError('Current password is incorrect');
///
/// // Show success message
/// controller.showSuccess('Password updated successfully');
/// ```
///
/// @see REFACTOR_UI.md section 13 for security form guidelines
class SecurityFormController extends ChangeNotifier {
  ProfileFormState _state = const ProfileFormState(
    formType: ProfileFormType.settings,
  );

  /// Current state
  ProfileFormState get state => _state;

  /// Loading state
  bool get isLoading => _state.isLoading;

  /// Form validity (true/false only, no field details)
  bool get isFormValid => _state.isFormValid;

  /// Error/Success messages (screen-level only)
  String? get errorMessage => _state.errorMessage;
  String? get successMessage => _state.successMessage;

  /// Check if form has error
  bool get hasError => _state.hasError;

  /// Check if form has success
  bool get hasSuccess => _state.hasSuccess;

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

  /// Clear all messages
  void clearMessages() {
    if (_state.errorMessage != null || _state.successMessage != null) {
      _state = _state.copyWith(clearError: true, clearSuccess: true);
      notifyListeners();
    }
  }

  /// Reset form to initial state
  void reset() {
    _state = const ProfileFormState(formType: ProfileFormType.settings);
    notifyListeners();
  }
}
