// Dart
import 'package:flutter_riverpod/flutter_riverpod.dart';

// Internal
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/profile/data/repositories/bank_account_repository_impl.dart';
import 'package:labuda/domains/user/profile/domain/entities/bank_account_entity.dart';
import 'package:labuda/domains/user/profile/domain/repositories/i_bank_account_repository.dart';

/// Provider for bank account repository
final bankAccountRepositoryProvider = Provider<IBankAccountRepository>((ref) {
  final apiClient = ref.watch(apiClientProvider);
  final logger = ref.watch(loggerServiceProvider);
  return BankAccountRepositoryImpl(apiClient, logger: logger);
});

/// Stream provider for user's bank accounts
///
/// Watches all bank accounts for a given user ID.
/// Used for seller payout bank account management.
final bankAccountsStreamProvider =
    StreamProvider.family<Result<List<BankAccountEntity>>, String>((
      ref,
      userId,
    ) {
      final repository = ref.watch(bankAccountRepositoryProvider);
      return repository.watchBankAccounts(userId);
    });

/// Provider to get primary bank account
final primaryBankAccountProvider =
    FutureProvider.family<Result<BankAccountEntity?>, String>((
      ref,
      userId,
    ) async {
      final repository = ref.read(bankAccountRepositoryProvider);
      return repository.getPrimaryBankAccount(userId);
    });
