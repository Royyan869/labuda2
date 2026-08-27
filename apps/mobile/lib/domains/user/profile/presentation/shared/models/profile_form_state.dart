import 'package:flutter/foundation.dart';

/// Profile form type enum
enum ProfileFormType { personalInformation, editProfile, settings }

/// Profile form state model
///
/// Contains screen-level UI state for profile forms.
/// Field-level validation MUST be handled at widget/Form level.
///
/// ## Architecture Lock
///
/// ✅ ALLOWED in this state:
/// - `isLoading` - screen-level loading state
/// - `errorMessage` / `successMessage` - screen-level messages
/// - `isFormValid` - boolean aggregation ONLY (true/false, no field details)
///
/// ❌ FORBIDDEN in this state:
/// - Field-specific validation state (e.g., `emailValidation`, `phoneError`)
/// - Field focus state (e.g., `isEmailFocused`, `isPhoneFocused`)
/// - Async verification status (e.g., `isCheckingPhone`)
/// - Individual field error messages
///
/// @see REFACTOR_UI.md section 11 for non-auth form guidelines
@immutable
class ProfileFormState {
  final ProfileFormType formType;
  final bool isLoading;
  final bool isFormValid;
  final String? errorMessage;
  final String? successMessage;
  final DateTime? lastUpdated;

  const ProfileFormState({
    required this.formType,
    this.isLoading = false,
    this.isFormValid = false,
    this.errorMessage,
    this.successMessage,
    this.lastUpdated,
  });

  /// Personal information form state
  const ProfileFormState.personalInformation()
    : formType = ProfileFormType.personalInformation,
      isLoading = false,
      isFormValid = false,
      errorMessage = null,
      successMessage = null,
      lastUpdated = null;

  /// Edit profile form state
  const ProfileFormState.editProfile()
    : formType = ProfileFormType.editProfile,
      isLoading = false,
      isFormValid = false,
      errorMessage = null,
      successMessage = null,
      lastUpdated = null;

  /// Settings form state
  const ProfileFormState.settings()
    : formType = ProfileFormType.settings,
      isLoading = false,
      isFormValid = false,
      errorMessage = null,
      successMessage = null,
      lastUpdated = null;

  /// Check if form has error
  bool get hasError => errorMessage != null;

  /// Check if form has success
  bool get hasSuccess => successMessage != null;

  /// Copy with method for immutable updates
  ProfileFormState copyWith({
    ProfileFormType? formType,
    bool? isLoading,
    bool? isFormValid,
    String? errorMessage,
    String? successMessage,
    DateTime? lastUpdated,
    bool clearError = false,
    bool clearSuccess = false,
  }) {
    return ProfileFormState(
      formType: formType ?? this.formType,
      isLoading: isLoading ?? this.isLoading,
      isFormValid: isFormValid ?? this.isFormValid,
      errorMessage: clearError ? null : (errorMessage ?? this.errorMessage),
      successMessage: clearSuccess
          ? null
          : (successMessage ?? this.successMessage),
      lastUpdated: lastUpdated ?? this.lastUpdated,
    );
  }

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is ProfileFormState &&
          runtimeType == other.runtimeType &&
          formType == other.formType &&
          isLoading == other.isLoading &&
          isFormValid == other.isFormValid &&
          errorMessage == other.errorMessage &&
          successMessage == other.successMessage;

  @override
  int get hashCode =>
      formType.hashCode ^
      isLoading.hashCode ^
      isFormValid.hashCode ^
      errorMessage.hashCode ^
      successMessage.hashCode;
}
