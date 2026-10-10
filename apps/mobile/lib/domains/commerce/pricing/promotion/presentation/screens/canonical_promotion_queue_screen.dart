library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/commerce/pricing/promotion/data/dto/promotion_contract_dto.dart';
import 'package:hishumi/domains/commerce/pricing/promotion/presentation/providers/canonical_promotion_providers.dart';
import 'package:hishumi/shared/widgets/app_dialog.dart';
import 'package:hishumi/shared/widgets/app_snackbar.dart';

/// Canonical "Kelola Produk" queue screen for ONE existing promotion contract.
///
/// AUTHORITY BOUNDARY:
/// - The queue is read from GET /promotions/contracts/:id/targets and rendered
///   verbatim. The backend owns every queue rule (minimum 1, maximum 10,
///   duplicates, kind, ownership, eligibility).
/// - "Tambah Produk" appends through POST /promotions/contracts/:id/targets
///   (refill). It NEVER creates a promotion, funding intent, or payment.
/// - After every mutation the queue provider is invalidated and re-read, so the
///   UI renders server truth — never optimistic local state.
class CanonicalPromotionQueueScreen extends ConsumerStatefulWidget {
  final String contractId;
  final String kind; // internal | external

  const CanonicalPromotionQueueScreen({
    super.key,
    required this.contractId,
    required this.kind,
  });

  @override
  ConsumerState<CanonicalPromotionQueueScreen> createState() =>
      _CanonicalPromotionQueueScreenState();
}

