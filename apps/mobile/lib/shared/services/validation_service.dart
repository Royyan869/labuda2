import 'package:labuda/core/src/interfaces/services/i_validation_service.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/shared/helpers/canonical_email_validator.dart';
import 'package:labuda/shared/helpers/canonical_password_policy.dart';
import 'package:labuda/shared/helpers/canonical_phone_validator.dart';
import 'package:labuda/shared/helpers/canonical_url_validator.dart';

/// Implementation ValidationService dengan Result pattern
///
/// Mengikuti DEVELOPMENT_STANDARDS_V1_ID.md:
/// - Interface-first design ✅
/// - Result pattern untuk error handling ✅
/// - No hardcoded secrets ✅
/// - Proper error handling dengan try-catch ✅
///
/// STAGE 4B-1: email / phone / URL validation delegate to the canonical
/// field validators (CanonicalEmailValidator / CanonicalPhoneValidator /
/// CanonicalUrlValidator). This service is a thin delegating wrapper — it
/// owns NO validation policy of its own for those fields.
///
/// STAGE 4B-2: the dead client-side anti-circumvention/content-moderation
/// hook was removed. Content moderation authority is the backend moderation
/// domain (backend/internal/governance/moderation/), not this service.
class ValidationService implements IValidationService {
  const ValidationService();

  @override
  Future<Result<bool>> validateEmail(String email) async {
    final message = CanonicalEmailValidator.validationMessage(email);
    if (message != null) {
      return Result.error(message);
    }
    return Result.success(true);
  }

  @override
  Future<Result<bool>> validatePassword(String password) async {
    try {
      // Delegates to the canonical Labuda password policy authority.
      // The policy is: min 8 chars + uppercase + lowercase + digit.
      final message = CanonicalPasswordPolicy.validationMessage(password);
      if (message != null) {
        return Result.error(message);
      }
      return Result.success(true);
    } catch (e) {
      return Result.error('Failed to validate password: ${e.toString()}');
    }
  }

  @override
  Future<Result<bool>> validatePhoneNumber(String phoneNumber) async {
    final message = CanonicalPhoneValidator.validationMessage(phoneNumber);
    if (message != null) {
      return Result.error(message);
    }
    return Result.success(true);
  }

  @override
  Future<Result<bool>> validateUrl(String url) async {
    final message = CanonicalUrlValidator.validationMessage(url);
    if (message != null) {
      return Result.error(message);
    }
    return Result.success(true);
  }
}
