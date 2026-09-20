import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/config/seller_upgrade_config_entity.dart';
import 'package:labuda/core/config/seller_upgrade_config_provider.dart' as config;
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/finance/transaction/payment/domain/entities/payment.dart'
    show PaymentMethodOption;
import 'package:labuda/domains/finance/transaction/payment/presentation/widgets/payment_method_picker_sheet.dart';
import 'package:labuda/domains/user/preference/seller/data/dto/seller_dto.dart';
import 'package:labuda/domains/user/preference/seller/data/seller_providers.dart'
    show sellerRemoteDatasourceProvider, sellerRepositoryProvider;
import 'package:labuda/domains/user/preference/seller/domain/entities/seller_state.dart';
import 'package:labuda/domains/user/preference/seller/domain/entities/seller_subscription.dart';
import 'package:labuda/shared/shared.dart';

/// Canonical seller renewal lifecycle.
///
/// **PAYMENT-ONLY** for users who are ALREADY sellers:
/// read-only subscription context → payment method → subscription payment
/// → Payment WebView → status polling → renewal result.
///
/// Forbidden here: seller onboarding (`POST /seller/onboarding`), any seller or
/// store profile mutation, registration terms, and registration wizard steps.
/// Seller/store edits belong to seller profile management, never to renewal.
class SellerRenewalScreen extends ConsumerStatefulWidget {
  const SellerRenewalScreen({super.key});

  @override
  ConsumerState<SellerRenewalScreen> createState() => _SellerRenewalScreenState();
}

class _SellerRenewalScreenState extends ConsumerState<SellerRenewalScreen> {
  SellerSubscriptionPaymentMethodsDto? _methods;
  SellerSubscriptionPaymentMethodDto? _selected;
  bool _loadingMethods = false;
  String? _methodsError;
  bool _submitting = false;
  int _principalEpoch = 0;
  ProviderSubscription<AuthState>? _authSub;

  @override
  void initState() {
    super.initState();
    _authSub = ref.listenManual<AuthState>(authControllerProvider, (prev, next) {
      final prevId = _principalId(prev);
      final nextId = _principalId(next);
      if (prevId != nextId) {
        _principalEpoch++;
        if (mounted) setState(() {});
      }
    }, fireImmediately: true);
    WidgetsBinding.instance.addPostFrameCallback((_) => _loadMethods());
  }

  String? _principalId(AuthState? s) {
    return switch (s) {
      AuthStateAuthenticated(:final user) => user.id,
      AuthStateLoading(:final principal) => principal?.uid,
      AuthStateFirebaseAuthenticated(:final userId) => userId,
      AuthStateSyncingWithBackend(:final userId) => userId,
      AuthStateRequiresProfileCompletion(:final userId) => userId,
      AuthStateAccountRestricted(:final user) => user.id,
      _ => null,
    };
  }

  String? _currentUserId() {
    final s = ref.read(authControllerProvider);
    if (s is AuthStateAuthenticated) return s.user.id;
    return null;
  }

  bool _isCurrent(int epoch, String? uid) {
    if (!mounted) return false;
    if (epoch != _principalEpoch) return false;
    return _currentUserId() == uid;
  }

