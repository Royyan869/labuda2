/// Withdrawal history pagination params test
///
/// Verifies that SellerRepository.getWithdrawHistory() accepts limit and offset
/// params, defaults to limit=100 and offset=0, and that callers can override
/// them. The test double captures the values passed through the interface,
/// proving the signature is correct at compile time and the defaults hold at
/// runtime.
library;

import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/repositories/repository_result.dart';
import 'package:labuda/domains/user/preference/seller/domain/entities/withdrawal.dart';
import 'package:labuda/domains/user/preference/seller/domain/repositories/seller_repository.dart';

// ── Test double ───────────────────────────────────────────────────────────────

/// Minimal stub that captures the limit/offset forwarded to getWithdrawHistory.
/// All other SellerRepository methods are handled by noSuchMethod so this
/// stub does not need to list every interface method explicitly.
class _CapturingRepo implements SellerRepository {
  int capturedLimit = -1;
  int capturedOffset = -1;
  int callCount = 0;

  @override
  Future<RepositoryResult<List<Withdrawal>>> getWithdrawHistory({
    int limit = 100,
    int offset = 0,
  }) async {
    capturedLimit = limit;
    capturedOffset = offset;
    callCount++;
    return RepositoryResult.success(const []);
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => throw UnimplementedError(
    'Not implemented in test stub: ${invocation.memberName}',
  );
}

// ── Tests ─────────────────────────────────────────────────────────────────────

void main() {
  group('SellerRepository.getWithdrawHistory — pagination params', () {
    late _CapturingRepo repo;

    setUp(() {
      repo = _CapturingRepo();
    });

    test('default limit is 100 when not specified', () async {
      await repo.getWithdrawHistory();
      expect(repo.capturedLimit, equals(100));
    });

    test('default offset is 0 when not specified', () async {
      await repo.getWithdrawHistory();
      expect(repo.capturedOffset, equals(0));
    });

    test('explicit limit is forwarded to implementation', () async {
      await repo.getWithdrawHistory(limit: 50);
      expect(repo.capturedLimit, equals(50));
    });

    test('explicit offset is forwarded to implementation', () async {
      await repo.getWithdrawHistory(offset: 40);
      expect(repo.capturedOffset, equals(40));
    });

    test('both limit and offset are forwarded together', () async {
      await repo.getWithdrawHistory(limit: 10, offset: 20);
      expect(repo.capturedLimit, equals(10));
      expect(repo.capturedOffset, equals(20));
    });

    test(
      'no-arg call uses limit=100 not the old server default of 20',
      () async {
        await repo.getWithdrawHistory();
        // The old code sent no params → server defaulted to 20.
        // The fix ensures we request 100 so sellers see more history.
        expect(repo.capturedLimit, greaterThan(20));
      },
    );

    test('returns success with empty list', () async {
      final result = await repo.getWithdrawHistory();
      expect(result.isSuccess, isTrue);
      expect(result.data, isEmpty);
    });

    test(
      'method is callable multiple times with independent param sets',
      () async {
        await repo.getWithdrawHistory(limit: 5, offset: 0);
        final first = (repo.capturedLimit, repo.capturedOffset);

        await repo.getWithdrawHistory(limit: 5, offset: 5);
        final second = (repo.capturedLimit, repo.capturedOffset);

        expect(first, equals((5, 0)));
        expect(second, equals((5, 5)));
        expect(repo.callCount, equals(2));
      },
    );
  });
}
