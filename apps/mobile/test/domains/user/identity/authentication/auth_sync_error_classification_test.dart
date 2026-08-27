// PASS 2A / F1: structured auth-sync error classification.
//
// Verifies classifyAuthSyncError() classifies failed backend responses from
// POST /api/v1/auth/firebase/exchange and GET /users/me using the backend's
// structured errorCode/statusCode (see
// backend/internal/identity/auth/delivery/http/auth_handler.go) as the
// PRIMARY signal, rather than free-text message matching — which silently
// misclassified the backend's actual messages before this fix:
//   - 401 INVALID_TOKEN:   "Invalid or expired Firebase token"
//   - 403 ACCOUNT_DELETED: "Account has been deleted"
//   - 403 ACCOUNT_INACTIVE: "Account is {status}"
// None of these contained the old matcher's substrings ('invalid token',
// 'user deleted'), so all three fell through to a generic, indefinitely-
// retryable AuthSyncErrorKind.backendFailure.
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/user/identity/authentication/presentation/providers/auth_controller.dart';

void main() {
  group('classifyAuthSyncError — structured codes (primary)', () {
    test('INVALID_TOKEN (401) classifies as identityInvalid', () {
      final kind = classifyAuthSyncError(
        'Invalid or expired Firebase token',
        errorCode: 'INVALID_TOKEN',
        statusCode: 401,
      );
      expect(kind, AuthSyncErrorKind.identityInvalid);
    });

    test('ACCOUNT_DELETED (403) classifies as accountDeleted', () {
      final kind = classifyAuthSyncError(
        'Account has been deleted',
        errorCode: 'ACCOUNT_DELETED',
        statusCode: 403,
      );
      expect(kind, AuthSyncErrorKind.accountDeleted);
    });

    test('ACCOUNT_INACTIVE (403) classifies as accountInactive', () {
      final kind = classifyAuthSyncError(
        'Account is suspended',
        errorCode: 'ACCOUNT_INACTIVE',
        statusCode: 403,
      );
      expect(kind, AuthSyncErrorKind.accountInactive);
    });

    test('ACCOUNT_INACTIVE (403) classifies as accountInactive regardless of '
        'the specific status word in the message (banned variant)', () {
      final kind = classifyAuthSyncError(
        'Account is banned',
        errorCode: 'ACCOUNT_INACTIVE',
        statusCode: 403,
      );
      expect(kind, AuthSyncErrorKind.accountInactive);
    });

    test('structured code wins even if the message text would otherwise '
        'match a different fallback bucket', () {
      // Message contains "connection" (a backendUnavailable keyword) but
      // the structured code must still win.
      final kind = classifyAuthSyncError(
        'connection rejected: invalid or expired firebase token',
        errorCode: 'INVALID_TOKEN',
        statusCode: 401,
      );
      expect(kind, AuthSyncErrorKind.identityInvalid);
    });
  });

  group('classifyAuthSyncError — backend-unreachable / network errors', () {
    test('a bare 5xx status with no matching code is backendUnavailable', () {
      final kind = classifyAuthSyncError(
        'Internal Server Error',
        errorCode: null,
        statusCode: 500,
      );
      expect(kind, AuthSyncErrorKind.backendUnavailable);
    });

    test('BACKEND_UNREACHABLE code (from the mobile connectionError '
        'interceptor) is not one of the three identity codes and falls back '
        'to free-text matching, which classifies it as backendUnavailable', () {
      final kind = classifyAuthSyncError(
        'Cannot reach Labuda server. Check that the backend is running '
        'and the device is on the same network.',
        errorCode: 'BACKEND_UNREACHABLE',
      );
      expect(kind, AuthSyncErrorKind.backendUnavailable);
    });

    test(
      'a timeout with no structured code falls back to backendUnavailable',
      () {
        final kind = classifyAuthSyncError('SYNC TIMEOUT');
        expect(kind, AuthSyncErrorKind.backendUnavailable);
      },
    );

    test('a socket exception string falls back to backendUnavailable', () {
      final kind = classifyAuthSyncError('SocketException: Connection refused');
      expect(kind, AuthSyncErrorKind.backendUnavailable);
    });
  });

  group('classifyAuthSyncError — generic backend failure', () {
    test(
      'a generic 4xx validation error with no matching code is backendFailure',
      () {
        final kind = classifyAuthSyncError(
          'Username already taken',
          errorCode: 'USERNAME_TAKEN',
          statusCode: 409,
        );
        expect(kind, AuthSyncErrorKind.backendFailure);
      },
    );

    test(
      'an unrecognized error with no code/status/keywords is backendFailure',
      () {
        final kind = classifyAuthSyncError('Something went wrong');
        expect(kind, AuthSyncErrorKind.backendFailure);
      },
    );

    test('null error message with no code is backendFailure', () {
      final kind = classifyAuthSyncError(null);
      expect(kind, AuthSyncErrorKind.backendFailure);
    });
  });

  group(
    'classifyAuthSyncError — legacy free-text fallback (no structured code)',
    () {
      test(
        'raw Firebase SDK "user-not-found" still classifies as identityInvalid',
        () {
          final kind = classifyAuthSyncError('firebase_auth/user-not-found');
          expect(kind, AuthSyncErrorKind.identityInvalid);
        },
      );

      test(
        'raw Firebase SDK "auth/invalid-credential" still classifies as identityInvalid',
        () {
          final kind = classifyAuthSyncError('auth/invalid-credential');
          expect(kind, AuthSyncErrorKind.identityInvalid);
        },
      );
    },
  );
}
