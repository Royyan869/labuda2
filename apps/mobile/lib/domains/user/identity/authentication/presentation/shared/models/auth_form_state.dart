/// Type of authentication form
enum AuthFormType { signIn, signUp, forgotPassword }

/// Validation status for form fields
enum FieldValidationStatus { unknown, invalid, valid, checking }

/// Form field validation states
class FormFieldValidation {
  final FieldValidationStatus status;
  final String? errorMessage;

  const FormFieldValidation({
    this.status = FieldValidationStatus.unknown,
    this.errorMessage,
  });

  const FormFieldValidation.unknown()
    : status = FieldValidationStatus.unknown,
      errorMessage = null;

  const FormFieldValidation.valid()
    : status = FieldValidationStatus.valid,
      errorMessage = null;

  const FormFieldValidation.invalid(this.errorMessage)
    : status = FieldValidationStatus.invalid;

  const FormFieldValidation.checking()
    : status = FieldValidationStatus.checking,
      errorMessage = null;

  bool get isUnknown => status == FieldValidationStatus.unknown;
  bool get isValid => status == FieldValidationStatus.valid;
  bool get isInvalid => status == FieldValidationStatus.invalid;
  bool get isChecking => status == FieldValidationStatus.checking;

  FormFieldValidation copyWith({
    FieldValidationStatus? status,
    String? errorMessage,
  }) {
    return FormFieldValidation(
      status: status ?? this.status,
      errorMessage: errorMessage ?? this.errorMessage,
    );
  }
}

/// Password visibility state
class PasswordVisibility {
  final bool isPasswordVisible;
  final bool isConfirmPasswordVisible;

  const PasswordVisibility({
    this.isPasswordVisible = false,
    this.isConfirmPasswordVisible = false,
  });

  PasswordVisibility copyWith({
    bool? isPasswordVisible,
    bool? isConfirmPasswordVisible,
  }) {
    return PasswordVisibility(
      isPasswordVisible: isPasswordVisible ?? this.isPasswordVisible,
      isConfirmPasswordVisible:
          isConfirmPasswordVisible ?? this.isConfirmPasswordVisible,
    );
  }
}

/// Unified authentication form state
///
/// Replaces scattered boolean state variables across screens.
/// All UI state changes should go through [AuthFormController].
///
/// ## Architecture Note
///
/// Field-level validation is intentionally handled at widget/Form level.
/// AuthFormState only tracks screen-level UI state.
///
/// Mapping:
/// - Widget (AuthTextField)  → Field-level validation
/// - Form (Flutter Form)     → Error display
/// - Controller              → Form validity (true/false only)
/// - Screen                  → Submit logic
///
/// Controller does NOT know "which field is wrong and why".
/// Controller only knows "is the form valid or not".
class AuthFormState {
  final AuthFormType formType;
  final bool isLoading;
  final bool isFormValid;
  final bool rememberMe;
  final bool agreeToTerms;
  final PasswordVisibility passwordVisibility;
  final String? errorMessage;
  final String? successMessage;

  const AuthFormState({
    required this.formType,
    this.isLoading = false,
    this.isFormValid = false,
    this.rememberMe = false,
    this.agreeToTerms = false,
    this.passwordVisibility = const PasswordVisibility(),
    this.errorMessage,
    this.successMessage,
  });

  /// Initial state for sign in form
  const AuthFormState.signIn()
    : formType = AuthFormType.signIn,
      isLoading = false,
      isFormValid = false,
      rememberMe = false,
      agreeToTerms = false,
      passwordVisibility = const PasswordVisibility(),
      errorMessage = null,
      successMessage = null;

  /// Initial state for sign up form
  const AuthFormState.signUp()
    : formType = AuthFormType.signUp,
      isLoading = false,
      isFormValid = false,
      rememberMe = false,
      agreeToTerms = false,
      passwordVisibility = const PasswordVisibility(),
      errorMessage = null,
      successMessage = null;

  /// Initial state for forgot password form
  const AuthFormState.forgotPassword()
    : formType = AuthFormType.forgotPassword,
      isLoading = false,
      isFormValid = false,
      rememberMe = false,
      agreeToTerms = false,
      passwordVisibility = const PasswordVisibility(),
      errorMessage = null,
      successMessage = null;

  AuthFormState copyWith({
    AuthFormType? formType,
    bool? isLoading,
    bool? isFormValid,
    bool? rememberMe,
    bool? agreeToTerms,
    PasswordVisibility? passwordVisibility,
    String? errorMessage,
    String? successMessage,
    bool clearError = false,
    bool clearSuccess = false,
  }) {
    return AuthFormState(
      formType: formType ?? this.formType,
      isLoading: isLoading ?? this.isLoading,
      isFormValid: isFormValid ?? this.isFormValid,
      rememberMe: rememberMe ?? this.rememberMe,
      agreeToTerms: agreeToTerms ?? this.agreeToTerms,
      passwordVisibility: passwordVisibility ?? this.passwordVisibility,
      errorMessage: clearError ? null : (errorMessage ?? this.errorMessage),
      successMessage: clearSuccess
          ? null
          : (successMessage ?? this.successMessage),
    );
  }

  /// Check if current form type is sign in
  bool get isSignIn => formType == AuthFormType.signIn;

  /// Check if current form type is sign up
  bool get isSignUp => formType == AuthFormType.signUp;

  /// Check if current form type is forgot password
  bool get isForgotPassword => formType == AuthFormType.forgotPassword;

  /// Check if form is in an error state
  bool get hasError => errorMessage != null;

  /// Check if form is in a success state
  bool get hasSuccess => successMessage != null;
}