  Future<void> _loadMethods() async {
    if (!mounted || _loadingMethods || _methods != null) return;
    setState(() {
      _loadingMethods = true;
      _methodsError = null;
    });
    try {
      final m = await ref.read(sellerRemoteDatasourceProvider).getSubscriptionPaymentMethods();
      if (!mounted) return;
      setState(() {
        _methods = m;
        _loadingMethods = false;
      });
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() {
        _loadingMethods = false;
        _methodsError = e.code == 'NO_ACTIVE_CONFIG' ? 'Konfigurasi langganan belum tersedia.' : 'Gagal memuat metode pembayaran.';
      });
    } catch (_) {
      if (!mounted) return;
      setState(() {
        _loadingMethods = false;
        _methodsError = 'Gagal memuat metode pembayaran.';
      });
    }
  }

  Future<void> _pickMethod() async {
    final available = _methods?.methods ?? const [];
    if (available.isEmpty) return;
    final options = available
        .map((m) => PaymentMethodOption(
              methodCode: m.methodCode,
              displayName: m.displayName,
              buyerPaymentFeeAmount: m.serviceFeeAmount,
              totalPayableAmount: m.grossAmount,
            ))
        .toList();
    final code = await PaymentMethodPickerSheet.show(context, methods: options);
    if (!mounted || code == null) return;
    for (final m in available) {
      if (m.methodCode == code) {
        setState(() => _selected = m);
        return;
      }
    }
  }

  Future<SellerSubscription?> _loadBaseline(String uid) async {
    try {
      final repo = ref.read(sellerRepositoryProvider);
      final r = await repo.getSubscription(uid);
      if (r.isSuccess && r.data != null) return r.data;
    } catch (_) {}
    return null;
  }

  Future<void> _submit() async {
    if (_submitting) return;
    if (_selected == null) {
      AppSnackBar.showError(context, 'Pilih metode pembayaran terlebih dahulu');
      return;
    }
    final uid = _currentUserId();
    if (uid == null) {
      AppSnackBar.showError(context, 'User not authenticated');
      return;
    }
    setState(() => _submitting = true);
    final epoch = _principalEpoch;
    final baseline = await _loadBaseline(uid);
    if (!mounted) return;
    showDialog(context: context, barrierDismissible: false, builder: (_) => const Center(child: CircularProgressIndicator()));
    try {
      final data = await ref.read(sellerRemoteDatasourceProvider).initiateSubscriptionPayment(paymentMethodCode: _selected!.methodCode);
      if (mounted && Navigator.of(context).canPop()) Navigator.of(context).pop();
      if (!mounted) return;
      if (!_isCurrent(epoch, uid)) return;
      final url = data['payment_url'] as String?;
      if (url == null || url.isEmpty) {
        AppSnackBar.showError(context, 'Gagal mendapatkan URL pembayaran');
        return;
      }
      if (!mounted) return;
      await context.push('/payment-webview?url=${Uri.encodeComponent(url)}');
      if (!mounted) return;
      await _showPending(epoch, uid, baseline);
    } on ApiException catch (e) {
      if (mounted && Navigator.of(context).canPop()) Navigator.of(context).pop();
      if (!mounted) return;
      AppSnackBar.showError(context, e.message);
    } catch (_) {
      if (mounted && Navigator.of(context).canPop()) Navigator.of(context).pop();
      if (!mounted) return;
      AppSnackBar.showError(context, 'Gagal memproses pembayaran. Coba lagi.');
    } finally {
      if (mounted) setState(() => _submitting = false);
    }
  }

  Future<void> _showPending(int epoch, String uid, SellerSubscription? baseline) async {
    await showDialog<void>(
      context: context,
      barrierDismissible: false,
      builder: (ctx) => _RenewalPendingDialog(
        epoch: epoch,
        uid: uid,
        baseline: baseline,
        isCurrent: () => _isCurrent(epoch, uid),
        onSuccess: () async {
          if (Navigator.of(ctx).canPop()) Navigator.of(ctx).pop();
          if (mounted) {
            AppSnackBar.showSuccess(context, 'Perpanjangan seller berhasil diproses');
            Navigator.of(context).pop(true);
          }
        },
      ),
    );
  }

  @override
  void dispose() {
    _authSub?.close();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final cfgAsync = ref.watch(config.sellerUpgradeConfigProvider);
    final sellerState = SellerState.fromAuthUser(
      ref.watch(authenticatedUserProvider),
    );
    return Scaffold(
      appBar: AppBarCustom(title: 'Perpanjang Seller', leading: IconButton(icon: const Icon(Icons.close), onPressed: () => Navigator.of(context).pop())),
      body: cfgAsync.when(
        data: (cfg) => _buildBody(cfg, isDark, sellerState),
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (e, _) => Center(child: Text('Gagal memuat konfigurasi: $e')),
      ),
    );
  }

  Widget _buildBody(
    SellerUpgradeConfigEntity cfg,
    bool isDark,
    SellerState sellerState,
  ) {
    final methods = _methods;
    final principal = (methods?.principalAmount ?? cfg.yearlyFee.round()).toDouble();
    final sel = _selected;
    final fee = (sel?.serviceFeeAmount ?? 0).toDouble();
    return ListView(
      padding: const EdgeInsets.all(16),
      children: [
        Container(
          padding: const EdgeInsets.all(16),
          decoration: BoxDecoration(
            gradient: LinearGradient(colors: [AppColors.successGreen.withValues(alpha: 0.16), AppColors.successGreen.withValues(alpha: 0.05)]),
            borderRadius: BorderRadius.circular(16),
            border: Border.all(color: AppColors.successGreen.withValues(alpha: 0.35)),
          ),
          child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
            Text(sellerState.isExpired ? 'Renewal mode' : 'Early renewal mode', style: TextStyle(fontSize: 14, fontWeight: FontWeight.w700, color: isDark ? AppColors.neutralWhite : AppColors.neutralGray900)),
            const SizedBox(height: 6),
            Text(sellerState.isExpired ? 'Existing seller profile detected. Renew to restore market authority without recreating identity.' : 'Existing seller profile detected. Early renewal keeps your seller identity intact.', style: TextStyle(fontSize: 13, color: isDark ? AppColors.neutralGray200 : AppColors.neutralGray700)),
          ]),
        ),
        const SizedBox(height: 16),
        Container(
          padding: const EdgeInsets.all(16),
          decoration: BoxDecoration(color: isDark ? AppColors.darkGray700 : AppColors.neutralWhite, borderRadius: BorderRadius.circular(12), border: Border.all(color: isDark ? AppColors.darkGray600 : AppColors.neutralGray200)),
          child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
            Text('Seller Payment Summary', style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold, color: isDark ? AppColors.neutralGray200 : AppColors.neutralGray900)),
            const SizedBox(height: 16),
            Row(mainAxisAlignment: MainAxisAlignment.spaceBetween, children: [Text('Yearly subscription', style: TextStyle(fontSize: 14, color: isDark ? AppColors.neutralGray300 : AppColors.neutralGray700)), Text('Rp ${AppFormatters.formatCurrency(principal)}', style: const TextStyle(fontWeight: FontWeight.w600))]),
            const SizedBox(height: 12),
            _buildMethodSelector(isDark),
            if (sel != null) ...[
              const SizedBox(height: 12),
              Row(mainAxisAlignment: MainAxisAlignment.spaceBetween, children: [Text('Payment method fee', style: TextStyle(fontSize: 14, color: isDark ? AppColors.neutralGray300 : AppColors.neutralGray700)), Text('Rp ${AppFormatters.formatCurrency(fee)}')]),
            ],
            const Divider(height: 24),
            Row(mainAxisAlignment: MainAxisAlignment.spaceBetween, children: [Text('Total', style: const TextStyle(fontWeight: FontWeight.bold)), Text(sel == null ? 'Belum dipilih' : 'Rp ${AppFormatters.formatCurrency(sel.grossAmount.toDouble())}', style: const TextStyle(fontWeight: FontWeight.bold))]),
          ]),
        ),
        const SizedBox(height: 24),
        SizedBox(width: double.infinity, child: ElevatedButton(onPressed: _submitting ? null : _submit, child: _submitting ? const SizedBox(height: 20, width: 20, child: CircularProgressIndicator(strokeWidth: 2)) : const Text('Bayar & Perpanjang'))),
      ],
    );
  }

  Widget _buildMethodSelector(bool isDark) {
    final methods = _methods?.methods ?? const [];
    final sel = _selected;
    final loading = _loadingMethods;
    final err = _methodsError;
    final label = loading ? 'Memuat metode pembayaran...' : methods.isEmpty ? (err ?? 'Tidak ada metode pembayaran tersedia') : (sel?.displayName ?? 'Pilih metode pembayaran');
    return Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
      Text('Payment method', style: TextStyle(fontSize: 13, fontWeight: FontWeight.w600, color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray600)),
      const SizedBox(height: 6),
      InkWell(
        onTap: loading ? null : methods.isEmpty ? () => _loadMethods() : () => _pickMethod(),
        borderRadius: BorderRadius.circular(8),
        child: Container(
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 12),
          decoration: BoxDecoration(borderRadius: BorderRadius.circular(8), border: Border.all(color: isDark ? AppColors.darkGray600 : AppColors.neutralGray300)),
          child: Row(children: [Expanded(child: Text(label, style: TextStyle(fontSize: 14, color: methods.isEmpty && !loading ? AppColors.statusError : (isDark ? AppColors.neutralGray200 : AppColors.neutralGray900)))), if (!loading) Icon(Icons.chevron_right, size: 20, color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray600)]),
        ),
      ),
    ]);
  }
}

