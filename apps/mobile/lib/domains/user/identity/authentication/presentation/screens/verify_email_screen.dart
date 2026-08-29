import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';
import '../providers/email_verification_controller.dart';
import '../providers/email_verification_state.dart';

/// Verify Email Screen — minimal compile-correct placeholder.
///
/// The legacy verification portal (AuthStateRequiresEmailVerification +
/// VerificationDelivery*) was removed in the auth convergence — the current
/// canonical email-verified signal is [AuthStateAuthenticated.emailVerified]
/// and [EmailVerificationState] (verified/unverified). This screen is no
/// longer routed (see AuthModule — no verify-email route), but it must still
/// type-check for the analyzer gate. The UI below preserves the original
/// resend/refresh/sign-out affordances using the current
/// [EmailVerificationController] API (sendVerificationEmail / refresh).
class VerifyEmailScreen extends ConsumerStatefulWidget {
  const VerifyEmailScreen({super.key});

  @override
  ConsumerState<VerifyEmailScreen> createState() => _VerifyEmailScreenState();
}

class _VerifyEmailScreenState extends ConsumerState<VerifyEmailScreen>
    with WidgetsBindingObserver {
  Timer? _displayTimer;
  int _displayCooldownSeconds = 0;
  bool _isRefreshing = false;
  bool _isResending = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
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
      ref
          .read(emailVerificationControllerProvider.notifier)
          .refreshEmailVerificationStatus();
    }
  }

  void _startDisplayTimer(int seconds) {
    _displayCooldownSeconds = seconds;
    _displayTimer?.cancel();
    if (seconds <= 0) return;
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

  Future<void> _resendVerificationEmail() async {
    if (_isResending || _displayCooldownSeconds > 0) return;
    setState(() => _isResending = true);
    final ok = await ref
        .read(emailVerificationControllerProvider.notifier)
        .sendVerificationEmail();
    if (!mounted) return;
    setState(() => _isResending = false);
    if (ok) {
      setState(() => _displayCooldownSeconds = 60);
      _startDisplayTimer(60);
      AppSnackBar.showSuccess(context, 'Email verifikasi telah dikirim ulang');
    } else {
      final state = ref.read(emailVerificationControllerProvider);
      final msg = state is EmailVerificationError ? state.message : 'Gagal mengirim email verifikasi';
      AppSnackBar.showError(context, msg);
    }
  }

  Future<void> _checkVerificationStatus() async {
    if (_isRefreshing) return;
    setState(() => _isRefreshing = true);
    try {
      await ref
          .read(emailVerificationControllerProvider.notifier)
          .refreshEmailVerificationStatus();
      if (mounted) {
        setState(() => _isRefreshing = false);
        final controllerState = ref.read(emailVerificationControllerProvider);
        if (controllerState is EmailVerificationUnverified) {
          AppSnackBar.showError(
            context,
            'Email belum terverifikasi. Silakan cek inbox atau spam.',
          );
        } else if (controllerState is EmailVerificationError) {
          AppSnackBar.showError(context, controllerState.message);
        }
      }
    } catch (_) {
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
    if (authState is AuthStateAuthenticated) {
      email = authState.user.email;
    }

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
                  Container(
                    padding: const EdgeInsets.all(24),
                    decoration: BoxDecoration(
                      color: AppColors.statusWarning.withValues(alpha: 0.1),
                      shape: BoxShape.circle,
                    ),
                    child: const Icon(
                      Icons.mark_email_unread_outlined,
                      size: 64,
                      color: AppColors.statusWarning,
                    ),
                  ),
                  const SizedBox(height: 32),
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
                  Text(
                    'Akun kamu belum terverifikasi. '
                    'Kirim ulang email verifikasi atau periksa inbox kamu untuk tautan verifikasi.',
                    style: Theme.of(context).textTheme.bodyLarge?.copyWith(
                      color: isDark
                          ? AppColors.neutralGray300
                          : AppColors.neutralGray600,
                    ),
                    textAlign: TextAlign.center,
                  ),
                  const SizedBox(height: 24),
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
                  SizedBox(
                    width: double.infinity,
                    child: OutlinedButton(
                      onPressed:
                          (_isResending || _displayCooldownSeconds > 0)
                              ? null
                              : _resendVerificationEmail,
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
                        _displayCooldownSeconds > 0
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
