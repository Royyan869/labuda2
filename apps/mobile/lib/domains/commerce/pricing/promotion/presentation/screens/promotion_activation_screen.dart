library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:intl/intl.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/auction_providers.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction_status.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/for_sale_providers.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/external_product.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/instance_status.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/promotion_instance.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/promotion_ownership.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/target_type.dart';
import 'package:labuda/domains/commerce/pricing/promotion/presentation/providers/promotion_providers.dart';
import 'package:labuda/domains/commerce/pricing/promotion/presentation/screens/promotion_package_selection_screen.dart';

class PromotionActivationScreen extends ConsumerStatefulWidget {
  final TargetType? preselectedTargetType;
  final String? preselectedTargetId;
  final String? preselectedTargetTitle;
  final String? preselectedOwnershipId;
  final String? reassignInstanceId;

  const PromotionActivationScreen({
    super.key,
    this.preselectedTargetType,
    this.preselectedTargetId,
    this.preselectedTargetTitle,
    this.preselectedOwnershipId,
    this.reassignInstanceId,
  });

  @override
  ConsumerState<PromotionActivationScreen> createState() =>
      _PromotionActivationScreenState();
}

class _PromotionActivationScreenState
    extends ConsumerState<PromotionActivationScreen> {
  String? _selectedOwnershipId;
  _TargetOption? _selectedTarget;
  bool _isSubmitting = false;

  bool get _isReassignMode => widget.reassignInstanceId != null;

  @override
  void initState() {
    super.initState();
    _selectedOwnershipId = widget.preselectedOwnershipId;
    if (widget.preselectedTargetType != null &&
        widget.preselectedTargetId != null) {
      _selectedTarget = _TargetOption(
        type: widget.preselectedTargetType!,
        id: widget.preselectedTargetId!,
        title: widget.preselectedTargetTitle ?? 'Selected target',
        status: null,
        imageUrl: null,
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final authState = ref.watch(authControllerProvider);
    if (authState is! AuthStateAuthenticated) {
      return const Scaffold(
        body: Center(child: Text('Please login to activate promotion')),
      );
    }

    final sellerId = authState.user.id;
    final ownershipsAsync = ref.watch(myOwnershipsProvider);
    final instancesAsync = ref.watch(myInstancesProvider);
    final listingsAsync = ref.watch(
      sellerForSalesProvider(SellerForSalesParams(sellerId: sellerId)),
    );
    final auctionsAsync = ref.watch(
      _sellerPromotableAuctionsProvider(sellerId),
    );
    final externalProductsAsync = ref.watch(myExternalProductsProvider);

    return Scaffold(
      appBar: AppBar(
        title: Text(
          _isReassignMode ? 'Reassign Promotion' : 'Activate Promotion',
        ),
      ),
      body: ownershipsAsync.when(
        data: (ownershipsResult) => instancesAsync.when(
          data: (instancesResult) {
            if (!ownershipsResult.isSuccess || !instancesResult.isSuccess) {
              return _error(
                ownershipsResult.error ??
                    instancesResult.error ??
                    'Failed to load activation data',
              );
            }
            final ownerships = ownershipsResult.data ?? <PromotionOwnership>[];
            final instances = instancesResult.data ?? <PromotionInstance>[];
            final usableOwnerships = _filterUsableOwnerships(
              ownerships,
              instances,
            );
            PromotionOwnership? selectedOwnership;
            for (final ownership in usableOwnerships) {
              if (ownership.id == _selectedOwnershipId) {
                selectedOwnership = ownership;
                break;
              }
            }

            if (_selectedOwnershipId != null && selectedOwnership == null) {
              _selectedOwnershipId = null;
            }

            return Column(
              children: [
                Expanded(
                  child: ListView(
                    padding: const EdgeInsets.all(16),
                    children: [
                      const Text(
                        'Choose Ownership',
                        style: TextStyle(
                          fontSize: 16,
                          fontWeight: FontWeight.w700,
                        ),
                      ),
                      const SizedBox(height: 8),
                      if (usableOwnerships.isEmpty)
                        _ownershipEmptyState(context),
                      if (usableOwnerships.isNotEmpty)
                        ...usableOwnerships.map(
                          (ownership) => _OwnershipCard(
                            ownership: ownership,
                            isSelected: _selectedOwnershipId == ownership.id,
                            onTap: _isReassignMode
                                ? () {}
                                : () => setState(
                                    () => _selectedOwnershipId = ownership.id,
                                  ),
                          ),
                        ),
                      const SizedBox(height: 16),
                      const Text(
                        'Choose Target',
                        style: TextStyle(
                          fontSize: 16,
                          fontWeight: FontWeight.w700,
                        ),
                      ),
                      const SizedBox(height: 8),
                      listingsAsync.when(
                        data: (listings) => auctionsAsync.when(
                          data: (auctions) => externalProductsAsync.when(
                            data: (externalResult) {
                              final approvedProducts =
                                  (externalResult.isSuccess
                                      ? externalResult.data
                                      : null) ??
                                  <ExternalProduct>[];
                              final targets = _buildTargets(
                                listings,
                                auctions,
                                approvedProducts
                                    .where((p) => p.isApproved)
                                    .toList(),
                              );
                              if (targets.isEmpty) {
                                return const Padding(
                                  padding: EdgeInsets.all(8),
                                  child: Text('No promotable target found'),
                                );
                              }
                              return Column(
                                children: [
                                  ...targets.map(
                                    (target) => _TargetCard(
                                      target: target,
                                      isSelected:
                                          _selectedTarget?.id == target.id &&
                                          _selectedTarget?.type == target.type,
                                      onTap: () => setState(
                                        () => _selectedTarget = target,
                                      ),
                                    ),
                                  ),
                                  const SizedBox(height: 8),
                                  Align(
                                    alignment: Alignment.centerLeft,
                                    child: TextButton.icon(
                                      onPressed: () => context.push(
                                        RoutePaths.sellerExternalProducts,
                                      ),
                                      icon: const Icon(
                                        Icons.open_in_new,
                                        size: 16,
                                      ),
                                      label: const Text(
                                        'Manage External Products',
                                      ),
                                    ),
                                  ),
                                ],
                              );
                            },
                            loading: () => const Padding(
                              padding: EdgeInsets.all(8),
                              child: CircularProgressIndicator(),
                            ),
                            error: (e, _) =>
                                const Text('Data belum bisa dimuat.'),
                          ),
                          loading: () => const Padding(
                            padding: EdgeInsets.all(8),
                            child: CircularProgressIndicator(),
                          ),
                          error: (e, _) =>
                              const Text('Data belum bisa dimuat.'),
                        ),
                        loading: () => const Padding(
                          padding: EdgeInsets.all(8),
                          child: CircularProgressIndicator(),
                        ),
                        error: (e, _) => const Text('Data belum bisa dimuat.'),
                      ),
                    ],
                  ),
                ),
                SafeArea(
                  top: false,
                  child: Padding(
                    padding: const EdgeInsets.fromLTRB(16, 0, 16, 16),
                    child: SizedBox(
                      width: double.infinity,
                      child: ElevatedButton(
                        onPressed:
                            _canActivate(usableOwnerships) && !_isSubmitting
                            ? _submit
                            : null,
                        child: _isSubmitting
                            ? const CircularProgressIndicator()
                            : Text(
                                _isReassignMode
                                    ? 'Reassign Promotion'
                                    : 'Activate Promotion',
                              ),
                      ),
                    ),
                  ),
                ),
              ],
            );
          },
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (e, _) => const Center(child: Text('Data belum bisa dimuat.')),
        ),
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (e, _) => _error(e.toString()),
      ),
    );
  }

  Widget _ownershipEmptyState(BuildContext context) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: AppColors.neutralGray200),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text('No promotion package available'),
          const SizedBox(height: 8),
          ElevatedButton(
            onPressed: () => _openPackagePurchase(context),
            child: const Text('Buy Promotion Package'),
          ),
        ],
      ),
    );
  }

  Widget _error(String message) => Center(child: Text(message));

  bool _canActivate(List<PromotionOwnership> usableOwnerships) {
    if (_selectedOwnershipId == null || _selectedTarget == null) return false;
    return usableOwnerships.any((o) => o.id == _selectedOwnershipId);
  }

  List<PromotionOwnership> _filterUsableOwnerships(
    List<PromotionOwnership> ownerships,
    List<PromotionInstance> instances,
  ) {
    final activeOwnershipIds = instances
        .where((i) => i.status == InstanceStatus.active)
        .map((i) => i.ownershipId)
        .toSet();
    var usable = ownerships
        .where(
          (o) =>
              o.canActivate &&
              o.remainingDurationHours > 0 &&
              !activeOwnershipIds.contains(o.id),
        )
        .toList();
    if (_isReassignMode && widget.preselectedOwnershipId != null) {
      usable = usable
          .where((o) => o.id == widget.preselectedOwnershipId)
          .toList();
    }
    return usable;
  }

  List<_TargetOption> _buildTargets(
    List<ForSale> listings,
    List<Auction> auctions,
    List<ExternalProduct> approvedProducts,
  ) {
    final fixedPriceSaleTargets = listings
        .where((l) => l.status == ForSaleStatus.active)
        .map(
          (l) => _TargetOption(
            type: TargetType.forSale,
            id: l.forSaleId,
            title: l.title,
            status: l.status.displayName,
            imageUrl: l.media.isEmpty ? null : l.media.first.originalUrl,
          ),
        );
    final auctionTargets = auctions
        .where(
          (a) =>
              a.status == AuctionStatus.active ||
              a.status == AuctionStatus.scheduled,
        )
        .map(
          (a) => _TargetOption(
            type: TargetType.auction,
            id: a.id,
            title: a.title,
            status: a.status.displayName,
            imageUrl: a.media.isEmpty ? null : a.media.first.originalUrl,
          ),
        );
    final externalTargets = approvedProducts.map(
      (p) => _TargetOption(
        type: TargetType.externalProduct,
        id: p.id,
        title: p.title,
        status: 'Approved',
        imageUrl: p.media.isNotEmpty ? p.media.first.url : null,
      ),
    );
    return [...fixedPriceSaleTargets, ...auctionTargets, ...externalTargets];
  }

  Future<void> _submit() async {
    if (_selectedOwnershipId == null || _selectedTarget == null) return;
    setState(() => _isSubmitting = true);
    final controller = ref.read(promotionControllerProvider);
    final result = _isReassignMode
        ? await controller.reassignPromotion(
            instanceId: widget.reassignInstanceId!,
            newTargetType: _selectedTarget!.type,
            newTargetId: _selectedTarget!.id,
          )
        : await controller.activatePromotion(
            ownershipId: _selectedOwnershipId!,
            targetType: _selectedTarget!.type,
            targetId: _selectedTarget!.id,
          );
    if (!mounted) return;
    setState(() => _isSubmitting = false);
    if (!result.isSuccess) {
      if (CommerceRestrictionPresenter.isCommerceRestricted(result.errorCode)) {
        CommerceRestrictionPresenter.show(
          context,
          actionDescription: 'mengaktifkan promosi',
        );
        return;
      }
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(result.error ?? 'Activation failed'),
          backgroundColor: AppColors.primaryRed,
        ),
      );
      return;
    }
    ref.invalidate(myOwnershipsProvider);
    ref.invalidate(myInstancesProvider);
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(
          _isReassignMode ? 'Promotion reassigned' : 'Promotion activated',
        ),
      ),
    );
    context.go('${RoutePaths.sellerPromotions}/${result.data!.id}');
  }

  Future<void> _openPackagePurchase(BuildContext context) async {
    final targetType = _selectedTarget?.type ?? widget.preselectedTargetType;
    final targetId = _selectedTarget?.id ?? widget.preselectedTargetId;
    final fixedPriceSaleTitle =
        _selectedTarget?.title ?? widget.preselectedTargetTitle;
    final ownershipId = _selectedOwnershipId ?? widget.preselectedOwnershipId;

    final result = await Navigator.of(context)
        .push<PurchasePackageNavigationResult>(
          MaterialPageRoute(
            builder: (_) => PromotionPackageSelectionScreen(
              fixedPriceSaleId: targetType == TargetType.forSale
                  ? targetId
                  : null,
              fixedPriceSaleTitle: fixedPriceSaleTitle,
              returnToActivationOnSuccess: true,
              preselectedTargetType: targetType,
              preselectedTargetId: targetId,
              preselectedTargetTitle: fixedPriceSaleTitle,
              preselectedOwnershipId: ownershipId,
              reassignInstanceId: widget.reassignInstanceId,
            ),
          ),
        );

    if (!context.mounted) return;
    if (result?.message != null && result!.message!.isNotEmpty) {
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(result.message!)));
    }

    if (result?.goToActivation == true) {
      context.go(
        RoutePaths.sellerPromotionActivate,
        extra: {
          'preselectedTargetType': result?.preselectedTargetType,
          'preselectedTargetId': result?.preselectedTargetId,
          'preselectedTargetTitle': result?.preselectedTargetTitle,
          'preselectedOwnershipId': result?.preselectedOwnershipId,
          'reassignInstanceId': result?.reassignInstanceId,
        },
      );
    }
  }
}

