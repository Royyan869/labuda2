/// Canonical Promotion Providers
///
/// Riverpod providers for the canonical seller promotion management list.
/// The single list authority is promotion_contracts via GET /promotions/contracts.
/// The legacy /promotions/my surface is purged.
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/pricing/promotion/data/dto/promotion_contract_dto.dart';
import 'package:labuda/domains/commerce/pricing/promotion/data/repositories/promotion_contract_repository.dart';

/// Promotion Contract repository provider — canonical authority promotion_contracts.
final promotionContractRepositoryProvider =
    Provider<PromotionContractRepository>((ref) {
  final apiClient = ref.watch(apiClientProvider);
  return PromotionContractRepositoryImpl(apiClient);
});

/// My promotion contracts provider — canonical list via GET /promotions/contracts.
final myPromotionContractsProvider =
    FutureProvider.autoDispose<Result<PromotionContractListDto>>((ref) async {
  final repo = ref.watch(promotionContractRepositoryProvider);
  return repo.listMyContracts();
});