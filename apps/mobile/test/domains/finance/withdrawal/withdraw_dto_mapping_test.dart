/// Withdraw DTO mapping test
///
/// Verifies that WithdrawalItemDto and WithdrawHistoryResponseDto correctly
/// parse the actual backend response shape from GET /withdraw/history.
///
/// Backend response shape (withdrawal_handler_unified.go:ListWithdrawals):
///   { "withdrawals": [ { "withdrawal_id", "amount", "status",
///                         "reference_code", "requested_at", "processed_at" } ],
///     "total", "limit", "offset" }
library;

import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/user/preference/seller/data/dto/withdraw_dto.dart';

void main() {
  group('WithdrawalItemDto.fromJson - backend shape', () {
    test('parses withdrawal_id as id', () {
      final dto = WithdrawalItemDto.fromJson(_backendItem());
      expect(dto.id, equals('wd-uuid-123'));
    });

    test('parses amount', () {
      final dto = WithdrawalItemDto.fromJson(_backendItem());
      expect(dto.amount, equals(500000));
    });

    test('parses feeAmount from backend when present', () {
      final dto = WithdrawalItemDto.fromJson(_backendItem());
      expect(dto.feeAmount, equals(5000));
    });

    test('parses status', () {
      final dto = WithdrawalItemDto.fromJson(_backendItem());
      expect(dto.status, equals('SETTLED'));
    });

    test('parses requested_at as createdAt', () {
      final dto = WithdrawalItemDto.fromJson(_backendItem());
      expect(dto.createdAt, equals('2024-06-01T10:00:00Z'));
    });

    test('parses processed_at as updatedAt', () {
      final dto = WithdrawalItemDto.fromJson(_backendItem());
      expect(dto.updatedAt, equals('2024-06-02T10:00:00Z'));
    });

    test('falls back to requested_at when processed_at is null', () {
      final item = _backendItem();
      item['processed_at'] = null;
      final dto = WithdrawalItemDto.fromJson(item);
      expect(dto.updatedAt, equals('2024-06-01T10:00:00Z'));
    });

    test(
      'requested_at absent does not throw - createdAt falls back to empty string',
      () {
        final item = _backendItem();
        item.remove('requested_at');
        item.remove('created_at');
        // Must not throw; createdAt falls back to empty string
        expect(() => WithdrawalItemDto.fromJson(item), returnsNormally);
        final dto = WithdrawalItemDto.fromJson(item);
        expect(dto.createdAt, equals(''));
      },
    );

    test('sellerId defaults to empty string when absent', () {
      final dto = WithdrawalItemDto.fromJson(_backendItem());
      expect(dto.sellerId, equals(''));
    });

    test('bank fields are null when absent', () {
      final dto = WithdrawalItemDto.fromJson(_backendItem());
      expect(dto.bankNameSnapshot, isNull);
      expect(dto.accountNumberSnapshot, isNull);
    });
  });

  group('WithdrawResponseDto.fromJson - backend result shape', () {
    test('preserves backend fee_amount and total_debit_amount', () {
      // PASS_18H: total_debit_amount equals the requested amount (200000),
      // never amount + fee. The fee is deducted FROM the requested amount
      // at settlement, not added on top of the seller-payable debit.
      final dto = WithdrawResponseDto.fromJson(_backendResponse());
      expect(dto.feeAmount, equals(5000));
      expect(dto.totalDebitAmount, equals(200000));
    });

    test('defaults to zero when backend omits fee fields', () {
      final response = _backendResponse();
      response.remove('fee_amount');
      response.remove('total_debit_amount');

      final dto = WithdrawResponseDto.fromJson(response);
      expect(dto.feeAmount, equals(0));
      expect(dto.totalDebitAmount, equals(0));
    });
  });

  group('WithdrawHistoryResponseDto.fromJson - backend shape', () {
    test('parses withdrawals list', () {
      final dto = WithdrawHistoryResponseDto.fromJson(
        _backendHistoryResponse(),
      );
      expect(dto.withdrawals.length, equals(2));
    });

    test("parses count from backend 'total' key", () {
      // Backend sends "total" (not "count"); DTO must read the correct key.
      final dto = WithdrawHistoryResponseDto.fromJson(
        _backendHistoryResponse(),
      );
      expect(dto.count, equals(2));
    });

    test('total from backend overrides list length on paginated response', () {
      // Paginated response: backend total=50 but only 3 items in this page.
      // count must equal 50, not 3.
      final dto = WithdrawHistoryResponseDto.fromJson(_paginatedResponse());
      expect(dto.count, equals(50));
    });

    test('falls back to list length when total key absent', () {
      final response = _backendHistoryResponse();
      response.remove('total');
      final dto = WithdrawHistoryResponseDto.fromJson(response);
      expect(dto.count, equals(dto.withdrawals.length));
    });

    test('first withdrawal has correct id', () {
      final dto = WithdrawHistoryResponseDto.fromJson(
        _backendHistoryResponse(),
      );
      expect(dto.withdrawals[0].id, equals('wd-uuid-001'));
    });

    test('second withdrawal has correct status', () {
      final dto = WithdrawHistoryResponseDto.fromJson(
        _backendHistoryResponse(),
      );
      expect(dto.withdrawals[1].status, equals('REQUESTED'));
    });

    test('history rows preserve backend fee_amount', () {
      final dto = WithdrawHistoryResponseDto.fromJson(
        _backendHistoryResponse(),
      );
      expect(dto.withdrawals[0].feeAmount, equals(5000));
    });
  });
}

