library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/pricing/promotion/data/dto/promotion_contract_dto.dart';
import 'package:labuda/domains/commerce/pricing/promotion/presentation/providers/canonical_promotion_providers.dart';
import 'package:labuda/domains/finance/transaction/payment/domain/entities/payment.dart'
    show PaymentMethodOption;
import 'package:labuda/domains/finance/transaction/payment/presentation/widgets/payment_method_picker_sheet.dart';
import 'package:labuda/shared/utils/app_formatters.dart';

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
  final _budgetController = TextEditingController(text: '30000');
  final _durationController = TextEditingController(text: '3');
  final _cityIdsController = TextEditingController();
  bool _isLoading = false;

  @override
  void dispose() {
    _budgetController.dispose();
    _durationController.dispose();
    _cityIdsController.dispose();
    super.dispose();
  }

  List<String> _parsedCityIds() => _cityIdsController.text
      .split(',')
      .map((e) => e.trim())
      .where((e) => e.isNotEmpty)
      .toList();

  Future<void> _submit() async {
    if (!_formKey.currentState!.validate()) return;
    final budget = int.tryParse(_budgetController.text) ?? 0;
    final duration = int.tryParse(_durationController.text) ?? 0;
    final cityIds = _parsedCityIds();

    setState(() => _isLoading = true);

    // Step 1: the canonical funding gate. The backend decides whether a payment
    // is required and for exactly how much. When reusable PROMOTE_BALANCE
    // already covers the cost the backend creates nothing (no intent, no
    // billing) and reports payment_required = false.
    final gate = await _requestFundingIntent(budget, duration, cityIds);
    if (!mounted) return;
    if (gate == null) {
      setState(() => _isLoading = false);
      return;
    }

    // Step 2: reuse-first — sufficient reusable funding, no payment at all.
    if (!gate.paymentRequired) {
      await _createContract(budget, duration, cityIds);
      return;
    }

    // Step 3: exact shortage → the seller pays exactly the shortage through the
    // canonical disclosure + initiation, inside the canonical payment WebView.
    setState(() => _isLoading = false);
    final paid = await _openFundingPaymentSheet(gate);
    if (!mounted || paid != true) return;

    // Step 4: after the payment attempt, re-run the same canonical gate. A
    // settled payment credits reusable PROMOTE_BALANCE, so the gate reports no
    // payment required and Create proceeds. A still-pending payment leaves the
    // gate unchanged and the seller retries — the backend reuses the same
    // intent/billing, so retrying never duplicates an obligation or a payment.
    setState(() => _isLoading = true);
    final afterPayment = await _requestFundingIntent(budget, duration, cityIds);
    if (!mounted) return;
    if (afterPayment == null) {
      setState(() => _isLoading = false);
      return;
    }
    if (afterPayment.paymentRequired) {
      setState(() => _isLoading = false);
      ref.invalidate(promoteBalanceProvider);
      _showMessage(
        'Pembayaran belum terkonfirmasi. Tunggu sebentar, lalu coba lagi.',
      );
      return;
    }
    await _createContract(budget, duration, cityIds);
  }

  /// Asks the backend for this promotion's funding obligation (or the absence
  /// of one). Returns null — after showing the backend error — when it failed.
  Future<PromotionFundingIntentDto?> _requestFundingIntent(
    int budget,
    int duration,
    List<String> cityIds,
  ) async {
    final repo = ref.read(promotionContractRepositoryProvider);
    final result = await repo.createFundingIntent(
      kind: _kind,
      budgetRupiah: budget,
      durationDays: duration,
      cityIds: cityIds,
    );
    if (result.isSuccess && result.data != null) return result.data;
    _showMessage(result.error ?? 'Gagal menghitung kebutuhan dana promosi');
    return null;
  }

  /// Canonical Create. Only reachable when the backend reports that no payment
  /// is required, so an underfunded promotion can never be created.
  Future<void> _createContract(
    int budget,
    int duration,
    List<String> cityIds,
  ) async {
    final repo = ref.read(promotionContractRepositoryProvider);
    final result = await repo.createContract(
      kind: _kind,
      budgetRupiah: budget,
      durationDays: duration,
      cityIds: cityIds,
    );
    if (!mounted) return;
    setState(() => _isLoading = false);
    if (result.isSuccess) {
      _showMessage('Promosi berhasil dibuat');
      ref.invalidate(myPromotionContractsProvider);
      // Reusable funding changed (PROMOTE_BALANCE → PROMOTION_ALLOCATION).
      ref.invalidate(promoteBalanceProvider);
      context.pop();
    } else {
      _showMessage(result.error ?? 'Gagal membuat promosi');
    }
  }

  void _showMessage(String message) {
    ScaffoldMessenger.of(
      context,
    ).showSnackBar(SnackBar(content: Text(message)));
  }

  /// Opens the canonical exact-shortage payment surface and, when a payment was
  /// initiated, the canonical internal payment WebView. Returns true once the
  /// WebView closed on an initiated payment, null otherwise.
  Future<bool?> _openFundingPaymentSheet(
    PromotionFundingIntentDto intent,
  ) async {
    final paymentUrl = await showModalBottomSheet<String>(
      context: context,
      isScrollControlled: true,
      backgroundColor: Colors.transparent,
      builder: (_) => _PromotionFundingPaymentSheet(intent: intent),
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
      body: SingleChildScrollView(
        padding: const EdgeInsets.all(16),
        child: Form(
          key: _formKey,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              _ReusableFundingCard(balanceAsync: balanceAsync),
              const SizedBox(height: 16),
              DropdownButtonFormField<String>(
                value: _kind,
                decoration: const InputDecoration(labelText: 'Jenis Promosi'),
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
                onChanged: (v) => setState(() => _kind = v!),
              ),
              const SizedBox(height: 12),
              TextFormField(
                controller: _budgetController,
                decoration: const InputDecoration(
                  labelText: 'Budget (Rupiah)',
                  hintText: '30000',
                ),
                keyboardType: TextInputType.number,
                validator: (v) => (int.tryParse(v ?? '') ?? 0) <= 0
                    ? 'Budget harus >0'
                    : null,
              ),
              const SizedBox(height: 12),
              TextFormField(
                controller: _durationController,
                decoration: const InputDecoration(
                  labelText: 'Duration (hari)',
                  hintText: '3',
                ),
                keyboardType: TextInputType.number,
                validator: (v) => (int.tryParse(v ?? '') ?? 0) <= 0
                    ? 'Duration harus >0'
                    : null,
              ),
              const SizedBox(height: 12),
              TextFormField(
                controller: _cityIdsController,
                decoration: const InputDecoration(
                  labelText: 'City IDs (kosong = nasional)',
                  hintText: '3204,3171,5103',
                ),
              ),
              const SizedBox(height: 8),
              Text(
                'Kosong = nasional (unrestricted). Isi = arbitrary city set, contoh 3204=Bandung, 3171=Jaksel.',
                style: TextStyle(fontSize: 12, color: Colors.grey[600]),
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
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: AppColors.neutralGray200),
        color: Colors.white,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            'Saldo promo tersedia',
            style: TextStyle(fontSize: 12, color: AppColors.neutralGray600),
          ),
          const SizedBox(height: 6),
          balanceAsync.when(
            data: (result) {
              if (result.isSuccess && result.data != null) {
                return Text(
                  AppFormatters.formatCurrencyInt(result.data!.balance),
                  style: const TextStyle(
                    fontSize: 20,
                    fontWeight: FontWeight.bold,
                  ),
                );
              }
              return Text(
                result.error ?? 'Gagal memuat saldo promosi',
                style: TextStyle(fontSize: 14, color: AppColors.primaryRed),
              );
            },
            loading: () => const SizedBox(
              height: 20,
              width: 20,
              child: CircularProgressIndicator(strokeWidth: 2),
            ),
            error: (e, _) => Text(
              e.toString(),
              style: TextStyle(fontSize: 14, color: AppColors.primaryRed),
            ),
          ),
          const SizedBox(height: 4),
          Text(
            'Saldo ini bisa langsung dipakai untuk membuat promosi tanpa '
            'pembayaran baru.',
            style: TextStyle(fontSize: 12, color: AppColors.neutralGray500),
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

  const _PromotionFundingPaymentSheet({required this.intent});

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
    final code = await PaymentMethodPickerSheet.show(context, methods: options);
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
    return SafeArea(
      child: Container(
        constraints: BoxConstraints(
          maxHeight: MediaQuery.of(context).size.height * 0.9,
        ),
        decoration: const BoxDecoration(
          color: Colors.white,
          borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
        ),
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(16),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const Text(
                'Kekurangan dana promosi',
                style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold),
              ),
              const SizedBox(height: 12),
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
                style: TextStyle(fontSize: 12, color: AppColors.neutralGray600),
              ),
              const SizedBox(height: 16),
              const Text(
                'Metode pembayaran',
                style: TextStyle(fontSize: 13, fontWeight: FontWeight.w600),
              ),
              const SizedBox(height: 6),
              _methodSelector(disclosure, selected),
              if (selected != null) ...[
                const SizedBox(height: 12),
                _summaryRow(
                  'Biaya layanan',
                  selected.serviceFeeAmount,
                  bold: false,
                ),
                _summaryRow('Total bayar', selected.grossAmount, bold: true),
              ],
              if (_error != null) ...[
                const SizedBox(height: 12),
                Text(
                  _error!,
                  style: TextStyle(fontSize: 13, color: AppColors.primaryRed),
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
          ),
        ),
      ),
    );
  }

  Widget _summaryRow(String label, int amount, {required bool bold}) {
    final style = TextStyle(
      fontSize: 14,
      fontWeight: bold ? FontWeight.w700 : FontWeight.w400,
    );
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 2),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Text(label, style: style),
          Text(AppFormatters.formatCurrencyInt(amount), style: style),
        ],
      ),
    );
  }

  Widget _methodSelector(
    PromotionFundingPaymentMethodsDto? disclosure,
    PromotionFundingPaymentMethodDto? selected,
  ) {
    if (_loading) {
      return const Padding(
        padding: EdgeInsets.symmetric(vertical: 8),
        child: LinearProgressIndicator(),
      );
    }
    final methods = disclosure?.methods ?? const [];
    if (methods.isEmpty) {
      return Row(
        children: [
          const Expanded(child: Text('Metode pembayaran tidak tersedia')),
          TextButton(
            onPressed: _loadDisclosure,
            child: const Text('Coba lagi'),
          ),
        ],
      );
    }
    return InkWell(
      onTap: _pickMethod,
      borderRadius: BorderRadius.circular(8),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 12),
        decoration: BoxDecoration(
          borderRadius: BorderRadius.circular(8),
          border: Border.all(color: AppColors.neutralGray300),
        ),
        child: Row(
          children: [
            Expanded(
              child: Text(
                selected?.displayName ?? 'Pilih metode pembayaran',
                style: const TextStyle(fontSize: 14),
              ),
            ),
            Icon(
              Icons.chevron_right,
              size: 20,
              color: AppColors.neutralGray600,
            ),
          ],
        ),
      ),
    );
  }
}
