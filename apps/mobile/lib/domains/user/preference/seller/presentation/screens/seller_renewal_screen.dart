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
    // COPY-AUTHORITY: "Aktivasi" wording is only correct when the seller has
    // NEVER had an interval (baseline null — GET /seller/subscription 404).
    // Expired-interval renewal IS still a renewal ("Perpanjangan"), and early
    // renewal likewise. Audit finding C.
    final firstActivation = baseline == null;
    // Batch 2 re-entry loop: settlement can outlive the 60s polling window
    // (VA takes minutes-hours). After the user closes the pending dialog, the
    // renewal screen must keep offering a manual status check until the
    // backend confirms — the seller is never stranded on the form.
    //
    // successHandled breaks the loop the moment success was surfaced once —
    // onSuccess may pop this screen, and the loop must never re-run its
    // body after that (double snackbar / double pop).
    var successHandled = false;
    var firstIteration = true;
    while (mounted && !successHandled && _isCurrent(epoch, uid)) {
      if (firstIteration) {
        // Right after submit: the auto-polling window is the check.
        firstIteration = false;
      } else {
        // Manual re-entry: check backend truth FIRST ("Cek status" must
        // check, not reopen the polling dialog), then offer to poll again.
        final confirmed = await _checkPaymentConfirmed(epoch, uid, baseline);
        if (successHandled || !mounted || !_isCurrent(epoch, uid)) return;
        if (confirmed) {
          successHandled = true;
          AppSnackBar.showSuccess(
            context,
            firstActivation
                ? 'Aktivasi seller berhasil — Anda sudah bisa jual dan lelang'
                : 'Perpanjangan seller berhasil diproses',
          );
          Navigator.of(context).pop(true);
          return;
        }
        final recheck = await showDialog<bool>(
          context: context,
          builder: (ctx) => AlertDialog(
            title: const Text('Pembayaran masih diproses'),
            content: const Text(
              'Pembayaran Anda belum terkonfirmasi. Cek ulang statusnya sekarang?',
            ),
            actions: [
              TextButton(
                onPressed: () => Navigator.of(ctx).pop(false),
                child: const Text('Nanti saja'),
              ),
              TextButton(
                onPressed: () => Navigator.of(ctx).pop(true),
                child: const Text('Cek status'),
              ),
            ],
          ),
        );
        if (recheck != true) return;
        // "Cek status" must CHECK, not reopen the polling dialog: continue
        // so the next iteration runs the manual check first.
        continue;
      }

      await showDialog<void>(
        context: context,
        barrierDismissible: false,
        builder: (ctx) => _RenewalPendingDialog(
          epoch: epoch,
          uid: uid,
          baseline: baseline,
          isCurrent: () => _isCurrent(epoch, uid),
          onSuccess: () async {
            successHandled = true;
            if (Navigator.of(ctx).canPop()) Navigator.of(ctx).pop();
            if (mounted) {
              AppSnackBar.showSuccess(
                context,
                firstActivation
                    ? 'Aktivasi seller berhasil — Anda sudah bisa jual dan lelang'
                    : 'Perpanjangan seller berhasil diproses',
              );
              Navigator.of(context).pop(true);
            }
          },
        ),
      );
      if (successHandled || !mounted || !_isCurrent(epoch, uid)) return;
    }
  }

  /// Manual status check for the Batch 2 re-entry loop. Authority transition
  /// alone confirms first activation; renewal additionally requires the
  /// interval expiry to have moved forward.
  Future<bool> _checkPaymentConfirmed(
    int epoch,
    String uid,
    SellerSubscription? baseline,
  ) async {
    await ref.read(authControllerProvider.notifier).forceRefreshAuthState();
    if (!mounted || !_isCurrent(epoch, uid)) return false;
    final s = ref.read(authControllerProvider);
    if (s is! AuthStateAuthenticated || s.user.hasMarketAuthority != true) {
      return false;
    }
    if (baseline == null) return true;
    try {
      final r = await ref.read(sellerRepositoryProvider).getSubscription(uid);
      return r.isSuccess &&
          r.data != null &&
          r.data!.expiryDate.isAfter(baseline.expiryDate);
    } catch (_) {
      return false;
    }
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
    // COPY-AUTHORITY (Batch 3): "Aktifkan" wording for sellers who never had
    // an interval (pendingActivation); "Perpanjang" only for real renewals.
    final activationMode = sellerState.isPendingActivation;
    return Scaffold(
      appBar: AppBarCustom(
        title: activationMode ? 'Aktifkan Langganan Seller' : 'Perpanjang Seller',
        leading: IconButton(icon: const Icon(Icons.close), onPressed: () => Navigator.of(context).pop()),
      ),
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
    // COPY-AUTHORITY mirrors build(): "Aktifkan" for sellers who never had an
    // interval, "Perpanjang" for real renewals.
    final activationMode = sellerState.isPendingActivation;
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
            Text(
              sellerState.isExpired
                  ? 'Mode perpanjang'
                  : activationMode
                      ? 'Mode aktivasi'
                      : 'Mode perpanjang dini',
              style: TextStyle(fontSize: 14, fontWeight: FontWeight.w700, color: isDark ? AppColors.neutralWhite : AppColors.neutralGray900),
            ),
            const SizedBox(height: 6),
            Text(
              sellerState.isExpired
                  ? 'Profil seller terdeteksi. Perpanjang untuk memulihkan otoritas jualan tanpa membuat identitas baru.'
                  : activationMode
                      ? 'Profil seller terdeteksi. Aktifkan langganan untuk mulai jual dan lelang — identitas seller Anda tetap dipakai.'
                      : 'Profil seller terdeteksi. Perpanjang dini menjaga identitas seller Anda tetap utuh.',
              style: TextStyle(fontSize: 13, color: isDark ? AppColors.neutralGray200 : AppColors.neutralGray700),
            ),
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
        SizedBox(width: double.infinity, child: ElevatedButton(onPressed: _submitting ? null : _submit, child: _submitting ? const SizedBox(height: 20, width: 20, child: CircularProgressIndicator(strokeWidth: 2)) : Text(activationMode ? 'Bayar & Aktifkan' : 'Bayar & Perpanjang'))),
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

class _RenewalPendingDialogState extends ConsumerState<_RenewalPendingDialog>
    with WidgetsBindingObserver {
  Timer? _timer;
  bool _timedOut = false;
  int _attempts = 0;

  @override
  void initState() {
    super.initState();
    // Batch 2: settlement can land while the dialog is backgrounded (user
    // switches to m-banking / wallet app). Resume is the natural moment the
    // truth changed — refresh immediately instead of waiting for the next 3s
    // tick or the 60s timeout.
    WidgetsBinding.instance.addObserver(this);
    _start();
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    _timer?.cancel();
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state != AppLifecycleState.resumed) return;
    if (!widget.isCurrent()) return;
    // Refresh once per resume; the periodic poll handles the rest.
    ref.read(authControllerProvider.notifier).forceRefreshAuthState();
  }
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
        // CANONICAL SUCCESS DETECTION — first activation (P1 fix):
        // baseline null means NO interval existed when the dialog opened
        // (GET /seller/subscription → 404), so the appearance of
        // hasMarketAuthority IS success: an active seller_subscriptions
        // interval can only be written by ProcessSuccessfulPaymentTx
        // (settled payment). This branch was previously a silent dead-end
        // (a bare return on null baseline, without cancelling the timer),
        // so first activation could NEVER show success and always fell
        // through to the 60s timeout.
        final baseline = widget.baseline;
        if (baseline == null) {
          t.cancel();
          await widget.onSuccess();
          return;
        }
        // Renewal (baseline existed — active or expired): confirm the
        // interval window actually moved forward.
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
        title: const Text('Memproses pembayaran'),
        content: Column(mainAxisSize: MainAxisSize.min, crossAxisAlignment: CrossAxisAlignment.start, children: [
          const LinearProgressIndicator(),
          const SizedBox(height: 16),
          Text(_timedOut
              ? 'Pembayaran masih diproses. Anda bisa menutup dialog ini dan memeriksa lagi nanti.'
              : 'Kami menunggu konfirmasi pembayaran dan aktivasi seller Anda.'),
        ]),
        actions: [
          // Batch 2: manual re-entry point. Settlement can outlive the polling
          // window (VA can take minutes-hours), so the user must never be
          // left without a way to close this dialog and re-check. Closing
          // keeps the renewal screen open for a fresh initiate (the backend
          // reuses the same pending payment idempotently).
          TextButton(
            onPressed: () => Navigator.of(context).pop(),
            child: const Text('Cek status pembayaran'),
          ),
        ],
      );
}