class _CanonicalPromotionQueueScreenState
    extends ConsumerState<CanonicalPromotionQueueScreen> {
  bool _busy = false;

  /// UX mirror of the backend queue capacity (final authority is the backend).
  static const int _maxTargets = 10;

  String _typeLabel(String type) {
    switch (type) {
      case 'for_sale':
        return 'For Sale';
      case 'auction':
        return 'Lelang';
      case 'external_product':
        return 'Eksternal';
      default:
        return type;
    }
  }

  /// Surfaces the backend's canonical queue error codes as seller-facing
  /// messages; never converts a rejection into a generic success.
  String _errorMessage(Result<dynamic> result, String fallback) {
    switch (result.errorCode) {
      case 'QUEUE_FULL':
        return 'Antrian penuh (maksimal $_maxTargets produk).';
      case 'QUEUE_DUPLICATE':
        return 'Produk sudah ada di antrian.';
      case 'QUEUE_KIND_MISMATCH':
        return 'Jenis produk tidak sesuai dengan promosi ini.';
      case 'PROMOTION_ALREADY_FINALIZED':
        return 'Promosi sudah selesai; antrian tidak dapat diubah.';
      case 'CONTRACT_NOT_OWNED':
        return 'Anda tidak memiliki promosi ini.';
      case 'CONTRACT_NOT_FOUND':
        return 'Promosi tidak ditemukan.';
    }
    return result.error ?? fallback;
  }

  Future<void> _addTarget(List<PromotionContractTargetDto> current) async {
    if (_busy || current.length >= _maxTargets) return;
    final pick = ref.read(promotionProductPickerProvider);
    final selection = await pick(context, ref, widget.kind);
    if (!mounted || selection == null) return;
    // UX duplicate guard; the backend remains the final authority.
    if (current.any((t) => t.targetId == selection.targetId)) {
      AppSnackBar.showWarning(context, 'Produk sudah ada di antrian');
      return;
    }
    setState(() => _busy = true);
    final result = await ref
        .read(promotionContractRepositoryProvider)
        .addTarget(
          contractId: widget.contractId,
          targetType: selection.targetType,
          targetId: selection.targetId,
        );
    if (!mounted) return;
    setState(() => _busy = false);
    if (result.isSuccess) {
      AppSnackBar.showSuccess(context, 'Produk ditambahkan ke antrian');
      // Render server truth: re-read the canonical queue.
      ref.invalidate(promotionTargetsProvider(widget.contractId));
    } else {
      AppSnackBar.showError(
        context,
        _errorMessage(result, 'Gagal menambah produk'),
      );
    }
  }

  Future<void> _removeTarget(PromotionContractTargetDto target) async {
    if (_busy) return;
    final confirmed = await AppDialog.confirm(
      context: context,
      title: 'Hapus produk dari antrian?',
      message: 'Produk akan dikeluarkan dari antrian promosi ini.',
      confirmLabel: 'Hapus',
      cancelLabel: 'Batal',
    );
    if (!confirmed || !mounted) return;
    setState(() => _busy = true);
    final result = await ref
        .read(promotionContractRepositoryProvider)
        .removeTarget(contractId: widget.contractId, targetId: target.targetId);
    if (!mounted) return;
    setState(() => _busy = false);
    if (result.isSuccess) {
      AppSnackBar.showSuccess(context, 'Produk dihapus dari antrian');
      ref.invalidate(promotionTargetsProvider(widget.contractId));
    } else {
      AppSnackBar.showError(
        context,
        _errorMessage(result, 'Gagal menghapus produk'),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final async = ref.watch(promotionTargetsProvider(widget.contractId));
    final targets = async.asData?.value.data?.targets;
    final full = (targets?.length ?? 0) >= _maxTargets;

    return Scaffold(
      appBar: AppBar(title: const Text('Kelola Produk')),
      floatingActionButton: FloatingActionButton.extended(
        onPressed: (_busy || targets == null || full)
            ? null
            : () => _addTarget(targets),
        icon: const Icon(Icons.add),
        label: Text(full ? 'Antrian penuh' : 'Tambah Produk'),
      ),
      // SAFE-AREA-38: the body owns the LIVE system bottom inset exactly
      // once — /seller/canonical-promotions/:contractId/queue is a FLAT
      // top-level GoRoute (SellerModule), so no shell bar owns it. The
      // queue uses an EXPLICIT `ListView.padding` (design p16), which
      // disables BoxScrollView's window-padding auto-consumption, so no
      // other widget in the body can own the bottom region. `top: false`:
      // the Scaffold AppBar owns the status-bar region (the body slot's
      // top is already below the bar). The extended FAB is positioned by
      // the Scaffold endFloat layout — an authority outside this wrapper.
      body: SafeArea(
        top: false,
        child: async.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (error, _) => _messageState(
            context,
            title: 'Gagal Memuat Antrian',
            message: error.toString(),
          ),
          data: (result) {
            if (result.isError || result.data == null) {
              return _messageState(
                context,
                title: 'Gagal Memuat Antrian',
                message: result.error ?? 'Gagal memuat antrian',
              );
            }
            final list = result.data!.targets;
            if (list.isEmpty) {
              return _messageState(
                context,
                title: 'Antrian kosong',
                message: 'Tambahkan minimal satu produk untuk promosi ini.',
              );
            }
            return ListView.separated(
              padding: const EdgeInsets.all(AppMetrics.p16),
              itemCount: list.length,
              separatorBuilder: (_, _) => const Divider(height: 1),
              itemBuilder: (context, index) {
                final t = list[index];
                return ListTile(
                  contentPadding: EdgeInsets.zero,
                  leading: CircleAvatar(
                    radius: 12,
                    child: Text('${index + 1}'),
                  ),
                  title: Text(_typeLabel(t.targetType)),
                  subtitle: Text(
                    t.targetId,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                  trailing: IconButton(
                    icon: const Icon(Icons.delete_outline),
                    onPressed: _busy ? null : () => _removeTarget(t),
                  ),
                );
              },
            );
          },
        ),
      ),
    );
  }

  Widget _messageState(
    BuildContext context, {
    required String title,
    required String message,
  }) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(AppMetrics.p24),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              Icons.inventory_2_outlined,
              size: AppIconSize.display,
              color: Theme.of(context).colorScheme.outline,
            ),
            const SizedBox(height: 16),
            Text(
              title,
              style: context.typeRoles.titleProminent.copyWith(
                fontWeight: FontWeight.bold,
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
          ],
        ),
      ),
    );
  }
}
