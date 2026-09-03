/// Seller Shipping Option Detail Screen
///
/// Manages province-level coverage for a single shipping option. City-level
/// overrides are intentionally deferred to a later phase; in Phase 1 the
/// province rate applies uniformly across that province's cities. Sellers
/// who need finer pricing can use the chat shipping-quote fallback that is
/// already live end-to-end.
///
/// Reuses [shippingSetupDetailNotifierProvider] which already implements
/// load / addCoverage / updateCoverage / deleteCoverage end-to-end.
library;

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/domains/commerce/transaction/shipping/domain/domain.dart';
import 'package:labuda/domains/commerce/transaction/shipping/presentation/providers/providers.dart';
import 'package:labuda/domains/commerce/transaction/shipping/presentation/providers/shipping_state.dart';

class SellerShippingSetupDetailScreen extends ConsumerStatefulWidget {
  final String optionId;
  const SellerShippingSetupDetailScreen({super.key, required this.optionId});

  @override
  ConsumerState<SellerShippingSetupDetailScreen> createState() =>
      _SellerShippingSetupDetailScreenState();
}

class _SellerShippingSetupDetailScreenState
    extends ConsumerState<SellerShippingSetupDetailScreen> {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      ref
          .read(shippingSetupDetailNotifierProvider.notifier)
          .loadOption(widget.optionId);
    });
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(shippingSetupDetailNotifierProvider);
    return Scaffold(
      appBar: AppBar(
        title: const Text('Cakupan Pengiriman'),
        backgroundColor: AppColors.primaryRed,
        foregroundColor: Colors.white,
      ),
      floatingActionButton: state is ShippingSetupDetailLoaded
          ? FloatingActionButton.extended(
              onPressed: () => _openAddCoverageSheet(state.option),
              backgroundColor: AppColors.primaryRed,
              icon: const Icon(Icons.add),
              label: const Text('Tambah Provinsi'),
            )
          : null,
      body: RefreshIndicator(
        onRefresh: () async {
          await ref
              .read(shippingSetupDetailNotifierProvider.notifier)
              .loadOption(widget.optionId);
        },
        child: _buildBody(state),
      ),
    );
  }

  Widget _buildBody(ShippingSetupDetailState state) {
    if (state is ShippingSetupDetailLoading ||
        state is ShippingSetupDetailInitial) {
      return const Center(child: CircularProgressIndicator());
    }
    if (state is ShippingSetupDetailError) {
      return ListView(
        physics: const AlwaysScrollableScrollPhysics(),
        padding: const EdgeInsets.all(24),
        children: [
          const SizedBox(height: 80),
          Icon(Icons.error_outline, size: 64, color: AppColors.error),
          const SizedBox(height: 16),
          const Text(
            'Gagal memuat cakupan pengiriman',
            textAlign: TextAlign.center,
            style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold),
          ),
          const SizedBox(height: 8),
          Text(
            state.message,
            textAlign: TextAlign.center,
            style: TextStyle(fontSize: 13, color: AppColors.neutralGray600),
          ),
          const SizedBox(height: 24),
          ElevatedButton(
            onPressed: () => ref
                .read(shippingSetupDetailNotifierProvider.notifier)
                .loadOption(widget.optionId),
            child: const Text('Coba Lagi'),
          ),
        ],
      );
    }
    if (state is ShippingSetupDetailLoaded) {
      final opt = state.option;
      final coverages = opt.coverageAreas
          .where((c) => c.provinceRate != null || c.cityOverrides.isNotEmpty)
          .toList();
      return ListView(
        physics: const AlwaysScrollableScrollPhysics(),
        padding: const EdgeInsets.fromLTRB(16, 16, 16, 96),
        children: [
          _HeaderCard(option: opt),
          const SizedBox(height: 16),
          const _CoverageHonestyNote(),
          const SizedBox(height: 12),
          if (coverages.isEmpty)
            _EmptyCoverage(onAdd: () => _openAddCoverageSheet(opt))
          else
            ...coverages.map(
              (c) => Padding(
                padding: const EdgeInsets.only(bottom: 8),
                child: _CoverageRow(
                  coverage: c,
                  onEdit: () => _openEditCoverageSheet(opt, c),
                  onDelete: () => _confirmDeleteCoverage(c),
                ),
              ),
            ),
        ],
      );
    }
    return const SizedBox.shrink();
  }

  Future<void> _openAddCoverageSheet(ShippingSetup option) async {
    final existingProvinceIds = option.coverageAreas
        .map((c) => c.provinceId)
        .toSet();
    final result = await showModalBottomSheet<_CoverageFormResult>(
      context: context,
      isScrollControlled: true,
      builder: (_) => _CoverageFormSheet(
        initial: null,
        disallowedProvinceIds: existingProvinceIds,
      ),
    );
    if (result == null || !mounted) return;
    final ok = await ref
        .read(shippingSetupDetailNotifierProvider.notifier)
        .addCoverage(
          option.id,
          AddCoverageRequest(
            provinceCode: result.provinceCode,
            provinceName: result.provinceName,
            rate: result.rate,
            isAvailable: true,
          ),
        );
    if (!mounted) return;
    if (ok) {
      AppSnackBar.showSuccess(context, 'Cakupan provinsi ditambahkan.');
    } else {
      final s = ref.read(shippingSetupDetailNotifierProvider);
      final msg = s is ShippingSetupDetailError
          ? s.message
          : 'Gagal menambah cakupan provinsi.';
      AppSnackBar.showError(context, msg);
    }
  }

  Future<void> _openEditCoverageSheet(
    ShippingSetup option,
    ShippingCoverage coverage,
  ) async {
    final result = await showModalBottomSheet<_CoverageFormResult>(
      context: context,
      isScrollControlled: true,
      builder: (_) => _CoverageFormSheet(
        initial: coverage,
        disallowedProvinceIds: const {},
      ),
    );
    if (result == null || !mounted) return;
    final ok = await ref
        .read(shippingSetupDetailNotifierProvider.notifier)
        .updateCoverage(
          coverage.provinceId,
          UpdateCoverageRequest(
            provinceName: result.provinceName,
            provinceRate: result.rate,
            isAvailable: coverage.isAvailable,
          ),
        );
    if (!mounted) return;
    if (ok) {
      AppSnackBar.showSuccess(context, 'Cakupan provinsi diperbarui.');
    } else {
      final s = ref.read(shippingSetupDetailNotifierProvider);
      final msg = s is ShippingSetupDetailError
          ? s.message
          : 'Gagal memperbarui cakupan provinsi.';
      AppSnackBar.showError(context, msg);
    }
  }

  Future<void> _confirmDeleteCoverage(ShippingCoverage coverage) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogCtx) => AlertDialog(
        title: const Text('Hapus Cakupan'),
        content: Text(
          'Hapus cakupan untuk ${coverage.provinceName}? '
          'Pembeli di provinsi ini tidak akan bisa memilih opsi pengiriman ini lagi.',
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
        .read(shippingSetupDetailNotifierProvider.notifier)
        .deleteCoverage(coverage.provinceId);
    if (!mounted) return;
    if (ok) {
      AppSnackBar.showSuccess(context, 'Cakupan provinsi dihapus.');
    } else {
      final s = ref.read(shippingSetupDetailNotifierProvider);
      final msg = s is ShippingSetupDetailError
          ? s.message
          : 'Gagal menghapus cakupan provinsi.';
      AppSnackBar.showError(context, msg);
    }
  }
}

