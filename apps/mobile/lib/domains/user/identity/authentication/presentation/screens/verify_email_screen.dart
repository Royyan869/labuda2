import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';
import '../providers/auth_controller.dart';
import '../providers/email_verification_controller.dart';
import '../providers/email_verification_state.dart';

/// Verify Email Screen — blocking onboarding gate for unverified email/password users.
///
/// User must verify their email before entering the application.
/// Google/Apple users skip this screen (email verified by provider).
///
/// Features:
/// - Shows authenticated email address
/// - Resend verification email with 60s cooldown
/// - "I've verified" refresh action — reloads Firebase user, rehydrates backend
/// - Sign out / change account action
/// - Loading and error states
///
/// Cooldown authority: [EmailVerificationController] owns all business logic
/// (persistence, eligibility, send). This screen only reads state and runs a
/// display-only [Timer] to refresh the countdown.
class VerifyEmailScreen extends ConsumerStatefulWidget {
  const VerifyEmailScreen({super.key});

  @override
  ConsumerState<VerifyEmailScreen> createState() => _VerifyEmailScreenState();
}

class _VerifyEmailScreenState extends ConsumerState<VerifyEmailScreen>
    with WidgetsBindingObserver {
  static const _cooldownSeconds = 60;
  Timer? _displayTimer;
  /// Loading sentinel: -1 means the persisted cooldown has not been read yet.
  /// The first frame must not show an enabled resend button while the async
  /// read is still in flight — that would let the user tap "Kirim Ulang Email"
  /// before the cooldown is known.
  int _displayCooldownSeconds = -1;
  bool _isRefreshing = false;
  bool _isResending = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _seedInitialCooldown();
  }

  /// Seed the display countdown from the delivery status carried in
  /// [AuthStateRequiresEmailVerification] or from the persisted cooldown.
  ///
  /// - [VerificationDeliverySent]: the exact `sentAt` is available
  ///   synchronously — compute remaining and start the timer. First frame
  ///   shows the countdown, no placeholder.
  /// - [VerificationDeliveryFailed]: no cooldown, button enabled immediately.
  /// - [VerificationDeliveryUnknown]: async read from persisted storage.
  ///   Brief disabled state acceptable on first frame.
  void _seedInitialCooldown() {
    final authState = ref.read(authControllerProvider);
    if (authState is! AuthStateRequiresEmailVerification) return;

    final delivery = authState.deliveryStatus;
    if (delivery is VerificationDeliverySent) {
      // Synchronous: sentAt is carried in the state, no async read needed.
      // The first frame shows the countdown with the exact remaining value.
      final remaining = _computeRemainingFromSentAt(delivery.sentAt);
      if (remaining > 0) {
        _displayCooldownSeconds = remaining;
        _startDisplayTimer();
      } else {
        _displayCooldownSeconds = 0;
      }
      return;
    }

    if (delivery is VerificationDeliveryFailed) {
      // No cooldown — send failed. Button enabled on first frame.
      _displayCooldownSeconds = 0;
      return;
    }

    // VerificationDeliveryUnknown: async read from persisted storage.
    _seedCooldownFromController();
  }

  /// Compute remaining cooldown seconds from [sentAt] using the current
  /// wall clock. Does not touch persisted storage — the sentAt timestamp
  /// was carried in [VerificationDeliverySent] and recordSent was already
  /// called before portal publication.
  int _computeRemainingFromSentAt(DateTime sentAt) {
    const cooldown = Duration(seconds: _cooldownSeconds);
    final elapsed = DateTime.now().difference(sentAt).inSeconds;
    final remaining = cooldown.inSeconds - elapsed;
    return remaining > 0 ? remaining : 0;
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    _displayTimer?.cancel();
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState lifecycleState) {
    if (lifecycleState == AppLifecycleState.resumed) {
      // Trigger the canonical verification coordinator. The coordinator
      // has its own single-flight guard — duplicate calls are safe.
      ref
          .read(emailVerificationControllerProvider.notifier)
          .refreshEmailVerificationStatus();
      // Re-seed the cooldown on resume — the user may have been in the
      // email client for a while and the display timer may have drifted.
      _seedCooldownFromController();
    }
  }

  /// Read the persisted cooldown from the canonical controller and seed the
  /// display countdown. The controller uses persisted timestamp + injected
  /// clock — this survives app restart, rebuild, and background/foreground.
  Future<void> _seedCooldownFromController() async {
    final remaining = await ref
        .read(emailVerificationControllerProvider.notifier)
        .cooldownRemainingSeconds();
    if (!mounted) return;
    setState(() {
      if (remaining > 0) {
        _displayCooldownSeconds = remaining;
        _startDisplayTimer();
      } else {
        // Cooldown has expired or was never set — enable the button.
        _displayCooldownSeconds = 0;
      }
    });
  }

  void _startDisplayTimer() {
    _displayTimer?.cancel();
    _displayTimer = Timer.periodic(const Duration(seconds: 1), (timer) {
      if (!mounted) {
        timer.cancel();
        return;
      }
      setState(() {
        if (_displayCooldownSeconds > 0) {
          _displayCooldownSeconds--;
        } else {
          timer.cancel();
        }
      });
    });
  }

  bool get _isResendDisabled =>
      _displayCooldownSeconds != 0 || _isResending;

  Future<void> _resendVerificationEmail() async {
    if (_isResendDisabled) return;
    setState(() => _isResending = true);

    final result = await ref
        .read(emailVerificationControllerProvider.notifier)
        .resendVerificationEmail();

    if (!mounted) return;
    setState(() => _isResending = false);

    switch (result) {
      case ResendSuccess():
        setState(() => _displayCooldownSeconds = _cooldownSeconds);
        _startDisplayTimer();
        AppSnackBar.showSuccess(context, 'Email verifikasi telah dikirim ulang');
      case ResendOnCooldown(:final remainingSeconds):
        setState(() => _displayCooldownSeconds = remainingSeconds);
        _startDisplayTimer();
      case ResendError(:final message):
        AppSnackBar.showError(context, message);
      case ResendAlreadyVerified():
        // AuthController publishes Authenticated, router redirects.
        break;
      case ResendAlreadyInProgress():
        // Silently ignore — single-flight guard.
        break;
    }
  }

  Future<void> _checkVerificationStatus() async {
    if (_isRefreshing) return;
    setState(() => _isRefreshing = true);

    try {
      // Delegate to the canonical verification orchestrator.
      // It owns reload, token refresh, and backend sync.
      await ref
          .read(emailVerificationControllerProvider.notifier)
          .refreshEmailVerificationStatus();

      if (mounted) {
        setState(() => _isRefreshing = false);
        final controllerState =
            ref.read(emailVerificationControllerProvider);
        if (controllerState is EmailVerificationUnverified) {
          AppSnackBar.showError(
            context,
            'Email belum terverifikasi. Silakan cek inbox atau spam.',
          );
        } else if (controllerState is EmailVerificationError) {
          AppSnackBar.showError(context, controllerState.message);
        }
        // If verified, the AuthController publishes Authenticated
        // and the router redirects to Home automatically.
      }
    } catch (e) {
      if (mounted) {
        setState(() => _isRefreshing = false);
        AppSnackBar.showError(context, 'Gagal memeriksa status verifikasi');
      }
    }
  }

  void _signOut() {
    ref.read(authControllerProvider.notifier).signOut();
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final authState = ref.watch(authControllerProvider);
    String? email;
    VerificationDeliveryStatus deliveryStatus =
        const VerificationDeliveryUnknown();

    if (authState is AuthStateRequiresEmailVerification) {
      email = authState.email;
      deliveryStatus = authState.deliveryStatus;
    }

    final isFailed = deliveryStatus is VerificationDeliveryFailed;

    return Scaffold(
      body: Container(
        decoration: BoxDecoration(
          gradient: isDark
              ? LinearGradient(
                  begin: Alignment.topCenter,
                  end: Alignment.bottomCenter,
                  colors: [AppColors.darkGray900, AppColors.darkGray800],
                )
              : const LinearGradient(
                  begin: Alignment.topCenter,
                  end: Alignment.bottomCenter,
                  colors: [AppColors.neutralGray50, AppColors.neutralWhite],
                ),
        ),
        child: SafeArea(
          child: Center(
            child: SingleChildScrollView(
              padding: const EdgeInsets.all(32),
              child: Column(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  // Email icon
                  Container(
                    padding: const EdgeInsets.all(24),
                    decoration: BoxDecoration(
                      color: (isFailed
                              ? AppColors.statusError
                              : AppColors.statusWarning)
                          .withValues(alpha: 0.1),
                      shape: BoxShape.circle,
                    ),
                    child: Icon(
                      isFailed
                          ? Icons.error_outline
                          : Icons.mark_email_unread_outlined,
                      size: 64,
                      color: isFailed
                          ? AppColors.statusError
                          : AppColors.statusWarning,
                    ),
                  ),
                  const SizedBox(height: 32),

                  // Title
                  Text(
                    'Verifikasi Email',
                    style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                      fontWeight: FontWeight.bold,
                      color: isDark
                          ? AppColors.neutralWhite
                          : AppColors.darkGray800,
                    ),
                    textAlign: TextAlign.center,
                  ),
                  const SizedBox(height: 16),

                  // Subtitle — varies by delivery status
                  Text(
                    switch (deliveryStatus) {
                      VerificationDeliveryFailed(:final message) => message,
                      VerificationDeliverySent() =>
                        'Kami telah mengirim email verifikasi ke alamat email '
                            'kamu. Silakan klik tautan di email tersebut untuk '
                            'melanjutkan.',
                      VerificationDeliveryUnknown() =>
                        'Akun kamu belum terverifikasi. '
                            'Kirim ulang email verifikasi atau periksa inbox '
                            'kamu untuk tautan verifikasi.',
                    },
                    style: Theme.of(context).textTheme.bodyLarge?.copyWith(
                      color: isDark
                          ? AppColors.neutralGray300
                          : AppColors.neutralGray600,
                    ),
                    textAlign: TextAlign.center,
                  ),
                  const SizedBox(height: 24),

                  // Email display
                  if (email != null && email.isNotEmpty)
                    Container(
                      padding: const EdgeInsets.all(16),
                      decoration: BoxDecoration(
                        color: isDark
                            ? AppColors.darkGray700.withValues(alpha: 0.5)
                            : AppColors.neutralGray50,
                        borderRadius: BorderRadius.circular(12),
                        border: Border.all(
                          color: isDark
                              ? AppColors.darkGray600
                              : AppColors.neutralGray200,
                        ),
                      ),
                      child: Row(
                        children: [
                          Icon(
                            Icons.email_outlined,
                            size: 20,
                            color: isDark
                                ? AppColors.neutralGray400
                                : AppColors.neutralGray600,
                          ),
                          const SizedBox(width: 12),
                          Expanded(
                            child: Text(
                              email,
                              style: TextStyle(
                                fontSize: 14,
                                fontWeight: FontWeight.w600,
                                color: isDark
                                    ? AppColors.neutralGray200
                                    : AppColors.neutralGray800,
                              ),
                            ),
                          ),
                        ],
                      ),
                    ),
                  const SizedBox(height: 32),

                  // Refresh / "I've verified" button
                  SizedBox(
                    width: double.infinity,
                    child: ElevatedButton(
                      onPressed: _isRefreshing ? null : _checkVerificationStatus,
                      style: ElevatedButton.styleFrom(
                        backgroundColor: AppColors.primaryRed,
                        foregroundColor: AppColors.light,
                        disabledBackgroundColor: isDark
                            ? AppColors.darkGray600
                            : AppColors.neutralGray300,
                        padding: const EdgeInsets.symmetric(vertical: 14),
                        shape: RoundedRectangleBorder(
                          borderRadius: BorderRadius.circular(12),
                        ),
                      ),
                      child: _isRefreshing
                          ? const SizedBox(
                              width: 20,
                              height: 20,
                              child: CircularProgressIndicator(
                                strokeWidth: 2,
                                color: AppColors.light,
                              ),
                            )
                          : const Text('Saya Sudah Verifikasi'),
                    ),
                  ),
                  const SizedBox(height: 12),

                  // Resend button
                  SizedBox(
                    width: double.infinity,
                    child: OutlinedButton(
                      onPressed:
                          _isResendDisabled ? null : _resendVerificationEmail,
                      style: OutlinedButton.styleFrom(
                        padding: const EdgeInsets.symmetric(vertical: 14),
                        shape: RoundedRectangleBorder(
                          borderRadius: BorderRadius.circular(12),
                        ),
                        side: BorderSide(
                          color: isDark
                              ? AppColors.neutralGray600
                              : AppColors.neutralGray300,
                        ),
                      ),
                      child: Text(
                        _displayCooldownSeconds < 0
                            ? 'Memuat...'
                            : _displayCooldownSeconds > 0
                            ? 'Kirim Ulang dalam 00:${_displayCooldownSeconds.toString().padLeft(2, '0')}'
                            : 'Kirim Ulang Email',
                        style: TextStyle(
                          color: isDark
                              ? AppColors.neutralGray300
                              : AppColors.neutralGray700,
                        ),
                      ),
                    ),
                  ),
                  const SizedBox(height: 24),

                  // Sign out button
                  SizedBox(
                    width: double.infinity,
                    child: TextButton(
                      onPressed: _isRefreshing ? null : _signOut,
                      style: TextButton.styleFrom(
                        padding: const EdgeInsets.symmetric(vertical: 14),
                      ),
                      child: Text(
                        'Ganti Akun',
                        style: TextStyle(
                          fontSize: 16,
                          color: isDark
                              ? AppColors.neutralGray400
                              : AppColors.neutralGray500,
                        ),
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}
