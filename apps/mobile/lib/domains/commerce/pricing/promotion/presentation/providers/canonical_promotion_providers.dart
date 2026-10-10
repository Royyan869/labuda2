/// Canonical Promotion Providers
///
/// Riverpod providers for the canonical seller promotion management list.
/// The single list authority is promotion_contracts via GET /promotions/contracts.
/// The legacy /promotions/my surface is purged.
library;

import 'package:flutter/widgets.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/commerce/pricing/promotion/data/dto/promotion_contract_dto.dart';
import 'package:hishumi/domains/commerce/pricing/promotion/data/repositories/promotion_contract_repository.dart';
import 'package:hishumi/domains/commerce/pricing/promotion/domain/entities/external_product.dart';
import 'package:hishumi/domains/commerce/pricing/promotion/presentation/providers/canonical_external_product_providers.dart';
import 'package:hishumi/domains/social/comment/presentation/widgets/commerce_resource_picker.dart';
import 'package:hishumi/shared/providers/auth_status_providers.dart';
import 'package:hishumi/shared/widgets/app_bottom_sheet_list_selection.dart';

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

/// Canonical persisted rolling queue for one promotion contract.
///
/// This is a READ projection of the backend authority
/// (promotion_contract_targets). After any queue mutation the provider is
/// invalidated and re-read, so the UI renders server truth — never optimistic
/// local state.
final promotionTargetsProvider = FutureProvider.autoDispose
    .family<Result<PromotionTargetListDto>, String>((ref, contractId) async {
      final repo = ref.watch(promotionContractRepositoryProvider);
      return repo.listTargets(contractId);
    });

/// Reusable promotion funding provider — canonical PROMOTE_BALANCE projection
/// via GET /promote-balance.
///
/// This is the ONLY mobile authority for "how much reusable promotion funding
/// does the seller have". It is a read-only projection of the ledger; the
/// client never computes or mutates the balance. Locked PROMOTION_ALLOCATION of
/// an active promotion is deliberately NOT part of this number — unused
/// allocation only becomes reusable after canonical finalization.
final promoteBalanceProvider =
    FutureProvider.autoDispose<Result<PromoteBalanceDto>>((ref) async {
      final repo = ref.watch(promotionContractRepositoryProvider);
      return repo.getPromoteBalance();
    });

/// One product chosen for the promotion queue, with a display title.
class PromotionProductSelection {
  final String targetType; // for_sale | auction | external_product
  final String targetId;
  final String title;

  const PromotionProductSelection({
    required this.targetType,
    required this.targetId,
    required this.title,
  });
}

/// Product picker boundary for the canonical create flow.
///
/// This is a UI boundary, NOT a business authority: the backend owns every
/// queue rule (minimum 1, maximum 10, kind, ownership, eligibility, scheduled
/// Auction). The production implementation composes the existing canonical
/// product sources — the shared commerce picker (For Sale + Auction) for
/// internal promotions and the existing external-product provider for external
/// promotions. Tests inject a deterministic selection.
typedef PromotionProductPicker =
    Future<PromotionProductSelection?> Function(
      BuildContext context,
      WidgetRef ref,
      String kind,
    );

final promotionProductPickerProvider = Provider<PromotionProductPicker>((ref) {
  return (context, ref, kind) async {
    if (kind == 'external') {
      final result = await ref.read(myExternalProductsProvider.future);
      final approved = (result.data ?? const <ExternalProduct>[])
          .where((p) => p.isApproved)
          .toList();
      if (approved.isEmpty || !context.mounted) return null;
      final selected =
          await AppBottomSheetListSelection.showListSelection<ExternalProduct>(
            context: context,
            title: 'Pilih Produk Eksternal',
            items: approved
                .map(
                  (p) => ListSelectionItem<ExternalProduct>(
                    title: p.title,
                    subtitle: p.externalUrl,
                    value: p,
                  ),
                )
                .toList(),
          );
      if (selected == null) return null;
      return PromotionProductSelection(
        targetType: 'external_product',
        targetId: selected.id,
        title: selected.title,
      );
    }
    // Internal: the shared commerce picker returns one For Sale / Auction.
    final selection = await CommerceResourcePicker.show(
      context,
      sellerId: ref.read(currentUserIdProvider),
    );
    if (selection == null) return null;
    return PromotionProductSelection(
      targetType: selection.resource.resourceType.wireValue,
      targetId: selection.resource.resourceId,
      title: selection.title,
    );
  };
});