// =============================================================================
// HEADER + NOTE + EMPTY STATE
// =============================================================================

class _HeaderCard extends StatelessWidget {
  final ShippingSetup option;
  const _HeaderCard({required this.option});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: AppColors.primaryRed.withValues(alpha: 0.05),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: AppColors.primaryRed.withValues(alpha: 0.2)),
      ),
      child: Row(
        children: [
          CircleAvatar(
            radius: 22,
            backgroundColor: AppColors.primaryRed.withValues(alpha: 0.15),
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
                    fontSize: 16,
                    fontWeight: FontWeight.bold,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  option.isActive ? 'Aktif' : 'Nonaktif',
                  style: TextStyle(
                    fontSize: 12,
                    color: option.isActive
                        ? AppColors.successGreen
                        : AppColors.neutralGray500,
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

class _CoverageHonestyNote extends StatelessWidget {
  const _CoverageHonestyNote();

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(10),
      decoration: BoxDecoration(
        color: AppColors.neutralGray100,
        borderRadius: BorderRadius.circular(8),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Icon(
            Icons.lightbulb_outline,
            size: 16,
            color: AppColors.neutralGray600,
          ),
          const SizedBox(width: 6),
          Expanded(
            child: Text(
              'Tarif berlaku untuk seluruh kota di provinsi yang Anda tambahkan. '
              'Untuk kasus khusus (ukuran ikan besar, alamat sulit dijangkau), '
              'gunakan kirim quote pengiriman di chat dengan pembeli.',
              style: TextStyle(fontSize: 12, color: AppColors.neutralGray700),
            ),
          ),
        ],
      ),
    );
  }
}

class _EmptyCoverage extends StatelessWidget {
  final VoidCallback onAdd;
  const _EmptyCoverage({required this.onAdd});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(24),
      decoration: BoxDecoration(
        border: Border.all(color: AppColors.neutralGray300),
        borderRadius: BorderRadius.circular(12),
      ),
      child: Column(
        children: [
          Icon(Icons.map_outlined, size: 56, color: AppColors.neutralGray400),
          const SizedBox(height: 12),
          const Text(
            'Belum Ada Cakupan',
            style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold),
          ),
          const SizedBox(height: 4),
          Text(
            'Tambahkan provinsi yang Anda layani beserta tarifnya.',
            textAlign: TextAlign.center,
            style: TextStyle(fontSize: 13, color: AppColors.neutralGray600),
          ),
          const SizedBox(height: 16),
          OutlinedButton.icon(
            onPressed: onAdd,
            icon: const Icon(Icons.add),
            label: const Text('Tambah Provinsi'),
          ),
        ],
      ),
    );
  }
}

