library;

import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/pricing/promotion/data/dto/promotion_contract_dto.dart';

abstract class PromotionContractRepository {
  Future<Result<PromotionContractListDto>> listMyContracts();
  Future<Result<PromotionContractDto>> getContract(String contractId);
  Future<Result<PromotionContractDto>> createContract({
    required String kind,
    required int budgetRupiah,
    required int durationDays,
    required List<String> cityIds,
  });

  /// GET /promote-balance — the seller's reusable PROMOTE_BALANCE projection.
  Future<Result<PromoteBalanceDto>> getPromoteBalance();

  /// POST /promotions/contracts/payment-intent — the canonical exact-shortage
  /// funding obligation for a proposed promotion.
  ///
  /// This is the single mobile funding gate: the backend decides whether a
  /// payment is required and for exactly how much. When the seller's reusable
  /// PROMOTE_BALANCE already covers the cost, the backend creates nothing (no
  /// intent, no billing) and reports `payment_required: false`.
  Future<Result<PromotionFundingIntentDto>> createFundingIntent({
    required String kind,
    required int budgetRupiah,
    required int durationDays,
    required List<String> cityIds,
  });

  /// GET /promotions/contracts/payment-intent/:id/payment-methods — read-only
  /// disclosure of the enabled payment methods with the backend-calculated fee
  /// and gross for THIS obligation. There is no client-side fee calculation and
  /// no fallback method list: if this fails, the picker fails.
  Future<Result<PromotionFundingPaymentMethodsDto>> getFundingPaymentMethods(
    String intentId,
  );

  /// POST /promotions/contracts/payment-intent/:id/pay — initiates the payment
  /// for this obligation with the seller-selected canonical method code.
  /// Returns the gateway redirect URL for the canonical payment WebView.
  Future<Result<PromotionFundingPaymentDto>> initiateFundingPayment({
    required String intentId,
    required String paymentMethodCode,
  });

  Future<Result<void>> pauseContract(String contractId);
  Future<Result<void>> resumeContract(String contractId);
  Future<Result<void>> finalizeContract(String contractId);
}

class PromotionContractRepositoryImpl implements PromotionContractRepository {
  final ApiClient _apiClient;
  PromotionContractRepositoryImpl(this._apiClient);

  @override
  Future<Result<PromotionContractListDto>> listMyContracts() async {
    try {
      final res = await _apiClient.get('/promotions/contracts');
      final data = res.data?['data'] as Map<String, dynamic>?;
      if (data == null) return Result.error('Invalid response');
      return Result.success(PromotionContractListDto.fromJson(data));
    } on ApiException catch (e) {
      return Result.error(e.message);
    } catch (e) {
      return Result.error(e.toString());
    }
  }

  @override
  Future<Result<PromotionContractDto>> getContract(String contractId) async {
    try {
      final res = await _apiClient.get('/promotions/contracts/$contractId');
      final data = res.data?['data']?['contract'] as Map<String, dynamic>?;
      if (data == null) return Result.error('Invalid response');
      return Result.success(PromotionContractDto.fromJson(data));
    } on ApiException catch (e) {
      return Result.error(e.message);
    } catch (e) {
      return Result.error(e.toString());
    }
  }

  @override
  Future<Result<PromotionContractDto>> createContract({
    required String kind,
    required int budgetRupiah,
    required int durationDays,
    required List<String> cityIds,
  }) async {
    try {
      final req = CreatePromotionContractRequestDto(
        kind: kind,
        budgetRupiah: budgetRupiah,
        durationDays: durationDays,
        cityIds: cityIds,
      );
      final res = await _apiClient.post(
        '/promotions/contracts',
        data: req.toJson(),
      );
      final data = res.data?['data']?['contract'] as Map<String, dynamic>?;
      if (data == null) return Result.error('Invalid response');
      return Result.success(PromotionContractDto.fromJson(data));
    } on ApiException catch (e) {
      return Result.error(e.message);
    } catch (e) {
      return Result.error(e.toString());
    }
  }

