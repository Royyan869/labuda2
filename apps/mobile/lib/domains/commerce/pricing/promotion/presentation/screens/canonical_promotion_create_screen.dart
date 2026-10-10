library;

import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/commerce/pricing/promotion/data/dto/promotion_contract_dto.dart';
import 'package:hishumi/domains/commerce/pricing/promotion/presentation/providers/canonical_promotion_providers.dart';
import 'package:hishumi/domains/finance/transaction/payment/domain/entities/payment.dart'
    show PaymentMethodOption;
import 'package:hishumi/domains/finance/transaction/payment/presentation/widgets/payment_method_picker_sheet.dart';
import 'package:hishumi/shared/models/wilayah_models.dart';
import 'package:hishumi/shared/payment/payment_method_trigger.dart';
import 'package:hishumi/shared/utils/app_formatters.dart';
import 'package:hishumi/shared/utils/money_input_formatter.dart';
import 'package:hishumi/shared/widgets/app_bottom_sheet_base.dart';
import 'package:hishumi/shared/widgets/app_snackbar.dart';
import 'package:hishumi/shared/widgets/app_text_field.dart';
import 'package:hishumi/shared/widgets/wilayah/city_dropdown.dart';
import 'package:hishumi/shared/widgets/wilayah/province_dropdown.dart';

/// Canonical "Buat Promosi" screen.
///
/// AUTHORITY BOUNDARY:
/// - The reusable funding number comes from GET /promote-balance (canonical
///   PROMOTE_BALANCE projection). The client never computes a balance.
/// - The funding gate is POST /promotions/contracts/payment-intent: the backend
///   decides whether a payment is required and for exactly how much. The client
///   never computes a shortage, a CPM, or a spend.
/// - The payment methods, each method's fee and each gross total come from
///   GET /promotions/contracts/payment-intent/:id/payment-methods. The client
///   never computes a fee and never falls back to a local method list.
/// - Creation calls the single canonical endpoint POST /promotions/contracts.
///
/// REUSE-FIRST: when the seller's reusable PROMOTE_BALANCE already covers the
/// promotion cost, the backend reports no payment required and Create draws
/// straight from the reusable balance — no picker, no initiation. When a
/// shortage exists, the seller pays exactly that shortage inside the canonical
/// payment WebView and the same gate is then re-run; an underfunded promotion is
/// never created.
class CanonicalPromotionCreateScreen extends ConsumerStatefulWidget {
  const CanonicalPromotionCreateScreen({super.key});

  @override
  ConsumerState<CanonicalPromotionCreateScreen> createState() =>
      _CanonicalPromotionCreateScreenState();
}

