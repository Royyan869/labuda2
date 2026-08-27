/// Create Discount Use Case
library;

import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/pricing/discount/domain/entities/discount_entity.dart';
import 'package:labuda/domains/commerce/pricing/discount/domain/repositories/i_discount_repository.dart';

class CreateDiscountUseCase {
  final IDiscountRepository _repository;

  CreateDiscountUseCase(this._repository);

  Future<Result<Discount>> call(CreateDiscountParams params) async {
    try {
      final existingDiscountResult = await _repository.getDiscountByCode(
        params.code,
      );

      if (existingDiscountResult.isSuccess) {
        final existing = existingDiscountResult.data!;
        if (existing.sellerId == params.sellerId) {
          return Result.error('Kode diskon "${params.code}" sudah digunakan');
        }
      }

      if (params.validUntil.isBefore(params.validFrom) ||
          params.validUntil.isAtSameMomentAs(params.validFrom)) {
        return Result.error('Tanggal selesai harus setelah tanggal mulai');
      }

      if (params.type == DiscountType.percentage) {
        if (params.value <= 0 || params.value > 100) {
          return Result.error('Nilai diskon persentase harus antara 1-100');
        }
      } else if (params.type == DiscountType.flatAmount) {
        if (params.value <= 0) {
          return Result.error('Nilai diskon harus lebih dari 0');
        }
      }

      if (params.sellerId == null) {
        return Result.error('Seller ID harus diisi untuk diskon');
      }

      final resolvedTargetMode = params.targetMode;
      final resolvedListingIds = params.applicableListingIds;
      final resolvedAuctionIds = params.applicableAuctionIds;
      final resolvedAppliesTo = params.appliesTo;

      if (resolvedTargetMode == DiscountTargetMode.selectedItems &&
          (resolvedListingIds?.isEmpty ?? true) &&
          (resolvedAuctionIds?.isEmpty ?? true)) {
        return Result.error(
          'Pilih minimal satu listing atau auction untuk diskon selected_items',
        );
      }

      final now = DateTime.now();
      final discount = Discount(
        id: '',
        code: params.code.toUpperCase(),
        description: params.description,
        type: params.type,
        value: params.value,
        minPurchase: params.minPurchase,
        maxDiscount: params.maxDiscount,
        maxUsagePerUser: params.maxUsagePerUser,
        totalUsageLimit: params.totalUsageLimit,
        appliesTo: resolvedAppliesTo,
        targetMode: resolvedTargetMode,
        sellerId: params.sellerId,
        applicableListingIds: resolvedListingIds,
        applicableAuctionIds: resolvedAuctionIds,
        validFrom: params.validFrom,
        validUntil: params.validUntil,
        isActive: params.isActive ?? true,
        currentUsageCount: 0,
        createdAt: now,
        createdBy: params.createdBy,
      );

      return await _repository.createDiscount(discount);
    } catch (e) {
      return Result.error('Gagal membuat diskon: ${e.toString()}');
    }
  }
}

class CreateDiscountParams {
  final String code;
  final String description;
  final DiscountType type;
  final double value;
  final double? minPurchase;
  final double? maxDiscount;
  final int? maxUsagePerUser;
  final int? totalUsageLimit;
  final DiscountAppliesTo appliesTo;
  final DiscountTargetMode targetMode;
  final String? sellerId;
  final List<String>? applicableListingIds;
  final List<String>? applicableAuctionIds;
  final DateTime validFrom;
  final DateTime validUntil;
  final bool? isActive;
  final String createdBy;

  const CreateDiscountParams({
    required this.code,
    required this.description,
    required this.type,
    required this.value,
    this.minPurchase,
    this.maxDiscount,
    this.maxUsagePerUser,
    this.totalUsageLimit,
    required this.appliesTo,
    required this.targetMode,
    this.sellerId,
    this.applicableListingIds,
    this.applicableAuctionIds,
    required this.validFrom,
    required this.validUntil,
    this.isActive,
    required this.createdBy,
  });
}
