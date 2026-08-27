/// Payment Result Screen
///
/// HARDENED IMPLEMENTATION with safety mechanisms:
/// - Backend authority for payment status
/// - Proper state management via Riverpod
/// - Race condition guards for polling overlap
/// - Cancellation token for graceful cleanup
/// - Explicit error states (timeout, network error, failed)
///
/// BACKEND IS THE SINGLE SOURCE OF TRUTH for payment status.
/// Frontend ONLY displays what backend confirms.
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart' as core;
import 'package:labuda/domains/finance/transaction/payment/presentation/providers/payment_result_notifier.dart';
import 'package:labuda/domains/finance/transaction/payment/presentation/providers/payment_result_state.dart'
    show PaymentResultScreenStatus, PaymentResultState;
import 'package:labuda/domains/system/support/presentation/screens/help_center_screen.dart';
import 'package:labuda/domains/system/support/presentation/widgets/pre_chat_form_sheet.dart';
import 'package:labuda/domains/user/identity/authentication/authentication.dart';
import 'package:url_launcher/url_launcher.dart';

part 'payment_result_screen_sections.dart';

/// Payment Result Screen - Hardened Implementation
///
/// This screen handles the critical payment result reconciliation flow.
/// It polls the backend for payment status and displays the result.
///
/// SAFETY MECHANISMS:
/// 1. All state managed by Riverpod notifier - no local state races
/// 2. Proper cleanup in dispose via notifier.stopChecking()
/// 3. Explicit error states - no silent failures
/// 4. Backend authority - status only from backend order entity
///
/// **CV2:** returnToChat enables seamless navigation back to chat after payment
class PaymentResultScreen extends ConsumerStatefulWidget {
  final String orderId;
  final String? orderNumber;

  /// **CV2:** Chat ID to return to after successful payment
  /// When set, a "Kembali ke Chat" button will be shown
  final String? returnToChat;

  const PaymentResultScreen({
    super.key,
    required this.orderId,
    this.orderNumber,
    this.returnToChat,
  });

  @override
  ConsumerState<PaymentResultScreen> createState() =>
      _PaymentResultScreenState();
}

