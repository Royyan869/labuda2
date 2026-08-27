/// WithdrawResult entity tests
///
/// Verifies that WithdrawResult.failure() correctly preserves the error
/// message passed by the caller (regression for the silent-discard bug).
library;

import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/user/preference/seller/domain/entities/withdrawal.dart';

void main() {
  group('WithdrawResult.failure — error message preservation', () {
    test('preserves error message from parameter', () {
      const reason = 'Seller not verified';
      final result = WithdrawResult.failure(reason);
      expect(result.error, equals(reason));
    });

    test('isSuccess is false', () {
      final result = WithdrawResult.failure('some error');
      expect(result.isSuccess, isFalse);
    });

    test('status is WithdrawalStatus.failed', () {
      final result = WithdrawResult.failure('any');
      expect(result.status, equals(WithdrawalStatus.failed));
    });

    test('withdrawalId is empty string', () {
      final result = WithdrawResult.failure('any');
      expect(result.withdrawalId, equals(''));
    });

    test('distinct error messages produce distinct results', () {
      final r1 = WithdrawResult.failure('reason A');
      final r2 = WithdrawResult.failure('reason B');
      expect(r1.error, isNot(equals(r2.error)));
    });

    test('empty string error is preserved (not silently replaced)', () {
      final result = WithdrawResult.failure('');
      expect(result.error, equals(''));
    });
  });

  group('WithdrawResult.success — unaffected by fix', () {
    test('success factory preserves withdrawalId and status', () {
      final result = WithdrawResult.success(
        'wd-123',
        WithdrawalStatus.requested,
      );
      expect(result.withdrawalId, equals('wd-123'));
      expect(result.status, equals(WithdrawalStatus.requested));
      expect(result.isSuccess, isTrue);
    });

    test('success factory preserves backend fee and total debit values', () {
      // PASS_18H: totalDebitAmount equals the requested amount (200000),
      // never amount + fee — the fee is deducted FROM it, not added on top.
      final result = WithdrawResult.success(
        'wd-123',
        WithdrawalStatus.processing,
        feeAmount: 5000,
        totalDebitAmount: 200000,
      );

      expect(result.feeAmount, equals(5000));
      expect(result.totalDebitAmount, equals(200000));
    });
  });
}