class _CanonicalPromotionCreateScreenState
    extends ConsumerState<CanonicalPromotionCreateScreen> {
  final _formKey = GlobalKey<FormState>();
  String _kind = 'internal';
  // Seeded in the canonical money-input display form.
  final _budgetController = TextEditingController(
    text: MoneyInputFormatter.display(30000),
  );
  final _durationController = TextEditingController(text: '3');
  // Canonical geographic targeting: a set of cities selected from the ONE
  // Geography Master. Empty = nationwide/unrestricted. Free-text entry is
  // forbidden — a seller may only select canonical entities.
  final List<City> _selectedCities = [];
  // Initial product queue (part of the promotion configuration, BEFORE
  // funding/payment). Selection order IS queue order. The backend owns every
  // queue rule (minimum 1, maximum 10, duplicates, kind, eligibility); this
  // list is transport only.
  final List<_SelectedTarget> _selectedTargets = [];
  bool _isLoading = false;

  /// Upper bound UX hint only — the backend remains the authority (max 10).
  static const int _maxTargets = 10;

  List<PromotionTargetDto> _targetDtos() =>
      _selectedTargets.map((e) => e.target).toList();

  void _appendTarget(PromotionTargetDto target, String title) {
    if (_selectedTargets.length >= _maxTargets) {
      AppSnackBar.showWarning(context, 'Maksimal $_maxTargets produk per promosi');
      return;
    }
    if (_selectedTargets.any((e) => e.target.targetId == target.targetId)) {
      AppSnackBar.showWarning(context, 'Produk sudah dipilih');
      return;
    }
    setState(
      () => _selectedTargets.add(_SelectedTarget(target: target, title: title)),
    );
  }

  void _removeTarget(String targetId) {
    setState(
      () => _selectedTargets.removeWhere((e) => e.target.targetId == targetId),
    );
  }

  /// Opens the canonical product-picker boundary and appends the chosen
  /// product to the ordered queue. The picker provider composes the existing
  /// canonical product sources (shared commerce picker for For Sale/Auction;
  /// external-product provider for external promotions); the backend owns
  /// every queue rule.
  Future<void> _addTarget() async {
    if (_selectedTargets.length >= _maxTargets) return;
    final pick = ref.read(promotionProductPickerProvider);
    final selection = await pick(context, ref, _kind);
    if (!mounted || selection == null) return;
    _appendTarget(
      PromotionTargetDto(
        targetType: selection.targetType,
        targetId: selection.targetId,
      ),
      selection.title,
    );
  }

  @override
  void dispose() {
    _budgetController.dispose();
    _durationController.dispose();
    super.dispose();
  }

  List<String> _parsedCityIds() => _selectedCities.map((c) => c.id).toList();

  String _targetTypeLabel(String type) {
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

  Future<void> _addCity() async {
    final city = await AppBottomSheetBase.show<City>(
      context: context,
      title: 'Tambah target wilayah',
      content: const _PromotionCityPickerSheet(),
    );
    if (!mounted || city == null) return;
    if (_selectedCities.any((c) => c.id == city.id)) return;
    setState(() => _selectedCities.add(city));
  }

  void _removeCity(City city) {
    setState(() => _selectedCities.removeWhere((c) => c.id == city.id));
  }

  Future<void> _submit() async {
    if (!_formKey.currentState!.validate()) return;
    if (_selectedTargets.isEmpty) {
      AppSnackBar.showError(
        context,
        'Pilih minimal satu produk untuk dipromosikan',
      );
      return;
    }
    final budget = MoneyInputFormatter.parseAmount(_budgetController.text) ?? 0;
    final duration = int.tryParse(_durationController.text) ?? 0;
    final cityIds = _parsedCityIds();
    // SUBMISSION SNAPSHOT: `kind` and the product queue are read by the funding
    // gate AND by contract creation across a multi-step async/payment flow.
    // Capture them once so changing the form mid-flow can never fund one
    // configuration and create a contract of another.
    final kind = _kind;
    final targets = _targetDtos();

    setState(() => _isLoading = true);

    // Step 1: the canonical funding gate. The backend decides whether a payment
    // is required and for exactly how much. When reusable PROMOTE_BALANCE
    // already covers the cost the backend creates nothing (no intent, no
    // billing) and reports payment_required = false.
    final gate = await _requestFundingIntent(
      kind,
      budget,
      duration,
      cityIds,
      targets,
    );
    if (!mounted) return;
    if (gate == null) {
      setState(() => _isLoading = false);
      return;
    }

    // Step 2: reuse-first — sufficient reusable funding, no payment at all.
    if (!gate.paymentRequired) {
      await _createContract(kind, budget, duration, cityIds, targets);
      return;
    }

    // Step 3: exact shortage → the seller pays exactly the shortage through the
    // canonical disclosure + initiation, inside the canonical payment WebView.
    setState(() => _isLoading = false);
    final paid = await _openFundingPaymentSheet(gate, duration);
    if (!mounted || paid != true) return;

    // Step 4: after the payment attempt, re-run the same canonical gate. A
    // settled payment credits reusable PROMOTE_BALANCE, so the gate reports no
    // payment required and Create proceeds. A still-pending payment leaves the
    // gate unchanged and the seller retries — the backend reuses the same
    // intent/billing, so retrying never duplicates an obligation or a payment.
    setState(() => _isLoading = true);
    final afterPayment = await _requestFundingIntent(
      kind,
      budget,
      duration,
      cityIds,
      targets,
    );
    if (!mounted) return;
    if (afterPayment == null) {
      setState(() => _isLoading = false);
      return;
    }
    if (afterPayment.paymentRequired) {
      setState(() => _isLoading = false);
      ref.invalidate(promoteBalanceProvider);
      AppSnackBar.showWarning(
        context,
        'Pembayaran belum terkonfirmasi. Tunggu sebentar, lalu coba lagi.',
      );
      return;
    }
    await _createContract(kind, budget, duration, cityIds, targets);
  }

  /// Asks the backend for this promotion's funding obligation (or the absence
  /// of one). Returns null — after showing the backend error — when it failed.
  Future<PromotionFundingIntentDto?> _requestFundingIntent(
    String kind,
    int budget,
    int duration,
    List<String> cityIds,
    List<PromotionTargetDto> targets,
  ) async {
    final repo = ref.read(promotionContractRepositoryProvider);
    final result = await repo.createFundingIntent(
      kind: kind,
      budgetRupiah: budget,
      durationDays: duration,
      cityIds: cityIds,
      targets: targets,
    );
    if (result.isSuccess && result.data != null) return result.data;
    if (!mounted) return null;
    AppSnackBar.showError(
      context,
      result.error ?? 'Gagal menghitung kebutuhan dana promosi',
    );
    return null;
  }

  /// Canonical Create. Only reachable when the backend reports that no payment
  /// is required, so an underfunded promotion can never be created.
  Future<void> _createContract(
    String kind,
    int budget,
    int duration,
    List<String> cityIds,
    List<PromotionTargetDto> targets,
  ) async {
    final repo = ref.read(promotionContractRepositoryProvider);
    final result = await repo.createContract(
      kind: kind,
      budgetRupiah: budget,
      durationDays: duration,
      cityIds: cityIds,
      targets: targets,
    );
    if (!mounted) return;
    setState(() => _isLoading = false);
    if (result.isSuccess) {
      AppSnackBar.showSuccess(context, 'Promosi berhasil dibuat');
      ref.invalidate(myPromotionContractsProvider);
      // Reusable funding changed (PROMOTE_BALANCE → PROMOTION_ALLOCATION).
      ref.invalidate(promoteBalanceProvider);
      context.pop();
    } else {
      AppSnackBar.showError(context, result.error ?? 'Gagal membuat promosi');
    }
  }

  /// Opens the canonical exact-shortage payment surface and, when a payment was
  /// initiated, the canonical internal payment WebView. Returns true once the
  /// WebView closed on an initiated payment, null otherwise.
  Future<bool?> _openFundingPaymentSheet(
    PromotionFundingIntentDto intent,
    int durationDays,
  ) async {
    final paymentUrl = await AppBottomSheetBase.show<String>(
      context: context,
      title: 'Kekurangan dana promosi',
      content: _PromotionFundingPaymentSheet(
        intent: intent,
        durationDays: durationDays,
      ),
    );
    if (!mounted || paymentUrl == null || paymentUrl.isEmpty) return null;
    // Payment URLs are presented exclusively inside Labuda's internal WebView.
    await context.push(
      '/payment-webview?url=${Uri.encodeComponent(paymentUrl)}',
    );
    if (!mounted) return null;
    return true;
  }

  @override
  Widget build(BuildContext context) {
    final balanceAsync = ref.watch(promoteBalanceProvider);
    return Scaffold(
      appBar: AppBar(title: const Text('Buat Promosi')),
      // Canonical body-level bottom-inset authority (SAFE-AREA-19): the ONE
      // `SafeArea` consumes the live system bottom inset for the whole body.
      // The scroll view's explicit `p16` padding below is DESIGN spacing only
      // — an explicit `ScrollView.padding` never inherits MediaQuery padding.
      body: SafeArea(
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(AppMetrics.p16),
          child: Form(
            key: _formKey,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                _ReusableFundingCard(balanceAsync: balanceAsync),
                const SizedBox(height: 16),
                DropdownButtonFormField<String>(
                  initialValue: _kind,
                  decoration:
                      const InputDecoration(labelText: 'Jenis Promosi'),
                  items: const [
                    DropdownMenuItem(
                      value: 'internal',
                      child: Text('Internal (For Sale/Auction)'),
                    ),
                    DropdownMenuItem(
                      value: 'external',
                      child: Text('Eksternal (Event/Business)'),
                    ),
                  ],
                  onChanged: (v) => setState(() {
                    _kind = v!;
                    // Switching kind changes the allowed target types; the
                    // queue must be rebuilt from scratch (one queue authority).
                    _selectedTargets.clear();
                  }),
                ),
                const SizedBox(height: 12),
                Text(
                  'Produk yang dipromosikan',
                  style: context.typeRoles.labelMicro.copyWith(
                    color: Theme.of(context).colorScheme.onSurfaceVariant,
                  ),
                ),
                const SizedBox(height: 6),
                if (_selectedTargets.isEmpty)
                  Text(
                    'Pilih minimal 1 produk. Urutan pilihan menjadi urutan '
                    'promosi (maksimal $_maxTargets).',
                    style: context.typeRoles.bodyDense.copyWith(
                      color: Theme.of(context).colorScheme.onSurfaceVariant,
                    ),
                  )
                else
                  Column(
                    children: [
                      for (var i = 0; i < _selectedTargets.length; i++)
                        ListTile(
                          contentPadding: EdgeInsets.zero,
                          dense: true,
                          leading: CircleAvatar(
                            radius: 12,
                            child: Text('${i + 1}'),
                          ),
                          title: Text(
                            _selectedTargets[i].title,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                          ),
                          subtitle: Text(
                            _targetTypeLabel(
                              _selectedTargets[i].target.targetType,
                            ),
                          ),
                          trailing: IconButton(
                            icon: const Icon(Icons.close),
                            onPressed: _isLoading
                                ? null
                                : () => _removeTarget(
                                    _selectedTargets[i].target.targetId,
                                  ),
                          ),
                        ),
                    ],
                  ),
                const SizedBox(height: 8),
                OutlinedButton.icon(
                  onPressed:
                      (_isLoading || _selectedTargets.length >= _maxTargets)
                          ? null
                          : _addTarget,
                  icon: const Icon(Icons.add),
                  label: const Text('Tambah Produk'),
                ),
                const SizedBox(height: 12),
                AppTextField(
                  controller: _budgetController,
                  labelText: 'Budget (Rupiah)',
                  hintText: '30000',
                  keyboardType: TextInputType.number,
                  inputFormatters: const [MoneyInputFormatter()],
                  validator: (v) =>
                      (MoneyInputFormatter.parseAmount(v ?? '') ?? 0) <= 0
                      ? 'Budget harus >0'
                      : null,
                ),
                const SizedBox(height: 12),
                AppTextField(
                  controller: _durationController,
                  labelText: 'Duration (hari)',
                  hintText: '3',
                  keyboardType: TextInputType.number,
                  validator: (v) => (int.tryParse(v ?? '') ?? 0) <= 0
                      ? 'Duration harus >0'
                      : null,
                ),
                const SizedBox(height: 12),
                Text(
                  'Target Wilayah',
                  style: context.typeRoles.labelMicro.copyWith(
                    color: Theme.of(context).colorScheme.onSurfaceVariant,
                  ),
                ),
                const SizedBox(height: 6),
                if (_selectedCities.isEmpty)
                  Text(
                    'Nasional (semua wilayah). Tambahkan kota untuk membatasi promosi.',
                    style: context.typeRoles.bodyDense.copyWith(
                      color: Theme.of(context).colorScheme.onSurfaceVariant,
                    ),
                  )
                else
                  Wrap(
                    spacing: 8,
                    runSpacing: 8,
                    children: _selectedCities
                        .map(
                          (city) => Chip(
                            label: Text(city.name),
                            onDeleted: () => _removeCity(city),
                          ),
                        )
                        .toList(),
                  ),
                const SizedBox(height: 8),
                OutlinedButton.icon(
                  onPressed: _isLoading ? null : _addCity,
                  icon: const Icon(Icons.add_location_alt_outlined),
                  label: const Text('Tambah Kota/Kabupaten'),
                ),
                const SizedBox(height: 24),
                SizedBox(
                  width: double.infinity,
                  child: ElevatedButton(
                    onPressed: _isLoading ? null : _submit,
                    child: _isLoading
                        ? const CircularProgressIndicator()
                        : const Text('Buat Promosi'),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

/// One selected queue entry (transport DTO + display title) held by the create
/// screen. Order in the list IS the promotion queue order.
class _SelectedTarget {
  final PromotionTargetDto target;
  final String title;

  const _SelectedTarget({required this.target, required this.title});
}

/// Canonical city/regency picker for promotion targeting.
///
/// The seller selects a province and a regency from the ONE Geography Master
/// through the canonical Geography API. No free-text city identity exists.
class _PromotionCityPickerSheet extends ConsumerStatefulWidget {
  const _PromotionCityPickerSheet();

  @override
  ConsumerState<_PromotionCityPickerSheet> createState() =>
      _PromotionCityPickerSheetState();
}

class _PromotionCityPickerSheetState
    extends ConsumerState<_PromotionCityPickerSheet> {
  Province? _province;
  City? _city;

  @override
  Widget build(BuildContext context) {
    return Column(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        ProvinceDropdown(
          selectedProvince: _province,
          onChanged: (province) => setState(() {
            _province = province;
            _city = null;
          }),
          labelText: 'Provinsi',
          hintText: 'Pilih provinsi',
        ),
        const SizedBox(height: 12),
        CityDropdown(
          selectedCity: _city,
          selectedProvince: _province,
          onChanged: (city) => setState(() => _city = city),
          labelText: 'Kota/Kabupaten',
          hintText: 'Pilih kota/kabupaten',
        ),
        const SizedBox(height: 16),
        ElevatedButton(
          onPressed: _city == null
              ? null
              : () => Navigator.of(context).pop(_city),
          child: const Text('Tambahkan'),
        ),
      ],
    );
  }
}

/// Read-only reusable PROMOTE_BALANCE card (backend projection).
class _ReusableFundingCard extends StatelessWidget {
  final AsyncValue<Result<PromoteBalanceDto>> balanceAsync;

  const _ReusableFundingCard({required this.balanceAsync});

  @override
  Widget build(BuildContext context) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(AppMetrics.p16),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(AppShape.r12),
        border: Border.all(color: Theme.of(context).colorScheme.outlineVariant),
        color: Theme.of(context).colorScheme.surface,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            'Saldo promo tersedia',
            style: context.typeRoles.labelMicro.copyWith(
              color: Theme.of(context).colorScheme.onSurfaceVariant,
            ),
          ),
          const SizedBox(height: 6),
          balanceAsync.when(
            data: (result) {
              if (result.isSuccess && result.data != null) {
                return Text(
                  AppFormatters.formatCurrencyInt(result.data!.balance),
                  style: context.typeRoles.titleProminent.copyWith(
                    fontWeight: FontWeight.bold,
                  ),
                );
              }
              return Text(
                result.error ?? 'Gagal memuat saldo promosi',
                style: context.typeRoles.bodyDense.copyWith(
                  color: Theme.of(context).colorScheme.primary,
                ),
              );
            },
            loading: () => const SizedBox(
              height: 20,
              width: 20,
              child: CircularProgressIndicator(strokeWidth: 2),
            ),
            error: (e, _) => Text(
              e.toString(),
              style: context.typeRoles.bodyDense.copyWith(
                color: Theme.of(context).colorScheme.primary,
              ),
            ),
          ),
          const SizedBox(height: 4),
          Text(
            'Saldo ini bisa langsung dipakai untuk membuat promosi tanpa '
            'pembayaran baru.',
            style: context.typeRoles.labelMicro.copyWith(
              color: Theme.of(context).colorScheme.outline,
            ),
          ),
        ],
      ),
    );
  }
}

/// Maps a canonical funding error to a seller-facing message.
///
/// The backend is the authority: 404/403/409 mean the obligation itself is
/// gone, foreign, or no longer payable — never "invent a method or a fee".
/// Anything else (transport/5xx) keeps the backend message and is retryable.
String _fundingErrorMessage(Result<dynamic> result, String fallback) {
  switch (result.statusCode) {
    case 404:
      return 'Pengajuan dana promosi tidak ditemukan. '
          'Ulangi proses pembuatan promosi.';
    case 403:
      return 'Anda hanya dapat membayar kekurangan dana milik akun Anda.';
    case 409:
      return 'Pengajuan pembayaran ini sudah tidak berlaku. '
          'Ulangi proses pembuatan promosi.';
  }
  return result.error ?? fallback;
}

/// Exact-shortage funding payment surface for one canonical funding obligation.
///
/// AUTHORITY BOUNDARY:
/// - the shortage, the available methods, every method's fee and every gross
///   total come from the canonical disclosure endpoint; nothing is computed here;
/// - the selected method_code is sent verbatim to the canonical pay endpoint;
/// - a failed disclosure never falls back to a locally invented method list, so
///   the picker fails closed with a retry.
///
/// Pops the gateway redirect URL once a payment was initiated, so the caller can
/// open the canonical payment WebView and then re-run the funding gate.
class _PromotionFundingPaymentSheet extends ConsumerStatefulWidget {
  final PromotionFundingIntentDto intent;
  final int durationDays;

  const _PromotionFundingPaymentSheet({
    required this.intent,
    required this.durationDays,
  });

  @override
  ConsumerState<_PromotionFundingPaymentSheet> createState() =>
      _PromotionFundingPaymentSheetState();
}

class _PromotionFundingPaymentSheetState
    extends ConsumerState<_PromotionFundingPaymentSheet> {
  PromotionFundingPaymentMethodsDto? _disclosure;
  PromotionFundingPaymentMethodDto? _selected;
  String? _error;
  bool _loading = true;
  bool _paying = false;

  @override
  void initState() {
    super.initState();
    _loadDisclosure();
  }

  Future<void> _loadDisclosure() async {
    final intentId = widget.intent.intentId;
    if (intentId == null || intentId.isEmpty) {
      setState(() {
        _loading = false;
        _error =
            'Pengajuan dana promosi tidak lengkap. Ulangi proses pembuatan.';
      });
      return;
    }
    setState(() {
      _loading = true;
      _error = null;
    });
    final result = await ref
        .read(promotionContractRepositoryProvider)
        .getFundingPaymentMethods(intentId);
    if (!mounted) return;
    if (result.isError || result.data == null) {
      setState(() {
        _loading = false;
        _error = _fundingErrorMessage(result, 'Gagal memuat metode pembayaran');
      });
      return;
    }
    setState(() {
      _loading = false;
      _disclosure = result.data;
    });
  }

  Future<void> _pickMethod() async {
    final methods = _disclosure?.methods ?? const [];
    if (methods.isEmpty) return;
    final options = methods
        .map(
          (m) => PaymentMethodOption(
            methodCode: m.methodCode,
            displayName: m.displayName,
            buyerPaymentFeeAmount: m.serviceFeeAmount,
            totalPayableAmount: m.grossAmount,
          ),
        )
        .toList();
    final code = await PaymentMethodPickerSheet.show(
      context,
      methods: options,
      // Reuse the currently selected method as the checked row (selected-state
      // semantics unchanged).
      selectedMethodCode: _selected?.methodCode,
    );
    if (!mounted || code == null) return;
    for (final m in methods) {
      if (m.methodCode == code) {
        setState(() => _selected = m);
        return;
      }
    }
  }

  Future<void> _pay() async {
    final intentId = widget.intent.intentId;
    final code = _selected?.methodCode;
    if (_paying || intentId == null || code == null) return;
    setState(() {
      _paying = true;
      _error = null;
    });
    final result = await ref
        .read(promotionContractRepositoryProvider)
        .initiateFundingPayment(intentId: intentId, paymentMethodCode: code);
    if (!mounted) return;
    setState(() => _paying = false);
    final paymentUrl = result.data?.paymentUrl ?? '';
    if (result.isError || paymentUrl.isEmpty) {
      setState(() {
        _error = _fundingErrorMessage(
          result,
          'Gagal memproses pembayaran. Coba lagi.',
        );
      });
      return;
    }
    Navigator.of(context).pop(paymentUrl);
  }

  @override
  Widget build(BuildContext context) {
    final intent = widget.intent;
    final disclosure = _disclosure;
    final selected = _selected;
    return Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              _textRow('Durasi', '${widget.durationDays} hari'),
              _textRow(
                'Perkiraan tayangan',
                '± ${AppFormatters.formatNumberInt(intent.estimatedImpressions)}',
              ),
              _summaryRow(
                'Total biaya promosi',
                intent.requiredCost,
                bold: false,
              ),
              _summaryRow(
                'Saldo promo tersedia',
                intent.availableFunding,
                bold: false,
              ),
              _summaryRow('Kekurangan tepat', intent.shortage, bold: true),
              const SizedBox(height: 8),
              Text(
                'Promosi dibuat setelah kekurangan tepat ini dibayar. '
                'Pembayaran tidak menambah saldo promo.',
                style: context.typeRoles.labelMicro.copyWith(
                  color: Theme.of(context).colorScheme.onSurfaceVariant,
                ),
              ),
              const SizedBox(height: 16),
              PaymentMethodTrigger(
                label: 'Metode pembayaran',
                selectedMethodCode: selected?.methodCode,
                selectedMethodDisplayName: selected?.displayName,
                isLoading: _loading,
                hasMethods: (disclosure?.methods ?? const []).isNotEmpty,
                errorMessage: (disclosure?.methods ?? const []).isEmpty
                    ? _error
                    : null,
                onTap: () => unawaited(_pickMethod()),
                onRetry: () => unawaited(_loadDisclosure()),
              ),
              if (selected != null) ...[
                const SizedBox(height: 12),
                _summaryRow(
                  'Biaya layanan',
                  selected.serviceFeeAmount,
                  bold: false,
                ),
                _summaryRow('Total bayar', selected.grossAmount, bold: true),
              ],
              if (_error != null &&
                  (disclosure?.methods ?? const []).isNotEmpty) ...[
                const SizedBox(height: 12),
                Text(
                  _error!,
                  style: context.typeRoles.bodyDense.copyWith(
                    color: Theme.of(context).colorScheme.primary,
                  ),
                ),
              ],
              const SizedBox(height: 20),
              SizedBox(
                width: double.infinity,
                child: ElevatedButton(
                  onPressed: (_paying || selected == null) ? null : _pay,
                  child: _paying
                      ? const SizedBox(
                          height: 20,
                          width: 20,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        )
                      : Text(
                          selected == null
                              ? 'Pilih metode pembayaran'
                              : 'Bayar Kekurangan',
                        ),
                ),
              ),
            ],
    );
  }

  Widget _textRow(String label, String value, {bool bold = false}) {
    final style = context.typeRoles.bodyDense.copyWith(
      fontWeight: bold ? FontWeight.w700 : FontWeight.w400,
    );
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: AppMetrics.p4),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Text(label, style: style),
          Text(value, style: style),
        ],
      ),
    );
  }

  Widget _summaryRow(String label, int amount, {required bool bold}) {
    final style = context.typeRoles.bodyDense.copyWith(
      fontWeight: bold ? FontWeight.w700 : FontWeight.w400,
    );
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: AppMetrics.p4),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Text(label, style: style),
          Text(AppFormatters.formatCurrencyInt(amount), style: style),
        ],
      ),
    );
  }
}
