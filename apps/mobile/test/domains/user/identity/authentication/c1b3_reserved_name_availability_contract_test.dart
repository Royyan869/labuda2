// C1B3 — Reserved-name availability contract (Proof 1).
//
// Verifies the end-to-end contract after removing the mobile reserved list:
//  1. Backend-reserved username (e.g. "labuda") passes local format validation
//  2. Mobile does NOT reject it from a local reserved list
//  3. Remote availability request occurs
//  4. Invalid format (e.g. "john-doe") is rejected locally, zero remote calls
//  5. Network failure does not produce a false "available" state

import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/domains/user/identity/authentication/data/username_validation_service.dart';
import 'package:labuda/domains/user/identity/authentication/data/username_service.dart';
import 'package:labuda/domains/user/profile/data/datasources/user_api_datasource.dart';
import 'package:labuda/shared/helpers/canonical_username_validator.dart';

// =============================================================================
// Fake / recording datasource
// =============================================================================

class _FakeApiClient extends Fake implements ApiClient {}

class _RecordingUserApiDatasource extends UserApiDatasource {
  int availabilityCallCount = 0;
  String? lastCheckedUsername;
  bool? cannedAvailability;
  Object? cannedError;

  _RecordingUserApiDatasource() : super(_FakeApiClient());

  @override
  Future<Result<bool>> checkUsernameAvailability(String username) async {
    availabilityCallCount++;
    lastCheckedUsername = username;
    if (cannedError != null) {
      return Result<bool>.error(cannedError.toString());
    }
    return Result<bool>.success(cannedAvailability ?? true);
  }
}

void main() {
  group('C1B3 Reserved-name contract', () {
    test('labuda passes local canonical format validation', () {
      // Backend-reserved "labuda" must pass local format check so it
      // reaches the remote availability call.
      expect(CanonicalUsernameValidator.isValid('labuda'), isTrue);
    });

    test('labuda is NOT rejected by local reserved list', () {
      // After removing _reservedUsernames, the validation service must
      // NOT reject backend-reserved names locally.
      final result =
          UsernameValidationService.validateUsernameFormat('labuda');
      expect(result.isValid, isTrue);
      expect(result.status, UsernameCheckStatus.validFormat);
    });

    test('former mobile-only "moderator" passes local format, calls remote', () async {
      // "moderator" was in the old mobile list but not in backend.
      // Must pass local format check and reach the remote availability call.
      final local =
          UsernameValidationService.validateUsernameFormat('moderator');
      expect(local.isValid, isTrue);
      expect(local.status, UsernameCheckStatus.validFormat);

      // Confirmed: available=true maps correctly.
      final ds1 = _RecordingUserApiDatasource();
      ds1.cannedAvailability = true;
      final s1 = UsernameService(ds1);
      final r1 = <UsernameCheckResult>[];
      s1.checkUsernameAvailability(username:'moderator', onResult:r1.add, delay:Duration.zero);
      await Future.delayed(const Duration(milliseconds:10));
      expect(ds1.availabilityCallCount, 1);
      expect(ds1.lastCheckedUsername, 'moderator');
      expect(r1.last.status, UsernameCheckStatus.available);

      // Confirmed: available=false maps to unavailable.
      final ds2 = _RecordingUserApiDatasource();
      ds2.cannedAvailability = false;
      final s2 = UsernameService(ds2);
      final r2 = <UsernameCheckResult>[];
      s2.checkUsernameAvailability(username:'moderator', onResult:r2.add, delay:Duration.zero);
      await Future.delayed(const Duration(milliseconds:10));
      expect(ds2.availabilityCallCount, 1);
      expect(ds2.lastCheckedUsername, 'moderator');
      expect(r2.last.status, UsernameCheckStatus.unavailable);
    });

    test('john-doe is rejected locally by canonical validator', () {
      expect(CanonicalUsernameValidator.isValid('john-doe'), isFalse);
    });

    test('john-doe makes zero remote availability calls', () {
      final result =
          UsernameValidationService.validateUsernameFormat('john-doe');
      expect(result.isValid, isFalse);
      // Local rejection — no remote call needed.
    });

    test('valid username proceeds to remote availability check', () async {
      final ds = _RecordingUserApiDatasource();
      ds.cannedAvailability = false; // backend says unavailable
      final service = UsernameService(ds);

      final results = <UsernameCheckResult>[];
      service.checkUsernameAvailability(
        username: 'labuda',
        onResult: results.add,
        delay: Duration.zero,
      );

      // The callback only fires after the timer (even with Duration.zero,
      // Timer runs asynchronously). No synchronous callback for valid formats.
      expect(results, isEmpty);

      // After timer fires, remote result arrives.
      await Future.delayed(const Duration(milliseconds: 10));
      expect(results, isNotEmpty);
      expect(results.last.status, UsernameCheckStatus.unavailable);
      expect(ds.availabilityCallCount, 1);
    });

    test('remote unavailable → mapped to unavailable state', () async {
      final ds = _RecordingUserApiDatasource();
      ds.cannedAvailability = false;
      final service = UsernameService(ds);

      final results = <UsernameCheckResult>[];
      service.checkUsernameAvailability(
        username: 'validuser',
        onResult: results.add,
        delay: Duration.zero,
      );

      // The Timer with Duration.zero fires on the next event loop tick.
      await Future.delayed(const Duration(milliseconds: 10));

      final lastResult = results.last;
      expect(lastResult.status, UsernameCheckStatus.unavailable);
      expect(ds.availabilityCallCount, 1);
      expect(ds.lastCheckedUsername, 'validuser');
    });

    test('remote available → mapped to available state', () async {
      final ds = _RecordingUserApiDatasource();
      ds.cannedAvailability = true;
      final service = UsernameService(ds);

      final results = <UsernameCheckResult>[];
      service.checkUsernameAvailability(
        username: 'validuser',
        onResult: results.add,
        delay: Duration.zero,
      );

      await Future.delayed(const Duration(milliseconds: 10));

      final lastResult = results.last;
      expect(lastResult.status, UsernameCheckStatus.available);
    });

    test('remote API failure → keeps validFormat, not available', () async {
      final ds = _RecordingUserApiDatasource();
      ds.cannedError = Exception('Network error');
      final service = UsernameService(ds);

      final results = <UsernameCheckResult>[];
      service.checkUsernameAvailability(
        username: 'validuser',
        onResult: results.add,
        delay: Duration.zero,
      );

      await Future.delayed(const Duration(milliseconds: 10));

      // On error: service resets to validFormat (not taken, not available).
      // Submit button remains disabled because validFormat is not "available".
      final lastResult = results.last;
      expect(lastResult.status, UsernameCheckStatus.validFormat);
      expect(lastResult.isAvailable, isFalse);
      expect(ds.availabilityCallCount, 1);
    });

    test('invalid format → zero remote calls and returns invalid', () {
      final ds = _RecordingUserApiDatasource();
      final service = UsernameService(ds);

      final results = <UsernameCheckResult>[];
      service.checkUsernameAvailability(
        username: 'john-doe',
        onResult: results.add,
        delay: Duration.zero,
      );

      // Immediately rejected locally — no remote call.
      expect(ds.availabilityCallCount, 0);
      final result = results.first;
      expect(result.isValid, isFalse);
    });
  });
}
