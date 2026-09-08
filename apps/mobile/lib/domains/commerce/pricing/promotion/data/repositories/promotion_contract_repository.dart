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
      final res = await _apiClient.post('/promotions/contracts', data: req.toJson());
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
