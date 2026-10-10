/// Canonical Promotion List Screen
///
/// Seller canonical promotion management list. Loads the authenticated
/// seller's own promotion contracts from GET /api/v1/promotions/contracts
/// (promotion_contracts authority) and exposes ONE explicit action per item:
/// "Lihat Analitik", which navigates to the canonical analytics route with
/// the canonical contract ID. This screen is deliberately NOT the legacy
/// promotion instance list, packages, or ownership surface.
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/shared/widgets/app_dialog.dart';
import 'package:hishumi/shared/widgets/app_snackbar.dart';
import 'package:hishumi/domains/commerce/pricing/promotion/data/dto/promotion_contract_dto.dart';
import 'package:hishumi/domains/commerce/pricing/promotion/data/repositories/promotion_contract_repository.dart';
import 'package:hishumi/domains/commerce/pricing/promotion/presentation/providers/canonical_promotion_providers.dart';
import 'package:hishumi/shared/utils/app_formatters.dart';

/// Screen that lists the authenticated seller's promotion contracts.
///
/// States are truthful: loading, data (empty collection is a valid
/// successful state), and error with a working Retry. Metrics (analytics)
/// are never shown here — the list endpoint does not carry them; analytics
/// appear only after the seller opens the existing analytics screen.
class CanonicalPromotionListScreen extends ConsumerWidget {
  const CanonicalPromotionListScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final promotionsAsync = ref.watch(myPromotionContractsProvider);