final _sellerPromotableAuctionsProvider = FutureProvider.family
    .autoDispose<List<Auction>, String>((ref, sellerId) async {
      final repository = ref.watch(auctionRepositoryProvider);
      final activeResult = await repository.getUserAuctions(
        sellerId: sellerId,
        status: AuctionStatus.active,
        limit: 100,
      );
      final scheduledResult = await repository.getUserAuctions(
        sellerId: sellerId,
        status: AuctionStatus.scheduled,
        limit: 100,
      );

      final active = activeResult.fold((ok) => ok, (_) => <Auction>[]);
      final scheduled = scheduledResult.fold((ok) => ok, (_) => <Auction>[]);
      final byId = <String, Auction>{};
      for (final item in [...active, ...scheduled]) {
        byId[item.id] = item;
      }
      return byId.values.toList();
    });

class _TargetOption {
  final TargetType type;
  final String id;
  final String title;
  final String? status;
  final String? imageUrl;

  const _TargetOption({
    required this.type,
    required this.id,
    required this.title,
    this.status,
    this.imageUrl,
  });
}

class _OwnershipCard extends StatelessWidget {
  final PromotionOwnership ownership;
  final bool isSelected;
  final VoidCallback onTap;

  const _OwnershipCard({
    required this.ownership,
    required this.isSelected,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onTap,
      child: Container(
        margin: const EdgeInsets.only(bottom: 10),
        padding: const EdgeInsets.all(12),
        decoration: BoxDecoration(
          borderRadius: BorderRadius.circular(12),
          border: Border.all(
            color: isSelected ? AppColors.primaryRed : AppColors.neutralGray200,
            width: isSelected ? 2 : 1,
          ),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              ownership.packageId,
              style: const TextStyle(fontWeight: FontWeight.w700),
            ),
            const SizedBox(height: 4),
            Text('Remaining: ${ownership.remainingDurationHours}h'),
            Text(
              'Valid until: ${DateFormat('dd MMM yyyy').format(ownership.expiresAt)}',
            ),
          ],
        ),
      ),
    );
  }
}

class _TargetCard extends StatelessWidget {
  final _TargetOption target;
  final bool isSelected;
  final VoidCallback onTap;

  const _TargetCard({
    required this.target,
    required this.isSelected,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onTap,
      child: Container(
        margin: const EdgeInsets.only(bottom: 10),
        padding: const EdgeInsets.all(12),
        decoration: BoxDecoration(
          borderRadius: BorderRadius.circular(12),
          border: Border.all(
            color: isSelected ? AppColors.primaryRed : AppColors.neutralGray200,
            width: isSelected ? 2 : 1,
          ),
        ),
        child: Row(
          children: [
            Container(
              width: 52,
              height: 52,
              decoration: BoxDecoration(
                borderRadius: BorderRadius.circular(8),
                color: AppColors.neutralGray100,
              ),
              clipBehavior: Clip.antiAlias,
              child: target.imageUrl == null
                  ? const Icon(Icons.image_outlined)
                  : Image.network(target.imageUrl!, fit: BoxFit.cover),
            ),
            const SizedBox(width: 10),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    target.title,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(fontWeight: FontWeight.w700),
                  ),
                  const SizedBox(height: 3),
                  Text(
                    '${target.type.value} ${target.status == null ? '' : '• ${target.status}'}',
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}