  @override
  Future<Result<PromoteBalanceDto>> getPromoteBalance() async {
    try {
      final res = await _apiClient.get('/promote-balance');
      final data = res.data?['data'] as Map<String, dynamic>?;
      if (data == null) return Result.error('Invalid response');
      return Result.success(PromoteBalanceDto.fromJson(data));
    } on ApiException catch (e) {
      return Result.error(e.message);
    } catch (e) {
      return Result.error(e.toString());
    }
  }

  @override
  Future<Result<PromotionFundingIntentDto>> createFundingIntent({
    required String kind,
    required int budgetRupiah,
    required int durationDays,
    required List<String> cityIds,
  }) async {
    try {
      final req = CreatePromotionContractRequestDto(
        kind: kind,
        budgetRupiah: budgetRupiah,
        durationDays: durationDays,
        cityIds: cityIds,
      );
      final res = await _apiClient.post(
        '/promotions/contracts/payment-intent',
        data: req.toJson(),
      );
      final data = res.data?['data']?['intent'] as Map<String, dynamic>?;
      if (data == null) return Result.error('Invalid response');
      return Result.success(PromotionFundingIntentDto.fromJson(data));
    } on ApiException catch (e) {
      return _apiError(e);
    } catch (e) {
      return Result.error(e.toString());
    }
  }

  @override
  Future<Result<PromotionFundingPaymentMethodsDto>> getFundingPaymentMethods(
    String intentId,
  ) async {
    try {
      final res = await _apiClient.get(
        '/promotions/contracts/payment-intent/$intentId/payment-methods',
      );
      final data = res.data?['data'] as Map<String, dynamic>?;
      if (data == null) return Result.error('Invalid response');
      return Result.success(PromotionFundingPaymentMethodsDto.fromJson(data));
    } on ApiException catch (e) {
      return _apiError(e);
    } catch (e) {
      return Result.error(e.toString());
    }
  }

  @override
  Future<Result<PromotionFundingPaymentDto>> initiateFundingPayment({
    required String intentId,
    required String paymentMethodCode,
  }) async {
    try {
      final res = await _apiClient.post(
        '/promotions/contracts/payment-intent/$intentId/pay',
        data: {'payment_method_code': paymentMethodCode},
      );
      final data = res.data?['data'] as Map<String, dynamic>?;
      if (data == null) return Result.error('Invalid response');
      return Result.success(PromotionFundingPaymentDto.fromJson(data));
    } on ApiException catch (e) {
      return _apiError(e);
    } catch (e) {
      return Result.error(e.toString());
    }
  }

  /// Preserves the backend's machine-readable code and HTTP status so callers
  /// can distinguish a missing intent (404), another seller's intent (403), an
  /// invalid/non-pending obligation (409) and a transport failure.
  Result<T> _apiError<T>(ApiException e) =>
      Result.error(e.message, code: e.code, statusCode: e.statusCode);

  @override
  Future<Result<void>> pauseContract(String contractId) async {
    try {
      await _apiClient.post('/promotions/contracts/$contractId/pause');
      return Result.success(null);
    } on ApiException catch (e) {
      return Result.error(e.message);
    } catch (e) {
      return Result.error(e.toString());
    }
  }

  @override
  Future<Result<void>> resumeContract(String contractId) async {
    try {
      await _apiClient.post('/promotions/contracts/$contractId/resume');
      return Result.success(null);
    } on ApiException catch (e) {
      return Result.error(e.message);
    } catch (e) {
      return Result.error(e.toString());
    }
  }

  @override
  Future<Result<void>> finalizeContract(String contractId) async {
    try {
      await _apiClient.post('/promotions/contracts/$contractId/finalize');
      return Result.success(null);
    } on ApiException catch (e) {
      return Result.error(e.message);
    } catch (e) {
      return Result.error(e.toString());
    }
  }
}
