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
import 'package:labuda/core/src/theme/app_theme.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart' as core;
import 'package:labuda/domains/commerce/transaction/checkout/presentation/providers/payment_result_notifier.dart';
import 'package:labuda/domains/commerce/transaction/checkout/presentation/providers/payment_result_state.dart'
    show PaymentResultScreenStatus, PaymentResultState;
import 'package:labuda/domains/system/support/presentation/screens/help_center_screen.dart';
import 'package:labuda/domains/system/support/presentation/widgets/pre_chat_form_sheet.dart';
import 'package:labuda/domains/user/identity/authentication/authentication.dart';
// Payment URLs are presented exclusively inside Labuda's internal WebView.

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

  /// Reopen the original payment URL inside Labuda's internal WebView.
  /// External-browser payment navigation is obsolete and must not be reintroduced.
  Future<void> _openExistingPaymentUrl(String paymentUrl) async {
    if (paymentUrl.isEmpty || !mounted) return;
    final encodedUrl = Uri.encodeComponent(paymentUrl);
    final encodedOrderId = Uri.encodeComponent(widget.orderId);
    await context.push(
      '/payment-webview?url=$encodedUrl&orderId=$encodedOrderId',
    );
  }

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;

    // Watch the payment result state
    final state = ref.watch(paymentResultProvider);

    return Scaffold(
      // Page canvas — canonical lowest tone in both modes (checkout precedent).
      backgroundColor: colorScheme.surfaceContainerLowest,
      body: SafeArea(
        child: Center(
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(core.AppMetrics.p24),
            child: _buildContent(state),
          ),
        ),
      ),
    );
  }

  Widget _buildContent(PaymentResultState state) {
    switch (state.status) {
      case PaymentResultScreenStatus.checking:
        return _buildCheckingContent(state);

      case PaymentResultScreenStatus.success:
        return _buildSuccessContent(state);

      case PaymentResultScreenStatus.failed:
        return _buildFailedContent(state);

      case PaymentResultScreenStatus.timeout:
        return _buildTimeoutContent(state);

      case PaymentResultScreenStatus.networkError:
        return _buildNetworkErrorContent(state);
    }
  }

  /// Checking state - actively polling backend
  Widget _buildCheckingContent(PaymentResultState state) {
    final colorScheme = Theme.of(context).colorScheme;
    final elapsedMessage = _getElapsedTimeMessage(state);

    return Column(
      mainAxisAlignment: MainAxisAlignment.center,
      children: [
        // Animated spinner
        SizedBox(
          width: 80,
          height: 80,
          child: CircularProgressIndicator(
            strokeWidth: 4,
            valueColor: AlwaysStoppedAnimation<Color>(colorScheme.primary),
          ),
        ),
        const SizedBox(height: 32),

        // Title
        Text(
          'Menunggu Konfirmasi Pembayaran',
          style: TextStyle(
            fontSize: core.AppType.s24,
            fontWeight: FontWeight.bold,
            color: colorScheme.onSurface,
          ),
        ),
        const SizedBox(height: 16),

        // Subtitle
        Text(
          'Mohon tunggu, kami sedang mengecek status pembayaran Anda...\nOrder: ${widget.orderNumber ?? widget.orderId}',
          textAlign: TextAlign.center,
          style: TextStyle(
            fontSize: core.AppType.s16,
            color: colorScheme.onSurfaceVariant,
          ),
        ),
        const SizedBox(height: 8),

        // Poll counter - shows transparency to user
        Text(
          'Pengecekan ke ${state.pollAttempts + 1}/${state.maxPollAttempts}',
          style: TextStyle(
            fontSize: core.AppType.s14,                color: colorScheme.onSurfaceVariant,
          ),
        ),

        // Elapsed time-based message (shows after 15s)
        if (elapsedMessage.isNotEmpty) ...[
          const SizedBox(height: 16),
          Container(
            padding: const EdgeInsets.symmetric(horizontal: core.AppMetrics.p16, vertical: core.AppMetrics.p12),
            decoration: BoxDecoration(
              color: context.statusColors.warning.withValues(alpha: 0.1),
              borderRadius: BorderRadius.circular(core.AppShape.r8),
              border: Border.all(
                color: context.statusColors.warning.withValues(alpha: 0.3),
              ),
            ),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(
                  Icons.info_outline,
                  size: AppIconSize.action,
                  color: context.statusColors.warning,
                ),
                const SizedBox(width: 8),
                Flexible(
                  child: Text(
                    elapsedMessage,
                    textAlign: TextAlign.center,
                    style: TextStyle(
                      fontSize: core.AppType.s14,
                      color: colorScheme.onSurfaceVariant,
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
            icon: const Icon(Icons.refresh, size: AppIconSize.action),
            label: const Text(
              'Coba Lagi',
              style: TextStyle(fontSize: core.AppType.s16, fontWeight: FontWeight.w600),
            ),
            style: OutlinedButton.styleFrom(
              padding: const EdgeInsets.symmetric(vertical: core.AppMetrics.p16),
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
              icon: const Icon(Icons.open_in_browser, size: AppIconSize.action),
              label: const Text(
                'Lanjutkan Pembayaran',
                style: TextStyle(fontSize: core.AppType.s16, fontWeight: FontWeight.w600),
              ),
              style: ElevatedButton.styleFrom(
                padding: const EdgeInsets.symmetric(vertical: core.AppMetrics.p16),
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
              fontSize: core.AppType.s14,
              color: colorScheme.onSurfaceVariant,
            ),
          ),
        ),
      ],
    );
  }

  /// Success state - backend confirmed payment successful
  /// **CV2:** Shows "Kembali ke Chat" button when returnToChat is set
  Widget _buildSuccessContent(PaymentResultState state) {
    final colorScheme = Theme.of(context).colorScheme;
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
            color: context.statusColors.success.withValues(alpha: 0.1),
            shape: BoxShape.circle,
          ),
          child: Icon(
            Icons.check_circle,
            size: AppIconSize.display,
            color: context.statusColors.success,
          ),
        ),
        const SizedBox(height: 32),

        // Title
        Text(
          'Pembayaran Berhasil',
          style: TextStyle(
            fontSize: core.AppType.s24,
            fontWeight: FontWeight.bold,
            color: colorScheme.onSurface,
          ),
        ),
        const SizedBox(height: 16),

        // Subtitle
        Text(
          'Pesanan Anda telah dibayar.\nOrder: ${widget.orderNumber ?? widget.orderId}',
          textAlign: TextAlign.center,
          style: TextStyle(
            fontSize: core.AppType.s16,
            color: colorScheme.onSurfaceVariant,
          ),
        ),
        const SizedBox(height: 24),

        // PHASE 2 HARDENING: "Apa Selanjutnya?" section
        // Provides post-payment clarity to buyers
        _NextStepsSection(),
        const SizedBox(height: 32),

        // Lihat Pesanan Button
        SizedBox(
          width: double.infinity,
          child: ElevatedButton(
            onPressed: _goToOrderDetail,
            style: ElevatedButton.styleFrom(
              padding: const EdgeInsets.symmetric(vertical: core.AppMetrics.p16),
            ),
            child: const Text(
              'Lihat Pesanan',
              style: TextStyle(fontSize: core.AppType.s16, fontWeight: FontWeight.w600),
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
              icon: const Icon(Icons.chat_bubble_outline, size: AppIconSize.action),
              label: const Text(
                'Kembali ke Chat',
                style: TextStyle(fontSize: core.AppType.s16, fontWeight: FontWeight.w600),
              ),
              style: OutlinedButton.styleFrom(
                padding: const EdgeInsets.symmetric(vertical: core.AppMetrics.p16),
              ),
            ),
          ),
        SizedBox(
          width: double.infinity,
          child: OutlinedButton(
            onPressed: _goToHome,
            style: OutlinedButton.styleFrom(
              padding: const EdgeInsets.symmetric(vertical: core.AppMetrics.p16),
            ),
            child: const Text(
              'Kembali ke Beranda',
              style: TextStyle(fontSize: core.AppType.s16, fontWeight: FontWeight.w600),
            ),
          ),
        ),
      ],
    );
  }

  /// Failed state - backend confirmed payment failed/expired/refunded
  /// PHASE 3 HARDENING: Added help CTAs for payment failure support
  Widget _buildFailedContent(PaymentResultState state) {
    final colorScheme = Theme.of(context).colorScheme;
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
            color: context.statusColors.error.withValues(alpha: 0.1),
            shape: BoxShape.circle,
          ),
          child: Icon(
            Icons.cancel,
            size: AppIconSize.display,
            color: context.statusColors.error,
          ),
        ),
        const SizedBox(height: 32),

        // Title - comes from the failure authority (the notifier's switch).
        // Never hardcoded: a cancellation, refund, or dispute must not be
        // accused of "Pembayaran Gagal".
        Text(
          state.title ?? 'Pembayaran Tidak Berhasil',
          style: TextStyle(
            fontSize: core.AppType.s24,
            fontWeight: FontWeight.bold,
            color: colorScheme.onSurface,
          ),
        ),
        const SizedBox(height: 16),

        // Subtitle with reason
        Text(
          '$reason\nOrder: ${widget.orderNumber ?? widget.orderId}',
          textAlign: TextAlign.center,
          style: TextStyle(
            fontSize: core.AppType.s16,
            color: colorScheme.onSurfaceVariant,
          ),
        ),
        const SizedBox(height: 24),

        // PHASE 3 HARDENING: Help section for payment failure
        Container(
          padding: const EdgeInsets.all(core.AppMetrics.p16),
          decoration: BoxDecoration(
            color: context.statusColors.warning.withValues(alpha: 0.1),
            borderRadius: BorderRadius.circular(core.AppShape.r12),
            border: Border.all(
              color: context.statusColors.warning.withValues(alpha: 0.3),
            ),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Icon(
                    Icons.help_outline,
                    color: context.statusColors.warning,
                    size: AppIconSize.action,
                  ),
                  const SizedBox(width: 8),
                  Text(
                    'Butuh bantuan pembayaran?',
                    style: TextStyle(
                      fontSize: core.AppType.s14,
                      fontWeight: FontWeight.w600,
                      color: colorScheme.onSurface,
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 8),
              Text(
                'Cek panduan pembayaran atau hubungi support untuk bantuan langsung.',
                style: TextStyle(
                  fontSize: core.AppType.s12,
                  color: colorScheme.onSurfaceVariant,
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
                      icon: const Icon(Icons.article_outlined, size: AppIconSize.inlineGlyph),
                      label: const Text('Panduan'),
                      style: OutlinedButton.styleFrom(
                        foregroundColor: context.statusColors.warning,
                        padding: const EdgeInsets.symmetric(vertical: core.AppMetrics.p8),
                        textStyle: const TextStyle(fontSize: core.AppType.s12),
                        side: BorderSide(
                          color: context.statusColors.warning.withValues(alpha: 0.5),
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
                      icon: const Icon(Icons.support_agent, size: AppIconSize.inlineGlyph),
                      label: const Text('Support'),
              style: ElevatedButton.styleFrom(
                backgroundColor: context.statusColors.warning,
                padding: const EdgeInsets.symmetric(vertical: core.AppMetrics.p8),
                textStyle: const TextStyle(fontSize: core.AppType.s12),
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
              padding: const EdgeInsets.symmetric(vertical: core.AppMetrics.p16),
            ),
            child: const Text(
              'Lihat Detail Pesanan',
              style: TextStyle(fontSize: core.AppType.s16, fontWeight: FontWeight.w600),
            ),
          ),
        ),
        const SizedBox(height: 12),
        SizedBox(
          width: double.infinity,
          child: OutlinedButton(
            onPressed: _goToHome,
            style: OutlinedButton.styleFrom(
              padding: const EdgeInsets.symmetric(vertical: core.AppMetrics.p16),
            ),
            child: const Text(
              'Kembali ke Beranda',
              style: TextStyle(fontSize: core.AppType.s16, fontWeight: FontWeight.w600),
            ),
          ),
        ),
      ],
    );
  }

  /// Timeout state - max polling attempts reached, status unknown
  Widget _buildTimeoutContent(PaymentResultState state) {
    final colorScheme = Theme.of(context).colorScheme;

    return Column(
      mainAxisAlignment: MainAxisAlignment.center,
      children: [
        // Warning icon
        Container(
          width: 100,
          height: 100,
          decoration: BoxDecoration(
            color: context.statusColors.warning.withValues(alpha: 0.1),
            shape: BoxShape.circle,
          ),
          child: Icon(
            Icons.pending_outlined,
            size: AppIconSize.display,
            color: context.statusColors.warning,
          ),
        ),
        const SizedBox(height: 32),

        // Title
        Text(
          'Status Pembayaran Belum Diketahui',
          style: TextStyle(
            fontSize: core.AppType.s24,
            fontWeight: FontWeight.bold,
            color: colorScheme.onSurface,
          ),
        ),
        const SizedBox(height: 16),

        // Subtitle
        Text(
          'Kami tidak dapat memverifikasi status pembayaran Anda setelah ${state.maxPollAttempts}x pengecekan.\nOrder: ${widget.orderNumber ?? widget.orderId}',
          textAlign: TextAlign.center,
          style: TextStyle(
            fontSize: core.AppType.s16,
            color: colorScheme.onSurfaceVariant,
          ),
        ),
        const SizedBox(height: 8),

        // Info message
        Container(
          padding: const EdgeInsets.all(core.AppMetrics.p12),
          margin: const EdgeInsets.symmetric(vertical: core.AppMetrics.p16),
          decoration: BoxDecoration(
            color: colorScheme.surfaceContainer,
            borderRadius: BorderRadius.circular(core.AppShape.r8),
          ),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Icon(
                Icons.info_outline,
                size: AppIconSize.action,
                color: colorScheme.onSurfaceVariant,
              ),
              const SizedBox(width: 8),
              Expanded(
                child: Text(
                  'Jika sudah membayar, status pembayaran akan diperbarui dalam beberapa menit. Silakan cek halaman pesanan Anda.',
                  style: TextStyle(
                    fontSize: core.AppType.s14,
                    color: colorScheme.onSurfaceVariant,
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
              padding: const EdgeInsets.symmetric(vertical: core.AppMetrics.p16),
            ),
            child: const Text(
              'Cek Status Lagi',
              style: TextStyle(fontSize: core.AppType.s16, fontWeight: FontWeight.w600),
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
              icon: const Icon(Icons.open_in_browser, size: AppIconSize.action),
              label: const Text(
                'Lanjutkan Pembayaran',
                style: TextStyle(fontSize: core.AppType.s16, fontWeight: FontWeight.w600),
              ),
              style: OutlinedButton.styleFrom(
                padding: const EdgeInsets.symmetric(vertical: core.AppMetrics.p16),
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
              padding: const EdgeInsets.symmetric(vertical: core.AppMetrics.p16),
            ),
            child: const Text(
              'Lihat Detail Pesanan',
              style: TextStyle(fontSize: core.AppType.s16, fontWeight: FontWeight.w600),
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
                fontSize: core.AppType.s16,
                color: colorScheme.onSurfaceVariant,
              ),
            ),
          ),
        ),
      ],
    );
  }

  /// Network error state - unable to reach backend
  Widget _buildNetworkErrorContent(PaymentResultState state) {
    final colorScheme = Theme.of(context).colorScheme;
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
            color: context.statusColors.error.withValues(alpha: 0.1),
            shape: BoxShape.circle,
          ),
          child: Icon(
            Icons.wifi_off,
            size: AppIconSize.display,
            color: context.statusColors.error,
          ),
        ),
        const SizedBox(height: 32),

        // Title
        Text(
          'Gagal Terhubung ke Server',
          style: TextStyle(
            fontSize: core.AppType.s24,
            fontWeight: FontWeight.bold,
            color: colorScheme.onSurface,
          ),
        ),
        const SizedBox(height: 16),

        // Subtitle
        Text(
          errorMessage,
          textAlign: TextAlign.center,
          style: TextStyle(
            fontSize: core.AppType.s16,
            color: colorScheme.onSurfaceVariant,
          ),
        ),
        const SizedBox(height: 32),

        // Buttons
        SizedBox(
          width: double.infinity,
          child: ElevatedButton(
            onPressed: _handleStatusCheck,
            style: ElevatedButton.styleFrom(
              padding: const EdgeInsets.symmetric(vertical: core.AppMetrics.p16),
            ),
            child: const Text(
              'Coba Lagi',
              style: TextStyle(fontSize: core.AppType.s16, fontWeight: FontWeight.w600),
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
              icon: const Icon(Icons.open_in_browser, size: AppIconSize.action),
              label: const Text(
                'Lanjutkan Pembayaran',
                style: TextStyle(fontSize: core.AppType.s16, fontWeight: FontWeight.w600),
              ),
              style: OutlinedButton.styleFrom(
                padding: const EdgeInsets.symmetric(vertical: core.AppMetrics.p16),
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
              padding: const EdgeInsets.symmetric(vertical: core.AppMetrics.p16),
            ),
            child: const Text(
              'Lihat Detail Pesanan',
              style: TextStyle(fontSize: core.AppType.s16, fontWeight: FontWeight.w600),
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
                fontSize: core.AppType.s16,
                color: colorScheme.onSurfaceVariant,
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