    return Scaffold(
      appBar: AppBar(title: const Text('Kelola Promosi')),
      floatingActionButton: FloatingActionButton.extended(
        onPressed: () => context.push(RoutePaths.sellerPromotionContractCreate),
        icon: const Icon(Icons.add),
        label: const Text('Buat Promosi'),
      ),
      // SAFE-AREA-37: the body owns the LIVE system bottom inset exactly
      // once — /seller/canonical-promotions is a FLAT top-level GoRoute
      // (SellerModule), so no shell bar owns it. The list carries an
      // EXPLICIT design padding (p16), which disables BoxScrollView's
      // window-padding auto-consumption, so no other widget in the body can
      // own the bottom region. `top: false`: the Scaffold AppBar owns the
      // status-bar region (the body slot's top padding is already removed
      // for it). The extended FAB is positioned by the Scaffold endFloat
      // layout — an authority outside this wrapper.
      body: SafeArea(
        top: false,
        child: promotionsAsync.when(
          data: (result) {
            if (result.isSuccess) {
              final list = result.data!;
              if (list.contracts.isEmpty) {
                return _buildEmptyState(context);
              }
              return _buildList(context, list.contracts);
            }
            return _buildErrorState(
              context,
              ref,
              result.error ?? 'Gagal memuat promosi',
            );
          },
          loading: () => _buildLoadingState(),
          error: (error, _) => _buildErrorState(context, ref, error.toString()),
        ),
      ),
    );
  }

  Widget _buildLoadingState() {
    return const Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          CircularProgressIndicator(),
          SizedBox(height: 16),
          Text('Memuat promosi...'),
        ],
      ),
    );
  }

  Widget _buildEmptyState(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(AppMetrics.p24),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              Icons.campaign_outlined,
              size: AppIconSize.display,
              color: Theme.of(context).colorScheme.outline,
            ),
            const SizedBox(height: 16),
            Text(
              'Belum ada promosi',
              style: context.typeRoles.titleProminent.copyWith(
                fontWeight: FontWeight.bold,
                color: Theme.of(context).colorScheme.onSurface,
              ),
            ),
            const SizedBox(height: 8),
            Text(
              'Promosi canonical Anda akan muncul di sini.',
              textAlign: TextAlign.center,
              style: context.typeRoles.bodyDense.copyWith(
                color: Theme.of(context).colorScheme.onSurfaceVariant,
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildErrorState(BuildContext context, WidgetRef ref, String message) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(AppMetrics.p24),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              Icons.error_outline,
              size: AppIconSize.display,
              color: Theme.of(context).colorScheme.primary,
            ),
            const SizedBox(height: 16),
            Text(
              'Gagal Memuat Promosi',
              style: context.typeRoles.titleProminent.copyWith(
                fontWeight: FontWeight.bold,
                color: Theme.of(context).colorScheme.onSurface,
              ),
            ),
            const SizedBox(height: 8),
            Text(
              message,
              textAlign: TextAlign.center,
              style: context.typeRoles.bodyDense.copyWith(
                color: Theme.of(context).colorScheme.onSurfaceVariant,
              ),
            ),
            const SizedBox(height: 24),
            ElevatedButton(
              onPressed: () => ref.invalidate(myPromotionContractsProvider),
              child: const Text('Coba Lagi'),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildList(
    BuildContext context,
    List<PromotionContractDto> contracts,
  ) {
    return ListView.separated(
      padding: const EdgeInsets.all(AppMetrics.p16),
      itemCount: contracts.length,
      separatorBuilder: (context, index) => const SizedBox(height: 12),
      itemBuilder: (context, index) {
        return _PromotionListItem(contract: contracts[index]);
      },
    );
  }
}

/// One canonical promotion contract row.
///
/// Lifecycle actions map 1:1 to the canonical owner endpoints:
///   active  → POST /promotions/contracts/:id/pause    (Jeda)
///   paused  → POST /promotions/contracts/:id/resume   (Lanjutkan)
///   active|paused → POST /promotions/contracts/:id/finalize (Hentikan)
/// A finalized contract offers no actions. Stopping releases the remaining
/// PROMOTION_ALLOCATION back to the reusable PROMOTE_BALANCE, so the reusable
/// funding provider is invalidated afterwards.
class _PromotionListItem extends ConsumerStatefulWidget {
  final PromotionContractDto contract;

  const _PromotionListItem({required this.contract});

  @override
  ConsumerState<_PromotionListItem> createState() => _PromotionListItemState();
}

class _PromotionListItemState extends ConsumerState<_PromotionListItem> {
  bool _busy = false;

  PromotionContractDto get contract => widget.contract;

  Future<void> _run(
    Future<Result<void>> Function(PromotionContractRepository repo) action,
    String successMessage,
  ) async {
    setState(() => _busy = true);
    final repo = ref.read(promotionContractRepositoryProvider);
    final result = await action(repo);
    if (!mounted) return;
    setState(() => _busy = false);
    if (result.isSuccess) {
      AppSnackBar.showSuccess(context, successMessage);
      ref.invalidate(myPromotionContractsProvider);
      // Finalization releases unused allocation back to the reusable balance.
      ref.invalidate(promoteBalanceProvider);
    } else {
      AppSnackBar.showError(context, result.error ?? 'Aksi gagal');
    }
  }

  Future<void> _confirmStop() async {
    final confirmed = await AppDialog.confirm(
      context: context,
      title: 'Hentikan promosi?',
      message:
          'Promosi berhenti sekarang dan sisa anggaran yang belum terpakai '
          'kembali ke saldo promo Anda.',
      confirmLabel: 'Hentikan',
      cancelLabel: 'Batal',
    );
    if (!confirmed) return;
    await _run(
      (repo) => repo.finalizeContract(contract.id),
      'Promosi dihentikan',
    );
  }

  @override
  Widget build(BuildContext context) {
    final isNationwide = contract.cityIds.isEmpty;
    return Container(
      padding: const EdgeInsets.all(AppMetrics.p16),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(AppShape.r12),
        border: Border.all(color: Theme.of(context).colorScheme.outlineVariant),
        color: Theme.of(context).colorScheme.surface,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(
                child: Text(
                  contract.kind == 'internal' ? 'Internal' : 'Eksternal',
                  style: context.typeRoles.titleCompact.copyWith(
                    fontWeight: FontWeight.w700,
                  ),
                ),
              ),
              Text(
                _statusLabel(contract.status),
                style: context.typeRoles.labelMicro.copyWith(
                  fontWeight: FontWeight.w600,
                  color: _statusColor(context, contract.status),
                ),
              ),
            ],
          ),
          const SizedBox(height: 8),
          Text(
            'Budget: ${AppFormatters.formatCurrencyInt(contract.budgetRupiah)}',
          ),
          Text('CPM: ${AppFormatters.formatCurrencyInt(contract.cpmRupiah)}'),
          Text(
            'Geografi: ${isNationwide ? 'Nasional' : contract.cityIds.join(', ')}',
          ),
          Text(
            'Dibuat: ${AppFormatters.formatDate(DateTime.parse(contract.createdAt))}',
          ),
          const SizedBox(height: 12),
          // Refill: the rolling product queue can be managed while the
          // promotion is not finalized (the backend rejects finalized
          // contracts; this is a UX mirror, not the authority).
          if (contract.status != 'finalized')
            SizedBox(
              width: double.infinity,
              child: OutlinedButton.icon(
                onPressed: () {
                  context.push(
                    '${RoutePaths.sellerCanonicalPromotionQueuePath(contract.id)}'
                    '?kind=${contract.kind}',
                  );
                },
                icon: const Icon(
                  Icons.inventory_2_outlined,
                  size: AppIconSize.action,
                ),
                label: const Text('Kelola Produk'),
              ),
            ),
          if (contract.status != 'finalized') const SizedBox(height: 8),
          SizedBox(
            width: double.infinity,
            child: OutlinedButton.icon(
              onPressed: () {
                context.push(
                  RoutePaths.sellerCanonicalPromotionAnalyticsPath(contract.id),
                );
              },
              icon: const Icon(
                Icons.analytics_outlined,
                size: AppIconSize.action,
              ),
              label: const Text('Lihat Analitik'),
            ),
          ),
          ..._lifecycleActions(context),
        ],
      ),
    );
  }

  /// Canonical owner lifecycle actions for this contract's status.
  ///
  /// active → Jeda (pause) + Hentikan (finalize)
  /// paused → Lanjutkan (resume) + Hentikan (finalize)
  /// finalized/other → none (terminal; no action can release or re-spend)
  List<Widget> _lifecycleActions(BuildContext context) {
    switch (contract.status) {
      case 'active':
        return [
          const SizedBox(height: 8),
          Row(
            children: [
              Expanded(
                child: OutlinedButton(
                  onPressed: _busy
                      ? null
                      : () => _run(
                          (repo) => repo.pauseContract(contract.id),
                          'Promosi dijeda',
                        ),
                  child: const Text('Jeda'),
                ),
              ),
              const SizedBox(width: 8),
              Expanded(
                child: OutlinedButton(
                  onPressed: _busy ? null : _confirmStop,
                  child: const Text('Hentikan'),
                ),
              ),
            ],
          ),
        ];
      case 'paused':
        return [
          const SizedBox(height: 8),
          Row(
            children: [
              Expanded(
                child: OutlinedButton(
                  onPressed: _busy
                      ? null
                      : () => _run(
                          (repo) => repo.resumeContract(contract.id),
                          'Promosi dilanjutkan',
                        ),
                  child: const Text('Lanjutkan'),
                ),
              ),
              const SizedBox(width: 8),
              Expanded(
                child: OutlinedButton(
                  onPressed: _busy ? null : _confirmStop,
                  child: const Text('Hentikan'),
                ),
              ),
            ],
          ),
        ];
      default:
        return const [];
    }
  }

  /// Canonical lifecycle labels.
  ///
  /// Authority: promotion_contract_status_enum — exactly
  /// prepared | active | paused | finalizing | finalized. The legacy
  /// created/funded/eligible/cancelled/failed vocabulary belonged to the
  /// competing `promotions` aggregate purged in migration 000081 and must
  /// never reappear here.
  static String _statusLabel(String status) {
    switch (status) {
      case 'prepared':
        return 'Disiapkan';
      case 'active':
        return 'Aktif';
      case 'paused':
        return 'Dijeda';
      case 'finalizing':
        return 'Finalisasi';
      case 'finalized':
        return 'Selesai';
      default:
        return status;
    }
  }

  static Color _statusColor(BuildContext context, String status) {
    switch (status) {
      case 'active':
        return context.statusColors.success;
      case 'paused':
        return context.statusColors.info;
      case 'finalized':
        return Theme.of(context).colorScheme.outline;
      default:
        return context.statusColors.info;
    }
  }
}