// =============================================================================
// COVERAGE ROW
// =============================================================================

class _CoverageRow extends StatelessWidget {
  final ShippingCoverage coverage;
  final VoidCallback onEdit;
  final VoidCallback onDelete;

  const _CoverageRow({
    required this.coverage,
    required this.onEdit,
    required this.onDelete,
  });

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final rate = coverage.provinceRate;
    return Card(
      elevation: 0,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(12),
        side: BorderSide(
          color: isDark ? AppColors.darkGray700 : AppColors.neutralGray200,
        ),
      ),
      child: ListTile(
        contentPadding: const EdgeInsets.fromLTRB(14, 4, 4, 4),
        title: Text(
          coverage.provinceName,
          style: const TextStyle(fontWeight: FontWeight.w600),
        ),
        subtitle: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const SizedBox(height: 2),              Text(
              rate == null
                  ? 'Tarif belum ditetapkan'
                  : 'Tarif: Rp ${AppFormatters.formatCurrency(rate)}',
              style: TextStyle(fontSize: 13, color: AppColors.neutralGray700),
            ),
          ],
        ),
        trailing: PopupMenuButton<String>(
          itemBuilder: (_) => const [
            PopupMenuItem(value: 'edit', child: Text('Edit')),
            PopupMenuItem(value: 'delete', child: Text('Hapus')),
          ],
          onSelected: (v) {
            if (v == 'edit') onEdit();
            if (v == 'delete') onDelete();
          },
        ),
        onTap: onEdit,
      ),
    );
  }
}

// =============================================================================
// COVERAGE FORM SHEET
// =============================================================================

class _CoverageFormResult {
  final String provinceCode;
  final String provinceName;
  final double rate;
  const _CoverageFormResult({
    required this.provinceCode,
    required this.provinceName,
    required this.rate,
  });
}

class _CoverageFormSheet extends ConsumerStatefulWidget {
  final ShippingCoverage? initial;
  final Set<String> disallowedProvinceIds;
  const _CoverageFormSheet({
    required this.initial,
    required this.disallowedProvinceIds,
  });

  @override
  ConsumerState<_CoverageFormSheet> createState() => _CoverageFormSheetState();
}

class _CoverageFormSheetState extends ConsumerState<_CoverageFormSheet> {
  Province? _province;
  final _rateCtrl = TextEditingController();
  bool _submitting = false;
  String? _validationMsg;

  @override
  void initState() {
    super.initState();
    final init = widget.initial;
    if (init != null) {
      _province = Province(id: init.provinceId, name: init.provinceName);
      if (init.provinceRate != null) {
        _rateCtrl.text = init.provinceRate!.toInt().toString();
      }
    }
  }

  @override
  void dispose() {
    _rateCtrl.dispose();
    super.dispose();
  }

  Future<void> _pickProvince() async {
    final picked = await showModalBottomSheet<Province>(
      context: context,
      isScrollControlled: true,
      builder: (_) => const _ProvincePickerSheet(),
    );
    if (picked == null || !mounted) return;
    if (widget.disallowedProvinceIds.contains(picked.id)) {
      setState(() {
        _validationMsg =
            'Provinsi ini sudah ada dalam daftar cakupan. Edit cakupan yang sudah ada.';
      });
      return;
    }
    setState(() {
      _province = picked;
      _validationMsg = null;
    });
  }

