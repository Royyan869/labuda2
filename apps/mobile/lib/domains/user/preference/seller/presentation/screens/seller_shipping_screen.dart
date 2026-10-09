/// Seller Shipping Setup Screen
///
/// Lets a seller manage their **global** shipping options (the seller-wide
/// catalog of shipping methods). ForSales later select a subset of these
/// options at create/edit time.
///
/// ONE-PACKAGE CONTRACT (Owner-locked): create and edit always route to the
/// canonical setup screen ([RoutePaths.sellerShippingSetup]) where the option
/// identity (jenis ekspedisi, nama ekspedisi, catatan privat seller) and its
/// destinations (provinsi + tarif all-in ongkir+packing + kualifikasi kota)
/// are authored and saved as ONE unit. The old metadata-only bottom-sheet
/// form (name + type) is a killed design and must not be reintroduced.
///
/// Reuses the existing data layer entirely:
///   - [shippingNotifierProvider] for the options list + CRUD
///   - [ShippingRepository] under the hood
///   - [ShippingHonestyMessages] for canonical UX copy
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/domains/commerce/transaction/shipping/domain/domain.dart';
import 'package:labuda/domains/commerce/transaction/shipping/presentation/providers/providers.dart';
import 'package:labuda/domains/commerce/transaction/shipping/presentation/providers/shipping_state.dart';
import 'package:labuda/domains/commerce/transaction/shipping/presentation/utils/shipping_honesty_messages.dart';

class SellerShippingScreen extends ConsumerStatefulWidget {
  const SellerShippingScreen({super.key});

  @override
  ConsumerState<SellerShippingScreen> createState() =>
      _SellerShippingScreenState();
}

class _SellerShippingScreenState extends ConsumerState<SellerShippingScreen> {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _reload());
  }

  void _reload() {
    final authState = ref.read(authControllerProvider);
    if (authState is! AuthStateAuthenticated) return;
    ref.read(shippingNotifierProvider.notifier).loadShippingSetups();
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final state = ref.watch(shippingNotifierProvider);

    return Scaffold(
      appBar: AppBar(title: const Text('Pengiriman')),
      floatingActionButton: FloatingActionButton.extended(
        onPressed: _openCreateSetup,
        backgroundColor: scheme.primary,
        icon: const Icon(Icons.add),
        label: const Text('Tambah Opsi'),
      ),
      // Canonical bar-less screen (SAFE-AREA-10): `SafeArea` is the LIVE
      // system-inset authority for this body — the explicit ListView padding
      // below would otherwise disable ScrollView's automatic MediaQuery
      // padding. FAB overlay clearance sits ABOVE this inset
      // (`contentEnd = systemInset + fabClearance`).
      body: SafeArea(
        child: RefreshIndicator(
          onRefresh: () async => _reload(),
          child: _buildBody(state),
        ),
      ),
    );
  }

  Widget _buildBody(ShippingSetupsListState state) {
    if (state is ShippingSetupsListLoading ||
        state is ShippingSetupsListInitial) {
      return const Center(child: CircularProgressIndicator());
    }
    if (state is ShippingSetupsListError) {
      return _ErrorView(message: state.message, onRetry: _reload);
    }
    if (state is ShippingSetupsListLoaded) {
      if (state.options.isEmpty) {
        return _EmptyView(onCreate: _openCreateSetup);
      }
      return ListView.separated(
        physics: const AlwaysScrollableScrollPhysics(),
        // Bottom = FAB overlay clearance only (SAFE-AREA-10): the body
        // `SafeArea` above this list owns the live system inset, so this
        // constant no longer doubles as an inset stand-in.
        padding: const EdgeInsets.fromLTRB(
          AppMetrics.p16,
          AppMetrics.p16,
          AppMetrics.p16,
          AppMetrics.fabClearance,
        ),
        itemCount: state.options.length + 1,
        separatorBuilder: (_, _) => const SizedBox(height: 8),
        itemBuilder: (context, index) {
          if (index == 0) return const _HonestyBanner();
          final opt = state.options[index - 1];
          return _OptionRow(
            option: opt,
            onTap: () => _openEditSetup(opt),
            onToggle: (v) => _toggleActive(opt, v),
            onEdit: () => _openEditSetup(opt),
            onDelete: () => _confirmDelete(opt),
          );
        },
      );
    }
    return const SizedBox.shrink();
  }

  /// Create mode — canonical one-package setup screen.
  Future<void> _openCreateSetup() async {
    final result = await context.push<ShippingSetup>(
      RoutePaths.sellerShippingSetup,
    );
    if (!mounted) return;
    if (result != null) {
      AppSnackBar.showSuccess(context, 'Opsi pengiriman ditambahkan.');
    }
    _reload();
  }

  /// Edit mode — hydrate the canonical setup screen from the option ID.
  Future<void> _openEditSetup(ShippingSetup opt) async {
    final result = await context.push<ShippingSetup>(
      RoutePaths.sellerShippingSetup,
      extra: opt.id,
    );
    if (!mounted) return;
    if (result != null) {
      AppSnackBar.showSuccess(context, 'Opsi pengiriman diperbarui.');
    }
    _reload();
  }

  Future<void> _toggleActive(ShippingSetup opt, bool isActive) async {
    final ok = await ref
        .read(shippingNotifierProvider.notifier)
        .toggleActiveStatus(opt.id, isActive);
    if (!mounted) return;
    if (ok) {
      _reload();
    } else {
      final s = ref.read(shippingNotifierProvider);
      final msg = s is ShippingSetupsListError
          ? s.message
          : 'Gagal mengubah status opsi pengiriman.';
      AppSnackBar.showError(context, msg);
    }
  }

  Future<void> _confirmDelete(ShippingSetup opt) async {
    // F9(a) convergence: pure destructive yes/no decision consumes the
    // canonical AppDialog.confirm grammar (same order, tone, bool contract).
    final confirmed = await AppDialog.confirm(
      context: context,
      title: 'Hapus Opsi Pengiriman',
      message:
          'Hapus "${opt.displayName}" dari daftar opsi pengiriman Anda? '
          'Jika opsi ini masih dipakai di ForSale atau Auction, penghapusan '
          'akan ditolak — matikan lewat tombol aktif sebagai gantinya.',
      confirmLabel: 'Hapus',
      cancelLabel: 'Batal',
      intent: AppDialogIntent.destructive,
    );
    if (!confirmed || !mounted) return;
    final ok = await ref
        .read(shippingNotifierProvider.notifier)
        .deleteShippingSetup(opt.id);
    if (!mounted) return;
    if (ok) {
      AppSnackBar.showSuccess(context, 'Opsi pengiriman dihapus.');
      _reload();
    } else {
      final s = ref.read(shippingNotifierProvider);
      final msg = s is ShippingSetupsListError
          ? s.message
          : 'Opsi tidak dapat dihapus karena masih ter-link ke listing. '
                'Matikan opsi ini sebagai gantinya.';
      AppSnackBar.showError(context, msg);
    }
  }
}

