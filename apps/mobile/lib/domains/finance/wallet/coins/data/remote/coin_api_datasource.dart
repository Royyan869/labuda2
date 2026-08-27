/// Coin API Datasource (Go Backend)
///
/// READ-ONLY data transport layer for Labuda Coins (loyalty points).
///
/// AUTHORITY MODEL:
///   - Mobile NEVER earns or spends coins directly.
///   - Earn:   backend-only via order completion worker.
///   - Spend:  backend-only via checkout order creation (useCoins flag only).
///   - Refund: backend-only via coins.refund_required outbox worker.
///
/// Active endpoints (auth-based — no userId in path/query):
///   GET  /api/v1/coins/balance       — current balance + lifetime stats
///   GET  /api/v1/coins/transactions  — paginated transaction history
library;

import 'package:dio/dio.dart';
import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/core/src/interfaces/services/i_logger_service.dart';
import '../dto/coin_dto.dart';

/// Remote datasource for Labuda Coins using Go backend.
///
/// All operations are READ-ONLY.  The backend identifies the caller via the
/// auth token — no userId is passed as a URL path param or query param.
class CoinApiDatasource {
  final ApiClient _apiClient;
  final ILoggerService? _logger;

  static const String _basePath = '/coins';

  CoinApiDatasource(this._apiClient, {ILoggerService? logger})
    : _logger = logger;

  // ============================================================
  // HELPER
  // ============================================================

  Future<Result<T>> _executeRequest<T>({
    required Future<Response<dynamic>> Function() request,
    required T Function(dynamic data) parser,
  }) async {
    try {
      final response = await request();
      final data = response.data;

      if (data is Map<String, dynamic>) {
        if (data['success'] == false && data['error'] != null) {
          final error = data['error'] as Map<String, dynamic>?;
          return Result.error(
            error?['message']?.toString() ?? 'Request failed',
          );
        }
        final parsedData = data['data'] ?? data;
        return Result.success(parser(parsedData));
      }

      return Result.success(parser(data));
    } on DioException catch (e) {
      final exception = _apiClient.extractException(e);
      _logger?.error(
        'API request failed: ${exception.message}',
        extra: {'code': exception.code, 'statusCode': exception.statusCode},
      );
      return Result.error(exception.message);
    } catch (e, stackTrace) {
      _logger?.error('Unexpected error', stackTrace: stackTrace);
      return Result.error('An unexpected error occurred');
    }
  }

  // ============================================================
  // Balance  —  GET /api/v1/coins/balance
  // ============================================================

  /// Returns the authenticated user's current coin balance.
  ///
  /// The [userId] parameter is accepted for interface compatibility only;
  /// the backend identifies the caller from the auth token.
  Future<Result<CoinBalanceDto>> getBalance(String userId) async {
    return _executeRequest<CoinBalanceDto>(
      request: () => _apiClient.get('$_basePath/balance'),
      parser: (data) => CoinBalanceDto.fromJson(data as Map<String, dynamic>),
    );
  }

  /// Polls balance every 30 seconds for reactive UI updates.
  Stream<CoinBalanceDto> watchBalance(String userId) {
    return Stream.periodic(const Duration(seconds: 30), (_) => userId).asyncMap(
      (_) async {
        final result = await getBalance(userId);
        return result.fold(
          (error) => CoinBalanceDto.empty(userId),
          (balance) => balance,
        );
      },
    );
  }

  // ============================================================
  // Transactions  —  GET /api/v1/coins/transactions
  // ============================================================

  /// Returns paginated transaction history for the authenticated user.
  ///
  /// The [userId] parameter is accepted for interface compatibility only;
  /// the backend identifies the caller from the auth token.
  Future<Result<List<CoinTransactionDto>>> getTransactions({
    required String userId,
    int limit = 50,
    int offset = 0,
  }) async {
    return _executeRequest<List<CoinTransactionDto>>(
      request: () => _apiClient.get(
        '$_basePath/transactions',
        queryParameters: {
          'limit': limit.toString(),
          'offset': offset.toString(),
        },
      ),
      parser: (data) {
        final list = data as List<dynamic>;
        return list
            .map((e) => CoinTransactionDto.fromJson(e as Map<String, dynamic>))
            .toList();
      },
    );
  }

  /// Polls transactions every 30 seconds for reactive UI updates.
  Stream<List<CoinTransactionDto>> watchTransactions({
    required String userId,
    int limit = 50,
  }) {
    return Stream.periodic(const Duration(seconds: 30), (_) => userId).asyncMap(
      (_) async {
        final result = await getTransactions(userId: userId, limit: limit);
        return result.fold(
          (error) => <CoinTransactionDto>[],
          (transactions) => transactions,
        );
      },
    );
  }
}
