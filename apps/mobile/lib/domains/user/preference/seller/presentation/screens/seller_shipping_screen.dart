/// Seller Shipping Setup Screen
///
/// Lets a seller manage their **global** shipping options (the seller-wide
/// catalog of shipping methods). Listings later select a subset of these
/// options at create/edit time (Phase 2).
///
/// Reuses the existing data layer entirely:
///   - [shippingNotifierProvider] for the options list + CRUD
///   - [ShippingRepository] under the hood
///   - [ShippingHonestyMessages] for canonical UX copy
///   - [ShippingType] enum from the domain
///
/// Per-option province/city coverage is managed on the option-detail screen.
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
    ref.read(shippingNotifierProvider.notifier).loadShippingOptions();
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
        onPressed: _openCreateOptionSheet,
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

  Widget _buildBody(ShippingOptionsListState state) {
    if (state is ShippingOptionsListLoading ||
        state is ShippingOptionsListInitial) {
      return const Center(child: CircularProgressIndicator());
    }
    if (state is ShippingOptionsListError) {
      return _ErrorView(message: state.message, onRetry: _reload);
    }
    if (state is ShippingOptionsListLoaded) {
      if (state.options.isEmpty) {
        return _EmptyView(onCreate: _openCreateOptionSheet);
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
            onTap: () => _openOptionDetail(opt),
            onToggle: (v) => _toggleActive(opt, v),
            onEdit: () => _openEditOptionSheet(opt),
            onDelete: () => _confirmDelete(opt),
          );
        },
      );
    }
    return const SizedBox.shrink();
  }

  Future<void> _openCreateOptionSheet() async {
    final result = await showModalBottomSheet<_OptionFormResult>(
      context: context,
      isScrollControlled: true,
      builder: (_) => _OptionFormSheet(initial: null),
    );
    if (result == null || !mounted) return;
    final created = await ref
        .read(shippingNotifierProvider.notifier)
        .createShippingOption(
          CreateShippingOptionRequest(
            name: result.name,
            type: result.type,
            expeditionName: result.expeditionName,
          ),
        );
    if (!mounted) return;
    if (created != null) {
      AppSnackBar.showSuccess(context, 'Opsi pengiriman ditambahkan.');
      _reload();
    } else {
      final s = ref.read(shippingNotifierProvider);
      final msg = s is ShippingOptionsListError
          ? s.message
          : 'Gagal menambah opsi pengiriman.';
      AppSnackBar.showError(context, msg);
    }
  }

  Future<void> _openEditOptionSheet(ShippingOption opt) async {
    final result = await showModalBottomSheet<_OptionFormResult>(
      context: context,
      isScrollControlled: true,
      builder: (_) => _OptionFormSheet(initial: opt),
    );
    if (result == null || !mounted) return;
    final ok = await ref
        .read(shippingNotifierProvider.notifier)
        .updateShippingOption(
          opt.id,
          UpdateShippingOptionRequest(
            name: result.name,
            expeditionName: result.expeditionName,
          ),
        );
    if (!mounted) return;
    if (ok) {
      AppSnackBar.showSuccess(context, 'Opsi pengiriman diperbarui.');
      _reload();
    } else {
      final s = ref.read(shippingNotifierProvider);
      final msg = s is ShippingOptionsListError
          ? s.message
          : 'Gagal memperbarui opsi pengiriman.';
      AppSnackBar.showError(context, msg);
    }
  }

  Future<void> _toggleActive(ShippingOption opt, bool isActive) async {
    final ok = await ref
        .read(shippingNotifierProvider.notifier)
        .toggleActiveStatus(opt.id, isActive);
    if (!mounted) return;
    if (ok) {
      _reload();
    } else {
      final s = ref.read(shippingNotifierProvider);
      final msg = s is ShippingOptionsListError
          ? s.message
          : 'Gagal mengubah status opsi pengiriman.';
      AppSnackBar.showError(context, msg);
    }
  }

  Future<void> _confirmDelete(ShippingOption opt) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogCtx) => AlertDialog(
        title: const Text('Hapus Opsi Pengiriman'),
        content: Text(
          'Hapus "${opt.displayName}" dari daftar opsi pengiriman Anda? '
          'Listing yang sebelumnya memilih opsi ini akan kehilangan tautan tersebut.',
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
        .deleteShippingOption(opt.id);
    if (!mounted) return;
    if (ok) {
      AppSnackBar.showSuccess(context, 'Opsi pengiriman dihapus.');
      _reload();
    } else {
      final s = ref.read(shippingNotifierProvider);
      final msg = s is ShippingOptionsListError
          ? s.message
          : 'Gagal menghapus opsi pengiriman.';
      AppSnackBar.showError(context, msg);
    }
  }

  void _openOptionDetail(ShippingOption opt) {
    context.push('/seller/shipping/${opt.id}');
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
                  ShippingHonestyMessages.optionsBySeller,
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
          'Tambahkan opsi pengiriman (kereta, bus, travel, pesawat, atau kustom). '
          'Listing baru wajib memilih minimal satu opsi sebelum bisa dipublish.',
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
  final ShippingOption option;
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
                      '${option.type.label}${option.expeditionName?.isNotEmpty == true ? ' • ${option.expeditionName}' : ''}',
                      style: TextStyle(
                        fontSize: 12,
                        color: AppColors.neutralGray600,
                      ),
                    ),
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
                  PopupMenuItem(value: 'edit', child: Text('Edit nama')),
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

// =============================================================================
// CREATE / EDIT FORM (BOTTOM SHEET)
// =============================================================================

class _OptionFormResult {
  final String name;
  final ShippingType type;
  final String? expeditionName;
  const _OptionFormResult({
    required this.name,
    required this.type,
    this.expeditionName,
  });
}

class _OptionFormSheet extends StatefulWidget {
  final ShippingOption? initial;
  const _OptionFormSheet({required this.initial});

  @override
  State<_OptionFormSheet> createState() => _OptionFormSheetState();
}

class _OptionFormSheetState extends State<_OptionFormSheet> {
  late ShippingType _type;
  late TextEditingController _nameCtrl;
  late TextEditingController _expeditionCtrl;

  @override
  void initState() {
    super.initState();
    _type = widget.initial?.type ?? ShippingType.custom;
    _nameCtrl = TextEditingController(text: widget.initial?.name ?? '');
    _expeditionCtrl = TextEditingController(
      text: widget.initial?.expeditionName ?? '',
    );
  }

  @override
  void dispose() {
    _nameCtrl.dispose();
    _expeditionCtrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final isEdit = widget.initial != null;
    return Padding(
      padding: EdgeInsets.only(
        left: 16,
        right: 16,
        top: 16,
        bottom: MediaQuery.of(context).viewInsets.bottom + 16,
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text(
            isEdit ? 'Edit Opsi Pengiriman' : 'Tambah Opsi Pengiriman',
            style: const TextStyle(fontSize: 18, fontWeight: FontWeight.bold),
          ),
          const SizedBox(height: 16),
          TextField(
            controller: _nameCtrl,
            decoration: const InputDecoration(
              labelText: 'Nama opsi *',
              hintText: 'Contoh: JNE Reguler',
              border: OutlineInputBorder(),
            ),
            textInputAction: TextInputAction.next,
          ),
          const SizedBox(height: 12),
          if (!isEdit) ...[
            const Text(
              'Jenis transportasi',
              style: TextStyle(fontSize: 13, fontWeight: FontWeight.w600),
            ),
            const SizedBox(height: 8),
            Wrap(
              spacing: 8,
              runSpacing: 8,
              children: ShippingType.values.map((t) {
                final selected = _type == t;
                return ChoiceChip(
                  label: Text('${t.emoji}  ${t.label}'),
                  selected: selected,
                  onSelected: (_) => setState(() => _type = t),
                );
              }).toList(),
            ),
            const SizedBox(height: 16),
          ],
          TextField(
            controller: _expeditionCtrl,
            decoration: const InputDecoration(
              labelText: 'Nama ekspedisi (opsional)',
              hintText: 'Contoh: JNE, J&T, kereta pribadi',
              border: OutlineInputBorder(),
            ),
            textInputAction: TextInputAction.done,
          ),
          const SizedBox(height: 20),
          Row(
            children: [
              Expanded(
                child: OutlinedButton(
                  onPressed: () => Navigator.of(context).pop(),
                  child: const Text('Batal'),
                ),
              ),
              const SizedBox(width: 8),
              Expanded(
                child: ElevatedButton(
                  style: ElevatedButton.styleFrom(
                    backgroundColor: AppColors.primaryRed,
                    foregroundColor: Colors.white,
                  ),
                  onPressed: () {
                    final optionName = _nameCtrl.text.trim();
                    if (optionName.isEmpty) {
                      ScaffoldMessenger.of(context).showSnackBar(
                        const SnackBar(content: Text('Nama opsi wajib diisi.')),
                      );
                      return;
                    }
                    final expeditionName = _expeditionCtrl.text.trim();
                    Navigator.of(context).pop(
                      _OptionFormResult(
                        name: optionName,
                        type: _type,
                        expeditionName: expeditionName.isEmpty
                            ? null
                            : expeditionName,
                      ),
                    );
                  },
                  child: Text(isEdit ? 'Simpan' : 'Tambah'),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}
