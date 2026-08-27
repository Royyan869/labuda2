import 'package:labuda/core/common/result.dart';

/// Interface untuk validation service yang menggunakan `Result<T>` pattern
///
/// Mengikuti DEVELOPMENT_STANDARDS_V1_ID.md:
/// - Interface-first design (WAJIB)
/// - `Result<T>` pattern untuk error handling (WAJIB)
/// - No business logic in UI widgets (WAJIB)
///
/// **NOTE:** Domain-specific validations (price, Koi size, etc.) have been moved to
/// CommerceValidationService in domains/commerce/catalog/shared/commerce_validation_service.dart
///
/// **STAGE 4D:** The dead generic validation surface (`validateUsername`,
/// `validateContent`, `isValidUserId`, `validateForm`, `validateRequired`,
/// `validateMinLength`, `validateMaxLength`) was removed after a final caller
/// audit proved zero consumers. Remaining methods all delegate to canonical
/// authorities (CanonicalEmailValidator / CanonicalPhoneValidator /
/// CanonicalUrlValidator / CanonicalPasswordPolicy). Username format
/// authority is CanonicalUsernameValidator via UsernameValidationService —
/// NOT this interface.
abstract class IValidationService {
  /// Validasi email dengan `Result<T>` pattern
  Future<Result<bool>> validateEmail(String email);

  /// Validasi password dengan `Result<T>` pattern
  Future<Result<bool>> validatePassword(String password);

  /// Validasi nomor telepon Indonesia
  Future<Result<bool>> validatePhoneNumber(String phoneNumber);

  /// Validasi URL media
  Future<Result<bool>> validateUrl(String url);
}
