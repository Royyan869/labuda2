import 'package:equatable/equatable.dart';

/// Bank Account Status — mirrors the backend entity (active / deleted).
enum BankAccountStatus {
  /// Account is active and usable for payout.
  active,

  /// Account has been soft-deleted.
  deleted,
}

extension BankAccountStatusExtension on BankAccountStatus {
  String toApiString() => name; // 'active' | 'deleted'

  static BankAccountStatus parse(String? raw) {
    switch (raw) {
      case 'active':
        return BankAccountStatus.active;
      case 'deleted':
        return BankAccountStatus.deleted;
      default:
        return BankAccountStatus.active; // fail-safe
    }
  }
}

/// Bank Account Entity for seller payout information.
///
/// Wire shape (backend snake_case JSON):
///   id, bank_name, bank_code, account_number, account_holder_name,
///   is_default, status, created_at, updated_at
///
/// Note: This is NOT connected to any wallet functionality.
///       It is purely for storing payout bank account information.
class BankAccountEntity extends Equatable {
  final String id;
  final String bankName;
  final String bankCode;
  final String accountNumber;
  final String accountHolderName;
  final bool isDefault;
  final BankAccountStatus status;
  final DateTime createdAt;
  final DateTime updatedAt;

  const BankAccountEntity({
    required this.id,
    required this.bankName,
    required this.bankCode,
    required this.accountNumber,
    required this.accountHolderName,
    required this.isDefault,
    required this.status,
    required this.createdAt,
    required this.updatedAt,
  });

  factory BankAccountEntity.fromMap(Map<String, dynamic> map) {
    return BankAccountEntity(
      id: map['id'] as String? ?? '',
      bankName: map['bank_name'] as String? ?? '',
      bankCode: map['bank_code'] as String? ?? '',
      accountNumber: map['account_number'] as String? ?? '',
      accountHolderName: map['account_holder_name'] as String? ?? '',
      isDefault: map['is_default'] as bool? ?? false,
      status: BankAccountStatusExtension.parse(map['status'] as String?),
      createdAt: DateTime.parse(map['created_at'] as String),
      updatedAt: DateTime.parse(map['updated_at'] as String),
    );
  }

  /// toMap is used for the POST /api/v1/bank-accounts request body.
  /// Uses snake_case to match backend binding tags.
  Map<String, dynamic> toMap() {
    return {
      'bank_name': bankName,
      'bank_code': bankCode,
      'account_number': accountNumber,
      'account_holder_name': accountHolderName,
      'is_default': isDefault,
    };
  }

  BankAccountEntity copyWith({
    String? id,
    String? bankName,
    String? bankCode,
    String? accountNumber,
    String? accountHolderName,
    bool? isDefault,
    BankAccountStatus? status,
    DateTime? createdAt,
    DateTime? updatedAt,
  }) {
    return BankAccountEntity(
      id: id ?? this.id,
      bankName: bankName ?? this.bankName,
      bankCode: bankCode ?? this.bankCode,
      accountNumber: accountNumber ?? this.accountNumber,
      accountHolderName: accountHolderName ?? this.accountHolderName,
      isDefault: isDefault ?? this.isDefault,
      status: status ?? this.status,
      createdAt: createdAt ?? this.createdAt,
      updatedAt: updatedAt ?? this.updatedAt,
    );
  }

  @override
  List<Object?> get props => [
    id,
    bankName,
    bankCode,
    accountNumber,
    accountHolderName,
    isDefault,
    status,
    createdAt,
    updatedAt,
  ];
}

/// Bank Info for dropdown selection (local-only, not serialized).
class BankInfo {
  final String code;
  final String name;
  final String icon;

  const BankInfo({required this.code, required this.name, required this.icon});
}
