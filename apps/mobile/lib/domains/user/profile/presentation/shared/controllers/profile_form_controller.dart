import 'package:flutter/foundation.dart';
import '../models/profile_form_state.dart';

/// Controller for profile form state
///
/// Centralized state management for profile forms.
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
/// - Field-specific validation state (e.g., `emailValidation`, `phoneError`)
/// - Field focus state (e.g., `isEmailFocused`, `isPhoneFocused`)
/// - Async verification status (e.g., `isCheckingPhone`)
/// - Individual field error messages
///
/// ## Why This Separation?
///
/// 1. **Widget-level validation** requires immediate UI feedback
/// 2. **Async verification** needs complex state (checking/verified/error)
/// 3. **Controller** only needs to know "can I submit?" (true/false)
///
/// If you need field-level state:
/// - Create a separate widget (e.g., `PhoneVerificationField`)
/// - Let that widget manage its own state
/// - Callback aggregated boolean to parent (isVerified, isValid)
///
/// Example usage:
/// ```dart
/// final controller = ProfileFormController.personalInformation();
///
/// // Update loading state
/// controller.setLoading(true);
///
/// // Update form validity
/// controller.setFormValid(_formKey.currentState?.validate() ?? false);
///
/// // Show error message
/// controller.showError('Failed to save');
/// ```
///
/// @see REFACTOR_UI.md section 11 for non-auth form guidelines
class ProfileFormController extends ChangeNotifier {
  ProfileFormState _state;

  ProfileFormController(ProfileFormType formType)
    : _state = ProfileFormState(formType: formType);

  /// Create controller for personal information form
  ProfileFormController.personalInformation()
    : _state = const ProfileFormState.personalInformation();

  /// Create controller for edit profile form
  ProfileFormController.editProfile()
    : _state = const ProfileFormState.editProfile();

  /// Create controller for settings form
  ProfileFormController.settings() : _state = const ProfileFormState.settings();

  /// Current state
  ProfileFormState get state => _state;

  /// Form type
  ProfileFormType get formType => _state.formType;

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
      _state = _state.copyWith(
        isFormValid: isValid,
        lastUpdated: DateTime.now(),
      );
      notifyListeners();
    }
  }

  /// Show screen-level error message
  void showError(String message) {
    _state = _state.copyWith(
      errorMessage: message,
      clearSuccess: true,
      lastUpdated: DateTime.now(),
    );
    notifyListeners();
  }

  /// Show screen-level success message
  void showSuccess(String message) {
    _state = _state.copyWith(
      successMessage: message,
      clearError: true,
      lastUpdated: DateTime.now(),
    );
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

  /// Reset form to initial state (keeps form type)
  void reset() {
    _state = ProfileFormState(formType: _state.formType);
    notifyListeners();
  }
}