// Helpers

// PASS_18H: total_debit_amount fixtures equal `amount` (never amount+fee) —
// the fee is deducted FROM the requested amount, not added on top.
Map<String, dynamic> _backendItem() => {
  'withdrawal_id': 'wd-uuid-123',
  'amount': 500000,
  'fee_amount': 5000,
  'total_debit_amount': 500000,
  'status': 'SETTLED',
  'reference_code': 'WD_REF_001',
  'requested_at': '2024-06-01T10:00:00Z',
  'processed_at': '2024-06-02T10:00:00Z',
};

Map<String, dynamic> _backendResponse() => {
  'withdrawal_id': 'wd-uuid-123',
  'status': 'REQUESTED',
  'fee_amount': 5000,
  'total_debit_amount': 200000,
};

Map<String, dynamic> _backendHistoryResponse() => {
  'withdrawals': [
    {
      'withdrawal_id': 'wd-uuid-001',
      'amount': 200000,
      'fee_amount': 5000,
      'total_debit_amount': 200000,
      'status': 'COMPLETED',
      'reference_code': 'WD_REF_001',
      'requested_at': '2024-06-01T10:00:00Z',
      'processed_at': '2024-06-03T12:00:00Z',
    },
    {
      'withdrawal_id': 'wd-uuid-002',
      'amount': 100000,
      'fee_amount': 5000,
      'total_debit_amount': 100000,
      'status': 'REQUESTED',
      'reference_code': null,
      'requested_at': '2024-06-05T08:00:00Z',
      'processed_at': null,
    },
  ],
  'total': 2,
  'limit': 20,
  'offset': 0,
};

/// Paginated response: only 3 items in page but DB total = 50.
Map<String, dynamic> _paginatedResponse() => {
  'withdrawals': [
    {
      'withdrawal_id': 'wd-uuid-010',
      'amount': 100000,
      'status': 'SETTLED',
      'requested_at': '2024-05-01T08:00:00Z',
      'processed_at': '2024-05-02T08:00:00Z',
    },
    {
      'withdrawal_id': 'wd-uuid-011',
      'amount': 200000,
      'status': 'REQUESTED',
      'requested_at': '2024-05-03T08:00:00Z',
      'processed_at': null,
    },
    {
      'withdrawal_id': 'wd-uuid-012',
      'amount': 150000,
      'status': 'PROCESSING',
      'requested_at': '2024-05-04T08:00:00Z',
      'processed_at': null,
    },
  ],
  'total': 50,
  'limit': 3,
  'offset': 0,
};
