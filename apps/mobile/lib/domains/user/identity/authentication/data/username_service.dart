import 'dart:async';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/user/profile/data/datasources/user_api_datasource.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart';
import 'username_validation_service.dart';

/// Service for checking username availability
/// Provides real-time validation with debouncing
class UsernameService {
  final UserApiDatasource _datasource;
  Timer? _debounceTimer;

  UsernameService(this._datasource);

  /// Check username availability with debouncing
  void checkUsernameAvailability({
    required String username,
    required void Function(UsernameCheckResult) onResult,
    Duration delay = const Duration(milliseconds: 500),
  }) {
    // 🛡️ RACE CONDITION: Cancel previous pending request
    _debounceTimer?.cancel();

    // First validate format synchronously
    final formatResult = UsernameValidationService.validateUsernameFormat(
      username,
    );
    if (!formatResult.isValid) {
      onResult(formatResult);
      return;
    }

    // Set debounce timer before calling API
    _debounceTimer = Timer(delay, () async {
      // Check availability via API (converts to lowercase internally)
      final result = await _datasource.checkUsernameAvailability(username);

      if (result.isSuccess && result.data == true) {
        onResult(UsernameCheckResult.available());
      } else if (result.isSuccess && result.data == false) {
        onResult(UsernameCheckResult.unavailable());
      } else {
        // 🛡️ ERROR HANDLING: Reset to idle on API failure
        // Don't mark as taken, don't mark as available
        // Submit button remains disabled (idle state is not available)
        onResult(UsernameCheckResult.validFormat());
      }
    });
  }

  /// Dispose resources
  void dispose() {
    _debounceTimer?.cancel();
  }
}

/// Provider for UsernameService
final usernameServiceProvider = Provider<UsernameService>((ref) {
  final datasource = ref.watch(userApiDatasourceProvider);
  return UsernameService(datasource);
});