class _PaymentResultScreenState extends ConsumerState<PaymentResultScreen>
    with WidgetsBindingObserver {
  // Cached in initState - `ref` is unsafe to use inside dispose() once the
  // element has begun unmounting, so the notifier reference is captured
  // once, up front, while `ref` is still guaranteed valid.
  late final PaymentResultNotifier _notifier;

  @override
  void initState() {
    super.initState();
    _notifier = ref.read(paymentResultProvider.notifier);
    WidgetsBinding.instance.addObserver(this);

    // Start checking payment status after first frame
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _notifier.startChecking(widget.orderId);
    });
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    // SAFETY: Always stop checking when screen is disposed
    // This prevents memory leaks and ghost polling
    _notifier.stopChecking();
    super.dispose();
  }

  /// App-resume hook: when the user returns from the external payment
  /// browser/app, trigger a status-only recheck. Never opens the payment
  /// URL - only [_handleContinuePayment] does that, on explicit user tap.
  @override
  void didChangeAppLifecycleState(AppLifecycleState lifecycleState) {
    if (lifecycleState == AppLifecycleState.resumed && mounted) {
      _notifier.recheckOnResume(widget.orderId);
    }
  }

  /// Navigate to order detail
  void _goToOrderDetail() {
    context.go('/orders/${widget.orderId}');
  }

  /// Navigate back to home
  void _goToHome() {
    context.go(core.RoutePaths.home);
  }

  /// **CV2:** Navigate back to chat after successful payment
  /// This provides chat-commerce continuity for transactions that started in chat
  void _goToChat() {
    if (widget.returnToChat == null || widget.returnToChat!.isEmpty) {
      // Fallback to home if no chat ID
      _goToHome();
      return;
    }
    // Navigate to chat detail screen
    context.go('/chat/${widget.returnToChat}');
  }

  /// Status-only recheck ("Cek Status Lagi" / "Coba Lagi").
  ///
  /// Never opens the payment URL - it only re-asks the backend for the
  /// current order/payment status. Reopening the browser is a distinct,
  /// explicit action (see [_handleContinuePayment]).
  Future<void> _handleStatusCheck() async {
    await _notifier.retry(widget.orderId);
  }

  /// Continue an existing payment ("Lanjutkan Pembayaran").
  ///
  /// Explicitly reopens the reusable, non-expired payment_url. Does not
  /// trigger a status check itself - the user is going to pay, not check.
  Future<void> _handleContinuePayment() async {
    final url = ref.read(paymentResultProvider).paymentUrl;
    if (url == null || url.isEmpty) return;
    await _openExistingPaymentUrl(url);
  }

  /// Reopen the original payment URL when the backend gives us one.
  Future<void> _openExistingPaymentUrl(String paymentUrl) async {
    if (paymentUrl.isEmpty) return;

    try {
      final uri = Uri.parse(paymentUrl);
      if (await canLaunchUrl(uri)) {
        await launchUrl(uri, mode: LaunchMode.externalApplication);
      }
    } catch (_) {
      // Fall back to polling retry; the status flow will still recover if the
      // payment was completed elsewhere.
    }
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    // Watch the payment result state
    final state = ref.watch(paymentResultProvider);

    return Scaffold(
      backgroundColor: isDark
          ? core.AppColors.darkGray900
          : core.AppColors.neutralGray50,
      body: SafeArea(
        child: Center(
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(24),
            child: _buildContent(state, isDark),
          ),
        ),
      ),
    );
  }

  Widget _buildContent(PaymentResultState state, bool isDark) {
    switch (state.status) {
      case PaymentResultScreenStatus.checking:
        return _buildCheckingContent(state, isDark);

      case PaymentResultScreenStatus.success:
        return _buildSuccessContent(state, isDark);

      case PaymentResultScreenStatus.failed:
        return _buildFailedContent(state, isDark);

      case PaymentResultScreenStatus.timeout:
        return _buildTimeoutContent(state, isDark);

      case PaymentResultScreenStatus.networkError:
        return _buildNetworkErrorContent(state, isDark);
    }
  }

  /// Checking state - actively polling backend
  Widget _buildCheckingContent(PaymentResultState state, bool isDark) {
    final elapsedMessage = _getElapsedTimeMessage(state);

    return Column(
      mainAxisAlignment: MainAxisAlignment.center,
      children: [
        // Animated spinner
        const SizedBox(
          width: 80,
          height: 80,
          child: CircularProgressIndicator(
            strokeWidth: 4,
            valueColor: AlwaysStoppedAnimation<Color>(
              core.AppColors.primaryRed,
            ),
          ),
        ),
        const SizedBox(height: 32),

        // Title
        Text(
          'Menunggu Konfirmasi Pembayaran',
          style: TextStyle(
            fontSize: 24,
            fontWeight: FontWeight.bold,
            color: isDark
                ? core.AppColors.neutralWhite
                : core.AppColors.neutralGray900,
          ),
        ),
        const SizedBox(height: 16),

        // Subtitle
        Text(
          'Mohon tunggu, kami sedang mengecek status pembayaran Anda...\nOrder: ${widget.orderNumber ?? widget.orderId}',
          textAlign: TextAlign.center,
          style: TextStyle(
            fontSize: 16,
            color: isDark
                ? core.AppColors.neutralGray400
                : core.AppColors.neutralGray600,
          ),
        ),
        const SizedBox(height: 8),

        // Poll counter - shows transparency to user
        Text(
          'Pengecekan ke ${state.pollAttempts + 1}/${state.maxPollAttempts}',
          style: TextStyle(
            fontSize: 14,
            color: isDark
                ? core.AppColors.neutralGray500
                : core.AppColors.neutralGray400,
          ),
        ),

        // Elapsed time-based message (shows after 15s)
        if (elapsedMessage.isNotEmpty) ...[
          const SizedBox(height: 16),
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
            decoration: BoxDecoration(
              color: core.AppColors.statusWarning.withValues(alpha: 0.1),
              borderRadius: BorderRadius.circular(8),
              border: Border.all(
                color: core.AppColors.statusWarning.withValues(alpha: 0.3),
              ),
            ),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(
                  Icons.info_outline,
                  size: 18,
                  color: core.AppColors.statusWarning,
                ),
                const SizedBox(width: 8),
                Flexible(
                  child: Text(
                    elapsedMessage,
                    textAlign: TextAlign.center,
                    style: TextStyle(
                      fontSize: 14,
                      color: isDark
                          ? core.AppColors.neutralGray300
                          : core.AppColors.neutralGray700,
                    ),
                  ),
                ),
              ],
            ),
          ),
        ],

        const SizedBox(height: 32),

        // Status-only recheck - allows manual refresh during processing
        SizedBox(
          width: double.infinity,
          child: OutlinedButton.icon(
            onPressed: state.isChecking ? null : _handleStatusCheck,
            icon: const Icon(Icons.refresh, size: 20),
            label: const Text(
              'Coba Lagi',
              style: TextStyle(fontSize: 16, fontWeight: FontWeight.w600),
            ),
            style: OutlinedButton.styleFrom(
              foregroundColor: isDark
                  ? core.AppColors.neutralWhite
                  : core.AppColors.neutralGray900,
              padding: const EdgeInsets.symmetric(vertical: 16),
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(8),
              ),
              side: BorderSide(
                color: isDark
                    ? core.AppColors.neutralGray700
                    : core.AppColors.neutralGray300,
              ),
            ),
          ),
        ),

        // Explicit reopen-payment action - only when a reusable, non-terminal
        // payment URL exists. Distinct from the status-only button above.
        if (state.canContinuePayment) ...[
          const SizedBox(height: 12),
          SizedBox(
            width: double.infinity,
            child: ElevatedButton.icon(
              onPressed: _handleContinuePayment,
              icon: const Icon(Icons.open_in_browser, size: 20),
              label: const Text(
                'Lanjutkan Pembayaran',
                style: TextStyle(fontSize: 16, fontWeight: FontWeight.w600),
              ),
              style: ElevatedButton.styleFrom(
                backgroundColor: core.AppColors.primaryRed,
                foregroundColor: Colors.white,
                padding: const EdgeInsets.symmetric(vertical: 16),
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(8),
                ),
              ),
            ),
          ),
        ],

        const SizedBox(height: 12),

        // Link to order detail
        TextButton(
          onPressed: _goToOrderDetail,
          child: Text(
            'Lihat Detail Pesanan',
            style: TextStyle(
              fontSize: 14,
              color: isDark
                  ? core.AppColors.neutralGray400
                  : core.AppColors.neutralGray600,
            ),
          ),
        ),
      ],
    );
  }

  /// Success state - backend confirmed payment successful
  /// **CV2:** Shows "Kembali ke Chat" button when returnToChat is set
  Widget _buildSuccessContent(PaymentResultState state, bool isDark) {
    final hasReturnToChat =
        widget.returnToChat != null && widget.returnToChat!.isNotEmpty;

    return Column(
      mainAxisAlignment: MainAxisAlignment.center,
      children: [
        // Success icon
        Container(
          width: 100,
          height: 100,
          decoration: BoxDecoration(
            color: core.AppColors.successGreen.withValues(alpha: 0.1),
            shape: BoxShape.circle,
          ),
          child: const Icon(
            Icons.check_circle,
            size: 64,
            color: core.AppColors.successGreen,
          ),
        ),
        const SizedBox(height: 32),

        // Title
        Text(
          'Pembayaran Berhasil',
          style: TextStyle(
            fontSize: 24,
            fontWeight: FontWeight.bold,
            color: isDark
                ? core.AppColors.neutralWhite
                : core.AppColors.neutralGray900,
          ),
        ),
        const SizedBox(height: 16),

        // Subtitle
        Text(
          'Pesanan Anda telah dibayar.\nOrder: ${widget.orderNumber ?? widget.orderId}',
          textAlign: TextAlign.center,
          style: TextStyle(
            fontSize: 16,
            color: isDark
                ? core.AppColors.neutralGray400
                : core.AppColors.neutralGray600,
          ),
        ),
        const SizedBox(height: 24),

        // PHASE 2 HARDENING: "Apa Selanjutnya?" section
        // Provides post-payment clarity to buyers
        _NextStepsSection(isDark: isDark),
        const SizedBox(height: 32),

        // Lihat Pesanan Button
        SizedBox(
          width: double.infinity,
          child: ElevatedButton(
            onPressed: _goToOrderDetail,
            style: ElevatedButton.styleFrom(
              backgroundColor: core.AppColors.primaryRed,
              foregroundColor: Colors.white,
              padding: const EdgeInsets.symmetric(vertical: 16),
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(8),
              ),
            ),
            child: const Text(
              'Lihat Pesanan',
              style: TextStyle(fontSize: 16, fontWeight: FontWeight.w600),
            ),
          ),
        ),
        const SizedBox(height: 12),

        // **CV2:** "Kembali ke Chat" button when returning from chat commerce
        if (hasReturnToChat)
          SizedBox(
            width: double.infinity,
            child: OutlinedButton.icon(
              onPressed: _goToChat,
              icon: const Icon(Icons.chat_bubble_outline, size: 20),
              label: const Text(
                'Kembali ke Chat',
                style: TextStyle(fontSize: 16, fontWeight: FontWeight.w600),
              ),
              style: OutlinedButton.styleFrom(
                foregroundColor: core.AppColors.primaryRed,
                padding: const EdgeInsets.symmetric(vertical: 16),
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(8),
                ),
                side: BorderSide(
                  color: core.AppColors.primaryRed.withValues(alpha: 0.5),
                ),
              ),
            ),
          ),
        SizedBox(
          width: double.infinity,
          child: OutlinedButton(
            onPressed: _goToHome,
            style: OutlinedButton.styleFrom(
              foregroundColor: isDark
                  ? core.AppColors.neutralWhite
                  : core.AppColors.neutralGray900,
              padding: const EdgeInsets.symmetric(vertical: 16),
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(8),
              ),
              side: BorderSide(
                color: isDark
                    ? core.AppColors.neutralGray700
                    : core.AppColors.neutralGray300,
              ),
            ),
            child: const Text(
              'Kembali ke Beranda',
              style: TextStyle(fontSize: 16, fontWeight: FontWeight.w600),
            ),
          ),
        ),
      ],
    );
  }

  /// Failed state - backend confirmed payment failed/expired/refunded
  /// PHASE 3 HARDENING: Added help CTAs for payment failure support
  Widget _buildFailedContent(PaymentResultState state, bool isDark) {
    final reason =
        state.errorMessage ??
        'Pembayaran tidak dapat diproses. Silakan coba lagi.';

    final authState = ref.watch(authControllerProvider);
    final userId = authState is AuthStateAuthenticated
        ? authState.user.id
        : null;
    final userName = authState is AuthStateAuthenticated
        ? authState.user.username
        : null;
    final userAvatar = authState is AuthStateAuthenticated
        ? authState.user.avatarUrl
        : null;

    return Column(
      mainAxisAlignment: MainAxisAlignment.center,
      children: [
        // Failed icon
        Container(
          width: 100,
          height: 100,
          decoration: BoxDecoration(
            color: core.AppColors.statusError.withValues(alpha: 0.1),
            shape: BoxShape.circle,
          ),
          child: const Icon(
            Icons.cancel,
            size: 64,
            color: core.AppColors.statusError,
          ),
        ),
        const SizedBox(height: 32),

        // Title
        Text(
          'Pembayaran Gagal',
          style: TextStyle(
            fontSize: 24,
            fontWeight: FontWeight.bold,
            color: isDark
                ? core.AppColors.neutralWhite
                : core.AppColors.neutralGray900,
          ),
        ),
        const SizedBox(height: 16),

        // Subtitle with reason
        Text(
          '$reason\nOrder: ${widget.orderNumber ?? widget.orderId}',
          textAlign: TextAlign.center,
          style: TextStyle(
            fontSize: 16,
            color: isDark
                ? core.AppColors.neutralGray400
                : core.AppColors.neutralGray600,
          ),
        ),
        const SizedBox(height: 24),

        // PHASE 3 HARDENING: Help section for payment failure
        Container(
          padding: const EdgeInsets.all(16),
          decoration: BoxDecoration(
            color: core.AppColors.warning.withValues(alpha: 0.1),
            borderRadius: BorderRadius.circular(12),
            border: Border.all(
              color: core.AppColors.warning.withValues(alpha: 0.3),
            ),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Icon(
                    Icons.help_outline,
                    color: core.AppColors.warning,
                    size: 18,
                  ),
                  const SizedBox(width: 8),
                  Text(
                    'Butuh bantuan pembayaran?',
                    style: TextStyle(
                      fontSize: 14,
                      fontWeight: FontWeight.w600,
                      color: isDark
                          ? core.AppColors.neutralWhite
                          : core.AppColors.neutralGray900,
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 8),
              Text(
                'Cek panduan pembayaran atau hubungi support untuk bantuan langsung.',
                style: TextStyle(
                  fontSize: 12,
                  color: core.AppColors.neutralGray600,
                ),
              ),
              const SizedBox(height: 12),
              Row(
                children: [
                  Expanded(
                    child: OutlinedButton.icon(
                      onPressed: () {
                        Navigator.of(context).push(
                          MaterialPageRoute(
                            builder: (context) => HelpCenterScreen(
                              userId: userId,
                              userName: userName,
                              userAvatar: userAvatar,
                            ),
                          ),
                        );
                      },
                      icon: const Icon(Icons.article_outlined, size: 16),
                      label: const Text('Panduan'),
                      style: OutlinedButton.styleFrom(
                        foregroundColor: core.AppColors.warning,
                        padding: const EdgeInsets.symmetric(vertical: 8),
                        textStyle: const TextStyle(fontSize: 12),
                        side: BorderSide(
                          color: core.AppColors.warning.withValues(alpha: 0.5),
                        ),
                      ),
                    ),
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: ElevatedButton.icon(
                      onPressed: userId != null
                          ? () {
                              showPreChatFormRefactored(
                                context,
                                userId: userId,
                                userName: userName ?? 'User',
                                userAvatar: userAvatar,
                                linkedOrderId: widget.orderId,
                              );
                            }
                          : null,
                      icon: const Icon(Icons.support_agent, size: 16),
                      label: const Text('Support'),
                      style: ElevatedButton.styleFrom(
                        backgroundColor: core.AppColors.warning,
                        foregroundColor: Colors.white,
                        padding: const EdgeInsets.symmetric(vertical: 8),
                        textStyle: const TextStyle(fontSize: 12),
                      ),
                    ),
                  ),
                ],
              ),
            ],
          ),
        ),
        const SizedBox(height: 24),

        // Buttons
        SizedBox(
          width: double.infinity,
          child: ElevatedButton(
            onPressed: _goToOrderDetail,
            style: ElevatedButton.styleFrom(
              backgroundColor: core.AppColors.primaryRed,
              foregroundColor: Colors.white,
              padding: const EdgeInsets.symmetric(vertical: 16),
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(8),
              ),
            ),
            child: const Text(
              'Lihat Detail Pesanan',
              style: TextStyle(fontSize: 16, fontWeight: FontWeight.w600),
            ),
          ),
        ),
        const SizedBox(height: 12),
        SizedBox(
          width: double.infinity,
          child: OutlinedButton(
            onPressed: _goToHome,
            style: OutlinedButton.styleFrom(
              foregroundColor: isDark
                  ? core.AppColors.neutralWhite
                  : core.AppColors.neutralGray900,
              padding: const EdgeInsets.symmetric(vertical: 16),
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(8),
              ),
              side: BorderSide(
                color: isDark
                    ? core.AppColors.neutralGray700
                    : core.AppColors.neutralGray300,
              ),
            ),
            child: const Text(
              'Kembali ke Beranda',
              style: TextStyle(fontSize: 16, fontWeight: FontWeight.w600),
            ),
          ),
        ),
      ],
    );
  }

  /// Timeout state - max polling attempts reached, status unknown
  Widget _buildTimeoutContent(PaymentResultState state, bool isDark) {
    return Column(
      mainAxisAlignment: MainAxisAlignment.center,
      children: [
        // Warning icon
        Container(
          width: 100,
          height: 100,
          decoration: BoxDecoration(
            color: core.AppColors.statusWarning.withValues(alpha: 0.1),
            shape: BoxShape.circle,
          ),
          child: const Icon(
            Icons.pending_outlined,
            size: 64,
            color: core.AppColors.statusWarning,
          ),
        ),
        const SizedBox(height: 32),

        // Title
        Text(
          'Status Pembayaran Belum Diketahui',
          style: TextStyle(
            fontSize: 24,
            fontWeight: FontWeight.bold,
            color: isDark
                ? core.AppColors.neutralWhite
                : core.AppColors.neutralGray900,
          ),
        ),
        const SizedBox(height: 16),

        // Subtitle
        Text(
          'Kami tidak dapat memverifikasi status pembayaran Anda setelah ${state.maxPollAttempts}x pengecekan.\nOrder: ${widget.orderNumber ?? widget.orderId}',
          textAlign: TextAlign.center,
          style: TextStyle(
            fontSize: 16,
            color: isDark
                ? core.AppColors.neutralGray400
                : core.AppColors.neutralGray600,
          ),
        ),
        const SizedBox(height: 8),

        // Info message
        Container(
          padding: const EdgeInsets.all(12),
          margin: const EdgeInsets.symmetric(vertical: 16),
          decoration: BoxDecoration(
            color: core.AppColors.neutralGray100,
            borderRadius: BorderRadius.circular(8),
          ),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Icon(
                Icons.info_outline,
                size: 18,
                color: isDark
                    ? core.AppColors.neutralGray400
                    : core.AppColors.neutralGray600,
              ),
              const SizedBox(width: 8),
              Expanded(
                child: Text(
                  'Jika sudah membayar, status pembayaran akan diperbarui dalam beberapa menit. Silakan cek halaman pesanan Anda.',
                  style: TextStyle(
                    fontSize: 13,
                    color: isDark
                        ? core.AppColors.neutralGray300
                        : core.AppColors.neutralGray700,
                  ),
                ),
              ),
            ],
          ),
        ),
        const SizedBox(height: 16),

        // Buttons
        SizedBox(
          width: double.infinity,
          child: ElevatedButton(
            onPressed: _handleStatusCheck,
            style: ElevatedButton.styleFrom(
              backgroundColor: core.AppColors.primaryRed,
              foregroundColor: Colors.white,
              padding: const EdgeInsets.symmetric(vertical: 16),
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(8),
              ),
            ),
            child: const Text(
              'Cek Status Lagi',
              style: TextStyle(fontSize: 16, fontWeight: FontWeight.w600),
            ),
          ),
        ),

        // Explicit reopen-payment action - only when a reusable, non-terminal
        // payment URL exists. Distinct from the status-only button above.
        if (state.canContinuePayment) ...[
          const SizedBox(height: 12),
          SizedBox(
            width: double.infinity,
            child: OutlinedButton.icon(
              onPressed: _handleContinuePayment,
              icon: const Icon(Icons.open_in_browser, size: 20),
              label: const Text(
                'Lanjutkan Pembayaran',
                style: TextStyle(fontSize: 16, fontWeight: FontWeight.w600),
              ),
              style: OutlinedButton.styleFrom(
                foregroundColor: core.AppColors.primaryRed,
                padding: const EdgeInsets.symmetric(vertical: 16),
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(8),
                ),
                side: BorderSide(
                  color: core.AppColors.primaryRed.withValues(alpha: 0.5),
                ),
              ),
            ),
          ),
        ],

        const SizedBox(height: 12),
        SizedBox(
          width: double.infinity,
          child: OutlinedButton(
            onPressed: _goToOrderDetail,
            style: OutlinedButton.styleFrom(
              foregroundColor: isDark
                  ? core.AppColors.neutralWhite
                  : core.AppColors.neutralGray900,
              padding: const EdgeInsets.symmetric(vertical: 16),
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(8),
              ),
              side: BorderSide(
                color: isDark
                    ? core.AppColors.neutralGray700
                    : core.AppColors.neutralGray300,
              ),
            ),
            child: const Text(
              'Lihat Detail Pesanan',
              style: TextStyle(fontSize: 16, fontWeight: FontWeight.w600),
            ),
          ),
        ),
        const SizedBox(height: 12),
        SizedBox(
          width: double.infinity,
          child: TextButton(
            onPressed: _goToHome,
            child: Text(
              'Kembali ke Beranda',
              style: TextStyle(
                fontSize: 16,
                color: isDark
                    ? core.AppColors.neutralGray400
                    : core.AppColors.neutralGray600,
              ),
            ),
          ),
        ),
      ],
    );
  }

  /// Network error state - unable to reach backend
  Widget _buildNetworkErrorContent(PaymentResultState state, bool isDark) {
    final errorMessage =
        state.errorMessage ??
        'Terjadi kesalahan koneksi. Silakan periksa koneksi internet Anda.';

    return Column(
      mainAxisAlignment: MainAxisAlignment.center,
      children: [
        // Error icon
        Container(
          width: 100,
          height: 100,
          decoration: BoxDecoration(
            color: core.AppColors.statusError.withValues(alpha: 0.1),
            shape: BoxShape.circle,
          ),
          child: const Icon(
            Icons.wifi_off,
            size: 64,
            color: core.AppColors.statusError,
          ),
        ),
        const SizedBox(height: 32),

        // Title
        Text(
          'Gagal Terhubung ke Server',
          style: TextStyle(
            fontSize: 24,
            fontWeight: FontWeight.bold,
            color: isDark
                ? core.AppColors.neutralWhite
                : core.AppColors.neutralGray900,
          ),
        ),
        const SizedBox(height: 16),

        // Subtitle
        Text(
          errorMessage,
          textAlign: TextAlign.center,
          style: TextStyle(
            fontSize: 16,
            color: isDark
                ? core.AppColors.neutralGray400
                : core.AppColors.neutralGray600,
          ),
        ),
        const SizedBox(height: 32),

        // Buttons
        SizedBox(
          width: double.infinity,
          child: ElevatedButton(
            onPressed: _handleStatusCheck,
            style: ElevatedButton.styleFrom(
              backgroundColor: core.AppColors.primaryRed,
              foregroundColor: Colors.white,
              padding: const EdgeInsets.symmetric(vertical: 16),
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(8),
              ),
            ),
            child: const Text(
              'Coba Lagi',
              style: TextStyle(fontSize: 16, fontWeight: FontWeight.w600),
            ),
          ),
        ),

        // Explicit reopen-payment action - only when a reusable, non-terminal
        // payment URL exists. Distinct from the status-only button above.
        if (state.canContinuePayment) ...[
          const SizedBox(height: 12),
          SizedBox(
            width: double.infinity,
            child: OutlinedButton.icon(
              onPressed: _handleContinuePayment,
              icon: const Icon(Icons.open_in_browser, size: 20),
              label: const Text(
                'Lanjutkan Pembayaran',
                style: TextStyle(fontSize: 16, fontWeight: FontWeight.w600),
              ),
              style: OutlinedButton.styleFrom(
                foregroundColor: core.AppColors.primaryRed,
                padding: const EdgeInsets.symmetric(vertical: 16),
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(8),
                ),
                side: BorderSide(
                  color: core.AppColors.primaryRed.withValues(alpha: 0.5),
                ),
              ),
            ),
          ),
        ],

        const SizedBox(height: 12),
        SizedBox(
          width: double.infinity,
          child: OutlinedButton(
            onPressed: _goToOrderDetail,
            style: OutlinedButton.styleFrom(
              foregroundColor: isDark
                  ? core.AppColors.neutralWhite
                  : core.AppColors.neutralGray900,
              padding: const EdgeInsets.symmetric(vertical: 16),
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(8),
              ),
              side: BorderSide(
                color: isDark
                    ? core.AppColors.neutralGray700
                    : core.AppColors.neutralGray300,
              ),
            ),
            child: const Text(
              'Lihat Detail Pesanan',
              style: TextStyle(fontSize: 16, fontWeight: FontWeight.w600),
            ),
          ),
        ),
        const SizedBox(height: 12),
        SizedBox(
          width: double.infinity,
          child: TextButton(
            onPressed: _goToHome,
            child: Text(
              'Kembali ke Beranda',
              style: TextStyle(
                fontSize: 16,
                color: isDark
                    ? core.AppColors.neutralGray400
                    : core.AppColors.neutralGray600,
              ),
            ),
          ),
        ),
      ],
    );
  }

  /// Get appropriate message based on elapsed time
  String _getElapsedTimeMessage(PaymentResultState state) =>
      _paymentResultGetElapsedTimeMessage(state);
}
