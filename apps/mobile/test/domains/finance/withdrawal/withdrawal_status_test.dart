/// WithdrawalStatus enum tests
///
/// Verifies PILOT_BLOCKED parsing, predicate correctness, and that
/// unknown status fallback does not produce dangerously misleading results.
library;

import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/user/preference/seller/domain/entities/withdrawal.dart';

void main() {
  group('WithdrawalStatus.fromString - PILOT_BLOCKED', () {
    test('PILOT_BLOCKED parses to pilotBlocked', () {
      expect(
        WithdrawalStatus.fromString('PILOT_BLOCKED'),
        equals(WithdrawalStatus.pilotBlocked),
      );
    });

    test('wire value of pilotBlocked is PILOT_BLOCKED', () {
      expect(WithdrawalStatus.pilotBlocked.value, equals('PILOT_BLOCKED'));
    });
  });

  group('WithdrawalStatus predicates - pilotBlocked', () {
    test('pilotBlocked is NOT isFinal', () {
      expect(WithdrawalStatus.pilotBlocked.isFinal, isFalse);
    });

    test('pilotBlocked IS isPending (inverse of isFinal)', () {
      expect(WithdrawalStatus.pilotBlocked.isPending, isTrue);
    });

    test('pilotBlocked is NOT isFailed', () {
      expect(WithdrawalStatus.pilotBlocked.isFailed, isFalse);
    });

    test('pilotBlocked is NOT isSuccessful', () {
      expect(WithdrawalStatus.pilotBlocked.isSuccessful, isFalse);
    });
  });

  group('WithdrawalStatus predicates - existing statuses unchanged', () {
    test(
      'settled isFinal',
      () => expect(WithdrawalStatus.settled.isFinal, isTrue),
    );
    test(
      'completed isFinal',
      () => expect(WithdrawalStatus.completed.isFinal, isTrue),
    );
    test(
      'failed isFinal',
      () => expect(WithdrawalStatus.failed.isFinal, isTrue),
    );
    test(
      'failedFinal isFinal',
      () => expect(WithdrawalStatus.failedFinal.isFinal, isTrue),
    );
    test(
      'requested NOT isFinal',
      () => expect(WithdrawalStatus.requested.isFinal, isFalse),
    );
    test(
      'processing NOT isFinal',
      () => expect(WithdrawalStatus.processing.isFinal, isFalse),
    );
    test(
      'failedRetryable NOT isFinal',
      () => expect(WithdrawalStatus.failedRetryable.isFinal, isFalse),
    );

    test(
      'failed isFailed',
      () => expect(WithdrawalStatus.failed.isFailed, isTrue),
    );
    test(
      'failedFinal isFailed',
      () => expect(WithdrawalStatus.failedFinal.isFailed, isTrue),
    );
    test(
      'failedRetryable isFailed',
      () => expect(WithdrawalStatus.failedRetryable.isFailed, isTrue),
    );
    test(
      'settled NOT isFailed',
      () => expect(WithdrawalStatus.settled.isFailed, isFalse),
    );
  });

  group('WithdrawalStatus.fromString - unknown fallback', () {
    test('unknown string falls back to unknown', () {
      expect(
        WithdrawalStatus.fromString('TOTALLY_UNKNOWN'),
        equals(WithdrawalStatus.unknown),
      );
    });

    test('PILOT_BLOCKED does NOT fall back to requested', () {
      expect(
        WithdrawalStatus.fromString('PILOT_BLOCKED'),
        isNot(equals(WithdrawalStatus.requested)),
      );
    });

    test('all known statuses parse without fallback', () {
      const knownValues = [
        'REQUESTED',
        'PROCESSING',
        'SUBMITTED',
        'SETTLING',
        'SETTLED',
        'COMPLETED',
        'FAILED',
        'FAILED_RETRYABLE',
        'FAILED_FINAL',
        'PILOT_BLOCKED',
      ];
      for (final wire in knownValues) {
        final parsed = WithdrawalStatus.fromString(wire);
        expect(
          parsed.value,
          equals(wire),
          reason: '$wire must round-trip through fromString without fallback',
        );
      }
    });

    test('unknown is neither final nor failed nor successful', () {
      expect(WithdrawalStatus.unknown.isFinal, isFalse);
      expect(WithdrawalStatus.unknown.isPending, isFalse);
      expect(WithdrawalStatus.unknown.isFailed, isFalse);
      expect(WithdrawalStatus.unknown.isSuccessful, isFalse);
    });
  });
}
