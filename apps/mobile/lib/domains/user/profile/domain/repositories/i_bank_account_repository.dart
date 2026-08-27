// Internal
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/profile/domain/entities/bank_account_entity.dart';

/// Repository interface for bank account operations.
///
/// Routes (backend):
///   GET  /api/v1/bank-accounts
///   POST /api/v1/bank-accounts
///   DELETE /api/v1/bank-accounts/:id
///   PATCH /api/v1/bank-accounts/:id/default
///
/// No PUT/update route exists — the backend only supports add/delete/set-default.
abstract class IBankAccountRepository {
  /// Watch all bank accounts for a user (converts single fetch to stream).
  Stream<Result<List<BankAccountEntity>>> watchBankAccounts(String userId);

  /// Add a new bank account.
  Future<Result<BankAccountEntity>> addBankAccount(BankAccountEntity account);

  /// Delete a bank account.
  Future<Result<void>> deleteBankAccount(String accountId);

  /// Set a bank account as the default payout destination.
  /// PATCH /api/v1/bank-accounts/:id/default
  Future<Result<void>> setPrimaryBankAccount(String accountId, String userId);

  /// Get the default payout bank account.
  /// Implemented as list-then-filter (no dedicated /primary or /default GET endpoint).
  Future<Result<BankAccountEntity?>> getPrimaryBankAccount(String userId);
}