// =============================================================================
// EMPTY / ERROR / HONESTY BANNERS
// =============================================================================

class _HonestyBanner extends StatelessWidget {
  const _HonestyBanner();

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.all(AppMetrics.p12),
      decoration: BoxDecoration(
        color: scheme.secondary.withValues(alpha: 0.08),
        borderRadius: BorderRadius.circular(AppShape.r12),
        border: Border.all(color: scheme.secondary.withValues(alpha: 0.25)),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(
            Icons.info_outline,
            size: AppIconSize.action,
            color: scheme.secondary,
          ),
          const SizedBox(width: 8),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  ShippingHonestyMessages.sellerManagedShipping,
                  style: context.typeRoles.bodyDense.copyWith(
                    fontWeight: FontWeight.w600,
                    color: scheme.onSurface,
                  ),
                ),
                const SizedBox(height: 4),
                Text(
                  'Tentukan sendiri opsi pengiriman sesuai ekspedisi langganan '
                  'Anda: pilih minimal satu provinsi tujuan beserta tarifnya. '
                  'Input biaya pengiriman beserta biaya packing jika ada.',
                  style: context.typeRoles.labelMicro.copyWith(
                    color: scheme.onSurfaceVariant,
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _EmptyView extends StatelessWidget {
  final VoidCallback onCreate;
  const _EmptyView({required this.onCreate});

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return ListView(
      physics: const AlwaysScrollableScrollPhysics(),
      // Same FAB clearance authority as the loaded list — no second
      // magic bottom value in this screen (SAFE-AREA-10).
      padding: const EdgeInsets.fromLTRB(
        AppMetrics.p24,
        AppMetrics.p48,
        AppMetrics.p24,
        AppMetrics.fabClearance,
      ),
      children: [
        const _HonestyBanner(),
        const SizedBox(height: 32),
        Icon(
          Icons.local_shipping_outlined,
          size: AppIconSize.display,
          color: scheme.onSurfaceVariant,
        ),
        const SizedBox(height: 16),
        Text(
          'Belum Ada Opsi Pengiriman',
          textAlign: TextAlign.center,
          style: context.typeRoles.titleSection.copyWith(
            fontWeight: FontWeight.bold,
          ),
        ),
        const SizedBox(height: 8),
        Text(
          'Buat satu paket opsi pengiriman: jenis ekspedisi, nama ekspedisi, '
          'tujuan provinsi beserta tarif (termasuk packing), dan catatan '
          'pribadi untuk Anda. ForSale baru wajib memilih minimal satu opsi '
          'sebelum bisa dipublish.',
          textAlign: TextAlign.center,
          style: context.typeRoles.bodyDense.copyWith(
            color: scheme.onSurfaceVariant,
          ),
        ),
        const SizedBox(height: 24),
        ElevatedButton.icon(
          onPressed: onCreate,
          style: ElevatedButton.styleFrom(
            padding: const EdgeInsets.symmetric(vertical: AppMetrics.p16),
          ),
          icon: const Icon(Icons.add),
          label: const Text('Tambah Opsi Pengiriman'),
        ),
      ],
    );
  }
}

class _ErrorView extends StatelessWidget {
  final String message;
  final VoidCallback onRetry;
  const _ErrorView({required this.message, required this.onRetry});

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return ListView(
      physics: const AlwaysScrollableScrollPhysics(),
      // L/R/top = visual gutter; bottom = FAB overlay clearance, the SAME
      // authority as the loaded/empty states (SAFE-AREA-11) — the body
      // `SafeArea` above owns the live system inset.
      padding: const EdgeInsets.fromLTRB(
        AppMetrics.p24,
        AppMetrics.p24,
        AppMetrics.p24,
        AppMetrics.fabClearance,
      ),
      children: [
        const SizedBox(height: 80),
        Icon(
          Icons.error_outline,
          size: AppIconSize.display,
          color: context.statusColors.error,
        ),
        const SizedBox(height: 16),
        Text(
          'Gagal memuat opsi pengiriman',
          textAlign: TextAlign.center,
          style: context.typeRoles.titleSection.copyWith(
            fontWeight: FontWeight.bold,
          ),
        ),
        const SizedBox(height: 8),
        Text(
          message,
          textAlign: TextAlign.center,
          style: context.typeRoles.bodyDense.copyWith(
            color: scheme.onSurfaceVariant,
          ),
        ),
        const SizedBox(height: 24),
        ElevatedButton(onPressed: onRetry, child: const Text('Coba Lagi')),
      ],
    );
  }
}

// =============================================================================
// LIST ROW
// =============================================================================

class _OptionRow extends StatelessWidget {
  final ShippingSetup option;
  final VoidCallback onTap;
  final ValueChanged<bool> onToggle;
  final VoidCallback onEdit;
  final VoidCallback onDelete;