  void _submit() {
    final province = _province;
    if (province == null) {
      setState(() => _validationMsg = 'Pilih provinsi terlebih dahulu.');
      return;
    }
    final rateText = _rateCtrl.text.trim().replaceAll('.', '');
    final rate = double.tryParse(rateText);
    if (rate == null || rate <= 0) {
      setState(() => _validationMsg = 'Masukkan tarif yang valid (Rp).');
      return;
    }
    setState(() => _submitting = true);
    Navigator.of(context).pop(
      _CoverageFormResult(
        provinceCode: province.id,
        provinceName: province.name,
        rate: rate,
      ),
    );
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
      child: SingleChildScrollView(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(
              isEdit ? 'Edit Cakupan Provinsi' : 'Tambah Cakupan Provinsi',
              style: const TextStyle(fontSize: 18, fontWeight: FontWeight.bold),
            ),
            const SizedBox(height: 16),
            InkWell(
              onTap: isEdit ? null : _pickProvince,
              child: InputDecorator(
                decoration: InputDecoration(
                  labelText: 'Provinsi',
                  border: const OutlineInputBorder(),
                  suffixIcon: isEdit ? null : const Icon(Icons.arrow_drop_down),
                ),
                child: Text(
                  _province?.name ?? 'Pilih provinsi',
                  style: TextStyle(
                    fontSize: 15,
                    color: _province == null ? AppColors.neutralGray500 : null,
                  ),
                ),
              ),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _rateCtrl,
              keyboardType: TextInputType.number,
              inputFormatters: [FilteringTextInputFormatter.digitsOnly],
              decoration: const InputDecoration(
                labelText: 'Tarif (Rp)',
                hintText: 'Contoh: 150000',
                prefixText: 'Rp ',
                border: OutlineInputBorder(),
              ),
            ),
            const SizedBox(height: 12),
            if (_validationMsg != null) ...[
              const SizedBox(height: 12),
              Text(
                _validationMsg!,
                style: TextStyle(fontSize: 12, color: AppColors.error),
              ),
            ],
            const SizedBox(height: 20),
            Row(
              children: [
                Expanded(
                  child: OutlinedButton(
                    onPressed: _submitting
                        ? null
                        : () => Navigator.of(context).pop(),
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
                    onPressed: _submitting ? null : _submit,
                    child: Text(isEdit ? 'Simpan' : 'Tambah'),
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

// =============================================================================
// PROVINCE PICKER SHEET (inline; reuses provincesProvider from shared)
// =============================================================================

class _ProvincePickerSheet extends ConsumerStatefulWidget {
  const _ProvincePickerSheet();

  @override
  ConsumerState<_ProvincePickerSheet> createState() =>
      _ProvincePickerSheetState();
}

class _ProvincePickerSheetState extends ConsumerState<_ProvincePickerSheet> {
  String _query = '';

  @override
  Widget build(BuildContext context) {
    final provincesAsync = ref.watch(provincesProvider);
    final mq = MediaQuery.of(context);
    return SizedBox(
      height: mq.size.height * 0.75,
      child: Padding(
        padding: EdgeInsets.only(
          left: 16,
          right: 16,
          top: 16,
          bottom: mq.viewInsets.bottom + 12,
        ),
        child: Column(
          children: [
            const Text(
              'Pilih Provinsi',
              style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold),
            ),
            const SizedBox(height: 12),
            TextField(
              decoration: const InputDecoration(
                prefixIcon: Icon(Icons.search),
                hintText: 'Cari provinsi...',
                border: OutlineInputBorder(),
                isDense: true,
              ),
              onChanged: (v) => setState(() => _query = v.toLowerCase()),
            ),
            const SizedBox(height: 12),
            Expanded(
              child: provincesAsync.when(
                loading: () => const Center(child: CircularProgressIndicator()),
                error: (e, _) => Center(
                  child: Text(
                    'Gagal memuat daftar provinsi: $e',
                    textAlign: TextAlign.center,
                    style: TextStyle(color: AppColors.error),
                  ),
                ),
                data: (provinces) {
                  final filtered = _query.isEmpty
                      ? provinces
                      : provinces
                            .where((p) => p.name.toLowerCase().contains(_query))
                            .toList();
                  if (filtered.isEmpty) {
                    return const Center(child: Text('Tidak ada hasil.'));
                  }
                  return ListView.builder(
                    itemCount: filtered.length,
                    itemBuilder: (_, i) {
                      final p = filtered[i];
                      return ListTile(
                        title: Text(p.name),
                        onTap: () => Navigator.of(context).pop(p),
                      );
                    },
                  );
                },
              ),
            ),
          ],
        ),
      ),
    );
  }
}