class _RenewalPendingDialog extends ConsumerStatefulWidget {
  final int epoch;
  final String uid;
  final SellerSubscription? baseline;
  final bool Function() isCurrent;
  final Future<void> Function() onSuccess;
  const _RenewalPendingDialog({required this.epoch, required this.uid, required this.baseline, required this.isCurrent, required this.onSuccess});
  @override
  ConsumerState<_RenewalPendingDialog> createState() => _RenewalPendingDialogState();
}

class _RenewalPendingDialogState extends ConsumerState<_RenewalPendingDialog> {
  Timer? _timer;
  bool _timedOut = false;
  int _attempts = 0;
  @override
  void initState() { super.initState(); _start(); }
  @override
  void dispose() { _timer?.cancel(); super.dispose(); }
  Future<void> _start() async {
    _timer = Timer.periodic(const Duration(seconds: 3), (t) async {
      if (!mounted) { t.cancel(); return; }
      if (_attempts >= 20) { t.cancel(); setState(() => _timedOut = true); return; }
      if (!widget.isCurrent()) { t.cancel(); if (mounted && Navigator.of(context).canPop()) Navigator.of(context).pop(); return; }
      _attempts++;
      await ref.read(authControllerProvider.notifier).forceRefreshAuthState();
      if (!mounted) { t.cancel(); return; }
      if (!widget.isCurrent()) { t.cancel(); if (mounted && Navigator.of(context).canPop()) Navigator.of(context).pop(); return; }
      final s = ref.read(authControllerProvider);
      if (s is AuthStateAuthenticated && s.user.hasMarketAuthority == true) {
        final baseline = widget.baseline;
        if (baseline == null) return;
        final cur = await _refresh();
        if (!mounted) { t.cancel(); return; }
        if (!widget.isCurrent()) { t.cancel(); if (mounted && Navigator.of(context).canPop()) Navigator.of(context).pop(); return; }
        if (cur != null && cur.expiryDate.isAfter(baseline.expiryDate)) { t.cancel(); await widget.onSuccess(); }
      }
    });
  }
  Future<SellerSubscription?> _refresh() async {
    try {
      final r = await ref.read(sellerRepositoryProvider).getSubscription(widget.uid);
      if (r.isSuccess && r.data != null) return r.data;
    } catch (_) {}
    return null;
  }
  @override
  Widget build(BuildContext context) => AlertDialog(
        title: const Text('Processing payment'),
        content: Column(mainAxisSize: MainAxisSize.min, crossAxisAlignment: CrossAxisAlignment.start, children: [const LinearProgressIndicator(), const SizedBox(height: 16), Text(_timedOut ? 'Payment is still being processed. You can close this dialog and check again later.' : 'We are waiting for payment confirmation and seller activation.')]),
        actions: [if (_timedOut) TextButton(onPressed: () => Navigator.of(context).pop(), child: const Text('Close'))],
      );
}
