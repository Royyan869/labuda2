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
    final state = ref.watch(shippingNotifierProvider);

    return Scaffold(
      appBar: AppBar(
        title: const Text('Pengiriman'),
        backgroundColor: AppColors.primaryRed,
        foregroundColor: Colors.white,
      ),
      floatingActionButton: FloatingActionButton.extended(
        onPressed: _openCreateSetup,
        backgroundColor: AppColors.primaryRed,
        icon: const Icon(Icons.add),
        label: const Text('Tambah Opsi'),
      ),
      body: RefreshIndicator(
        onRefresh: () async => _reload(),
        child: _buildBody(state),
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
        padding: const EdgeInsets.fromLTRB(16, 16, 16, 96),
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
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogCtx) => AlertDialog(
        title: const Text('Hapus Opsi Pengiriman'),
        content: Text(
          'Hapus "${opt.displayName}" dari daftar opsi pengiriman Anda? '
          'Jika opsi ini masih dipakai di ForSale atau Auction, penghapusan '
          'akan ditolak — matikan lewat tombol aktif sebagai gantinya.',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(dialogCtx).pop(false),
            child: const Text('Batal'),
          ),
          ElevatedButton(
            onPressed: () => Navigator.of(dialogCtx).pop(true),
            style: ElevatedButton.styleFrom(
              backgroundColor: AppColors.error,
              foregroundColor: Colors.white,
            ),
            child: const Text('Hapus'),
          ),
        ],
      ),
    );
    if (confirmed != true || !mounted) return;
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
    final isDark = Theme.of(context).brightness == Brightness.dark;
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: AppColors.primaryBlue.withValues(alpha: 0.08),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: AppColors.primaryBlue.withValues(alpha: 0.25),
        ),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Icon(
            Icons.info_outline,
            size: 18,
            color: AppColors.primaryBlue,
          ),
          const SizedBox(width: 8),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  ShippingHonestyMessages.sellerManagedShipping,
                  style: TextStyle(
                    fontSize: 13,
                    fontWeight: FontWeight.w600,
                    color: isDark
                        ? AppColors.neutralWhite
                        : AppColors.neutralGray900,
                  ),
                ),
                const SizedBox(height: 4),
                Text(
                  'Tentukan sendiri opsi pengiriman sesuai ekspedisi langganan '
                  'Anda: pilih minimal satu provinsi tujuan beserta tarifnya. '
                  'Input biaya pengiriman beserta biaya packing jika ada.',
                  style: TextStyle(
                    fontSize: 12,
                    color: AppColors.neutralGray600,
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
    return ListView(
      physics: const AlwaysScrollableScrollPhysics(),
      padding: const EdgeInsets.fromLTRB(24, 80, 24, 24),
      children: [
        const _HonestyBanner(),
        const SizedBox(height: 32),
        Icon(
          Icons.local_shipping_outlined,
          size: 72,
          color: AppColors.neutralGray400,
        ),
        const SizedBox(height: 16),
        const Text(
          'Belum Ada Opsi Pengiriman',
          textAlign: TextAlign.center,
          style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold),
        ),
        const SizedBox(height: 8),
        Text(
          'Buat satu paket opsi pengiriman: jenis ekspedisi, nama ekspedisi, '
          'tujuan provinsi beserta tarif (termasuk packing), dan catatan '
          'pribadi untuk Anda. ForSale baru wajib memilih minimal satu opsi '
          'sebelum bisa dipublish.',
          textAlign: TextAlign.center,
          style: TextStyle(fontSize: 14, color: AppColors.neutralGray600),
        ),
        const SizedBox(height: 24),
        ElevatedButton.icon(
          onPressed: onCreate,
          style: ElevatedButton.styleFrom(
            backgroundColor: AppColors.primaryRed,
            foregroundColor: Colors.white,
            padding: const EdgeInsets.symmetric(vertical: 14),
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
    return ListView(
      physics: const AlwaysScrollableScrollPhysics(),
      padding: const EdgeInsets.all(24),
      children: [
        const SizedBox(height: 80),
        Icon(Icons.error_outline, size: 64, color: AppColors.error),
        const SizedBox(height: 16),
        const Text(
          'Gagal memuat opsi pengiriman',
          textAlign: TextAlign.center,
          style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold),
        ),
        const SizedBox(height: 8),
        Text(
          message,
          textAlign: TextAlign.center,
          style: TextStyle(fontSize: 13, color: AppColors.neutralGray600),
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
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final note = option.internalNote?.trim() ?? '';
    return Card(
      elevation: 0,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(12),
        side: BorderSide(
          color: isDark ? AppColors.darkGray700 : AppColors.neutralGray200,
        ),
      ),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(12),
        child: Padding(
          padding: const EdgeInsets.fromLTRB(12, 12, 4, 12),
          child: Row(
            children: [
              CircleAvatar(
                radius: 22,
                backgroundColor: AppColors.primaryRed.withValues(alpha: 0.1),
                child: Text(option.emoji, style: const TextStyle(fontSize: 22)),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      option.displayName,
                      style: const TextStyle(
                        fontSize: 15,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    const SizedBox(height: 2),
                    Text(
                      '${option.type.label}'
                      ' · ${option.coverageAreas.length} provinsi',
                      style: TextStyle(
                        fontSize: 12,
                        color: AppColors.neutralGray600,
                      ),
                    ),
                    // Seller-private note: visible ONLY on seller surfaces.
                    if (note.isNotEmpty) ...[
                      const SizedBox(height: 2),
                      Text(
                        'Catatan: $note',
                        style: TextStyle(
                          fontSize: 11,
                          fontStyle: FontStyle.italic,
                          color: AppColors.neutralGray500,
                        ),
                      ),
                    ],
                  ],
                ),
              ),
              Switch(
                value: option.isActive,
                onChanged: onToggle,
                activeThumbColor: AppColors.primaryRed,
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