  const _OptionRow({
    required this.option,
    required this.onTap,
    required this.onToggle,
    required this.onEdit,
    required this.onDelete,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final note = option.internalNote?.trim() ?? '';
    return Card(
      elevation: AppElevation.none,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(AppShape.r12),
        side: BorderSide(color: scheme.outlineVariant),
      ),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(AppShape.r12),
        child: Padding(
          padding: const EdgeInsets.fromLTRB(
            AppMetrics.p12,
            AppMetrics.p12,
            AppMetrics.p4,
            AppMetrics.p12,
          ),
          child: Row(
            children: [
              CircleAvatar(
                radius: 22,
                backgroundColor: scheme.primary.withValues(alpha: 0.1),
                child: Text(
                  option.emoji,
                  style: context.typeRoles.titleProminent,
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      option.displayName,
                      style: context.typeRoles.titleCompact.copyWith(
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    const SizedBox(height: 2),
                    Text(
                      '${option.type.label}'
                      ' · ${option.coverageAreas.length} provinsi',
                      style: context.typeRoles.labelMicro.copyWith(
                        color: scheme.onSurfaceVariant,
                      ),
                    ),
                    // Seller-private note: visible ONLY on seller surfaces.
                    if (note.isNotEmpty) ...[
                      const SizedBox(height: 2),
                      Text(
                        'Catatan: $note',
                        style: context.typeRoles.labelMicro.copyWith(
                          fontStyle: FontStyle.italic,
                          color: scheme.onSurfaceVariant,
                        ),
                      ),
                    ],
                  ],
                ),
              ),
              Switch(
                value: option.isActive,
                onChanged: onToggle,
              ),
              PopupMenuButton<String>(
                itemBuilder: (_) => const [
                  PopupMenuItem(value: 'edit', child: Text('Edit paket')),
                  PopupMenuItem(value: 'delete', child: Text('Hapus')),
                ],
                onSelected: (v) {
                  if (v == 'edit') onEdit();
                  if (v == 'delete') onDelete();
                },
              ),
            ],
          ),
        ),
      ),
    );
  }
}
