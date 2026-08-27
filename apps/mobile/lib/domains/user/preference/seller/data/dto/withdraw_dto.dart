/// Withdraw DTOs
///
/// Data Transfer Objects for withdraw API serialization.
library;

import 'package:equatable/equatable.dart';

/// Withdraw Request DTO
class WithdrawRequestDto extends Equatable {
  final int amount;

  const WithdrawRequestDto({required this.amount});

  factory WithdrawRequestDto.fromJson(Map<String, dynamic> json) {
    return WithdrawRequestDto(amount: json['amount'] as int);
  }

  Map<String, dynamic> toJson() {
    return {'amount': amount};
  }

  @override
  List<Object?> get props => [amount];
}

/// Withdraw Response DTO
class WithdrawResponseDto extends Equatable {
  final String withdrawalId;
  final String status;
  final int feeAmount;
  final int totalDebitAmount;

  const WithdrawResponseDto({
    required this.withdrawalId,
    required this.status,
    this.feeAmount = 0,
    this.totalDebitAmount = 0,
  });

  factory WithdrawResponseDto.fromJson(Map<String, dynamic> json) {
    return WithdrawResponseDto(
      withdrawalId: json['withdrawal_id'] as String,
      status: json['status'] as String,
      feeAmount: json['fee_amount'] as int? ?? 0,
      totalDebitAmount: json['total_debit_amount'] as int? ?? 0,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'withdrawal_id': withdrawalId,
      'status': status,
      'fee_amount': feeAmount,
      'total_debit_amount': totalDebitAmount,
    };
  }

  @override
  List<Object?> get props => [
    withdrawalId,
    status,
    feeAmount,
    totalDebitAmount,
  ];
}

/// Withdrawal Item DTO (for history)
class WithdrawalItemDto extends Equatable {
  final String id;
  final String sellerId;
  final int amount;
  final int feeAmount;
  final String status;
  final String? bankNameSnapshot;
  final String? bankCodeSnapshot; // For payout rail integration
  final String? accountNumberSnapshot;
  final String? accountHolderSnapshot;
  final String createdAt;
  final String updatedAt;

  const WithdrawalItemDto({
    required this.id,
    required this.sellerId,
    required this.amount,
    required this.feeAmount,
    required this.status,
    this.bankNameSnapshot,
    this.bankCodeSnapshot,
    this.accountNumberSnapshot,
    this.accountHolderSnapshot,
    required this.createdAt,
    required this.updatedAt,
  });

  factory WithdrawalItemDto.fromJson(Map<String, dynamic> json) {
    // Backend sends withdrawal_id / requested_at / processed_at.
    // Legacy keys id / created_at / updated_at accepted as fallback.
    return WithdrawalItemDto(
      id: json['withdrawal_id'] as String? ?? json['id'] as String,
      sellerId: json['seller_id'] as String? ?? '',
      amount: json['amount'] as int,
      feeAmount: json['fee_amount'] as int? ?? 0,
      status: json['status'] as String,
      bankNameSnapshot: json['bank_name_snapshot'] as String?,
      bankCodeSnapshot: json['bank_code_snapshot'] as String?,
      accountNumberSnapshot: json['account_number_snapshot'] as String?,
      accountHolderSnapshot: json['account_holder_snapshot'] as String?,
      createdAt:
          json['requested_at'] as String? ??
          json['created_at'] as String? ??
          '',
      updatedAt:
          json['processed_at'] as String? ??
          json['updated_at'] as String? ??
          json['requested_at'] as String? ??
          '',
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'seller_id': sellerId,
      'amount': amount,
      'fee_amount': feeAmount,
      'status': status,
      if (bankNameSnapshot != null) 'bank_name_snapshot': bankNameSnapshot,
      if (bankCodeSnapshot != null) 'bank_code_snapshot': bankCodeSnapshot,
      if (accountNumberSnapshot != null)
        'account_number_snapshot': accountNumberSnapshot,
      if (accountHolderSnapshot != null)
        'account_holder_snapshot': accountHolderSnapshot,
      'created_at': createdAt,
      'updated_at': updatedAt,
    };
  }

  @override
  List<Object?> get props => [
    id,
    sellerId,
    amount,
    feeAmount,
    status,
    bankNameSnapshot,
    bankCodeSnapshot,
    accountNumberSnapshot,
    accountHolderSnapshot,
    createdAt,
    updatedAt,
  ];
}

/// Withdraw History Response DTO
class WithdrawHistoryResponseDto extends Equatable {
  final List<WithdrawalItemDto> withdrawals;
  final int count;

  const WithdrawHistoryResponseDto({
    required this.withdrawals,
    required this.count,
  });

  factory WithdrawHistoryResponseDto.fromJson(Map<String, dynamic> json) {
    final withdrawalsList = json['withdrawals'] as List<dynamic>;
    return WithdrawHistoryResponseDto(
      withdrawals: withdrawalsList
          .map((e) => WithdrawalItemDto.fromJson(e as Map<String, dynamic>))
          .toList(),
      count: json['total'] as int? ?? withdrawalsList.length,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'withdrawals': withdrawals.map((e) => e.toJson()).toList(),
      'count': count,
    };
  }

  @override
  List<Object?> get props => [withdrawals, count];
}
