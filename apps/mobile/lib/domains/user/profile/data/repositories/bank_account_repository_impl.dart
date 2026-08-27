import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/profile/domain/entities/bank_account_entity.dart';
import 'package:labuda/domains/user/profile/domain/repositories/i_bank_account_repository.dart';

/// Implementation of IBankAccountRepository using the REST API.
///
/// Backend routes:
///   GET    /api/v1/bank-accounts            → list seller's accounts
///   POST   /api/v1/bank-accounts            → create
///   DELETE /api/v1/bank-accounts/:id        → soft-delete
///   PATCH  /api/v1/bank-accounts/:id/default → set default
///
/// Note: No PUT/update route. No /primary or /default GET route.
class BankAccountRepositoryImpl implements IBankAccountRepository {
  final ApiClient _apiClient;
  final ILoggerService? _logger;

  BankAccountRepositoryImpl(this._apiClient, {ILoggerService? logger})
    : _logger = logger;

  @override
  Stream<Result<List<BankAccountEntity>>> watchBankAccounts(String userId) {
    return Stream.fromFuture(_fetchAll());
  }

  Future<Result<List<BankAccountEntity>>> _fetchAll() async {
    try {
      final response = await _apiClient.get('/bank-accounts');
      final data = response.data as Map<String, dynamic>;
      final list = data['data'] as List<dynamic>?;

      if (list == null) {
        return Result.success(<BankAccountEntity>[]);
      }

      final accounts = list
          .map((e) => BankAccountEntity.fromMap(e as Map<String, dynamic>))
          .toList();
      return Result.success(accounts);
    } catch (e) {
      _logger?.error(
        'Failed to get bank accounts',
        extra: {'error': e.toString()},
      );
      return Result.error('Gagal memuat rekening bank');
    }
  }

  @override
  Future<Result<BankAccountEntity>> addBankAccount(
    BankAccountEntity account,
  ) async {
    try {
      final response = await _apiClient.post(
        '/bank-accounts',
        data: account.toMap(),
      );
      final body = response.data as Map<String, dynamic>;
      final newAccount = BankAccountEntity.fromMap(
        body['data'] as Map<String, dynamic>,
      );
      return Result.success(newAccount);
    } catch (e) {
      _logger?.error(
        'Failed to add bank account',
        extra: {'error': e.toString()},
      );
      return Result.error('Gagal menambahkan rekening bank');
    }
  }

  @override
  Future<Result<void>> deleteBankAccount(String accountId) async {
    try {
      await _apiClient.delete('/bank-accounts/$accountId');
      return Result.success(null);
    } catch (e) {
      _logger?.error(
        'Failed to delete bank account',
        extra: {'error': e.toString()},
      );
      return Result.error('Gagal menghapus rekening bank');
    }
  }

  @override
  Future<Result<void>> setPrimaryBankAccount(
    String accountId,
    String userId,
  ) async {
    try {
      await _apiClient.patch('/bank-accounts/$accountId/default');
      return Result.success(null);
    } catch (e) {
      _logger?.error(
        'Failed to set default bank account',
        extra: {'error': e.toString()},
      );
      return Result.error('Gagal mengatur rekening utama');
    }
  }

  @override
  Future<Result<BankAccountEntity?>> getPrimaryBankAccount(
    String userId,
  ) async {
    final result = await _fetchAll();
    if (result.isError) {
      return Result.error(result.error ?? 'Gagal memuat rekening bank');
    }
    final primary = result.data?.where((a) => a.isDefault).firstOrNull;
    return Result.success(primary);
  }
}
