import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';

/// Verify Email Screen — D2 HARD GATE surface (design scope v2).
///
/// Shown exclusively while the auth state machine is in
/// [AuthStatePendingEmailVerification]: the Firebase identity exists but its
/// email is not verified, so the backend exchange is forbidden (INV-8:
/// verify → exchange, single path). This is the onboarding verify step —
/// NOT a profile surface (the progressive-era profile resend path is dead:
/// an authenticated user is always verified under the hard gate).
///
/// Behaviors:
/// - Auto-polls Firebase `reload()` every 5s while visible so the flow
///   continues by itself once the user clicks the link (read-only check;
///   the exchange runs exactly once, driven by the auth state machine
///   via [AuthController.checkPendingEmailVerification]).
/// - Resend with a 60s display cooldown.
/// - Escape hatch: "Ganti Akun" → clean signOut back to /welcome.
class VerifyEmailScreen extends ConsumerStatefulWidget {
  const VerifyEmailScreen({super.key});

  @override
  ConsumerState<VerifyEmailScreen> createState() => _VerifyEmailScreenState();
}

class _VerifyEmailScreenState extends ConsumerState<VerifyEmailScreen> {
  Timer? _pollTimer;
  Timer? _cooldownTimer;
  int _cooldownSeconds = 0;
  bool _isChecking = false;
  bool _isResending = false;

  static const _pollInterval = Duration(seconds: 5);

  @override
  void initState() {
    super.initState();
    _startPolling();
  }

  @override
  void dispose() {
    _pollTimer?.cancel();
    _cooldownTimer?.cancel();
    super.dispose();
  }

  void _startPolling() {
    _pollTimer?.cancel();
    _pollTimer = Timer.periodic(_pollInterval, (_) => _checkVerification());
  }

  Future<void> _checkVerification() async {
    if (_isChecking || !mounted) return;
    setState(() => _isChecking = true);
    try {
      // Single canonical bridge back into the exchange (INV-8). The
      // controller reloads the Firebase user and either parks here again
      // (still unverified) or runs the one exchange (verified).
      await ref
          .read(authControllerProvider.notifier)
          .checkPendingEmailVerification();
    } finally {
      if (mounted) setState(() => _isChecking = false);
    }
  }

  Future<void> _resendVerificationEmail() async {
    if (_isResending || _cooldownSeconds > 0) return;
    setState(() => _isResending = true);
    final ok = await ref
        .read(authControllerProvider.notifier)
        .resendVerificationEmail();
    if (!mounted) return;
    setState(() {
      _isResending = false;
      if (ok) _cooldownSeconds = 60;
    });
    if (ok) {
      _startCooldownCountdown();
      AppSnackBar.showSuccess(context, 'Email verifikasi telah dikirim ulang');
    } else {
      AppSnackBar.showError(
        context,
        'Gagal mengirim email verifikasi. Coba lagi beberapa saat.',
      );
    }
  }

  void _startCooldownCountdown() {
    _cooldownTimer?.cancel();
    _cooldownTimer = Timer.periodic(const Duration(seconds: 1), (timer) {
      if (!mounted) {
        timer.cancel();
        return;
      }
      setState(() {
        if (_cooldownSeconds > 0) {
          _cooldownSeconds--;
        } else {
          timer.cancel();
        }
      });
    });
  }

  void _signOut() {
    ref.read(authControllerProvider.notifier).signOut();
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final authState = ref.watch(authControllerProvider);

    String? email;
    String? pendingUsername;
    if (authState is AuthStatePendingEmailVerification) {
      email = authState.email;
      pendingUsername = authState.username;
    }

    return Scaffold(
      body: Container(
        decoration: BoxDecoration(
          gradient: LinearGradient(
            begin: Alignment.topCenter,
            end: Alignment.bottomCenter,
            colors: [scheme.surfaceContainerLow, scheme.surface],
          ),
        ),
        child: SafeArea(
          child: Center(
            child: SingleChildScrollView(
              padding: const EdgeInsets.all(AppMetrics.p32),
              child: Column(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  Container(
                    padding: const EdgeInsets.all(AppMetrics.p24),
                    decoration: BoxDecoration(
                      color: context.statusColors.warning.withValues(alpha: 0.1),
                      shape: BoxShape.circle,
                    ),
                    child: Icon(
                      Icons.mark_email_unread_outlined,
                      size: 64,
                      color: context.statusColors.warning,
                    ),
                  ),
                  const SizedBox(height: 32),
                  Text(
                    'Verifikasi Email',
                    style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                      fontWeight: FontWeight.bold,
                      color: scheme.onSurface,
                    ),
                    textAlign: TextAlign.center,
                  ),
                  const SizedBox(height: 16),
                  Text(
                    pendingUsername != null
                        ? 'Hampir selesai, $pendingUsername! Kami mengirim tautan '
                            'verifikasi ke email kamu. Buka tautan itu, lalu '
                            'kembali ke sini — pendaftaran lanjut otomatis.'
                        : 'Kami mengirim tautan verifikasi ke email kamu. '
                            'Buka tautan itu, lalu kembali ke sini untuk '
                            'melanjutkan masuk.',
                    style: Theme.of(context).textTheme.bodyLarge?.copyWith(
                      color: scheme.onSurfaceVariant,
                    ),
                    textAlign: TextAlign.center,
                  ),
                  const SizedBox(height: 24),
                  if (email != null && email.isNotEmpty)
                    Container(
                      padding: const EdgeInsets.all(AppMetrics.p16),
                      decoration: BoxDecoration(
                        color: scheme.surfaceContainerLow,
                        borderRadius: BorderRadius.circular(AppShape.r12),
                        border: Border.all(
                          color: scheme.outlineVariant,
                        ),
                      ),
                      child: Row(
                        children: [
                          Icon(
                            Icons.email_outlined,
                            size: 20,
                            color: scheme.onSurfaceVariant,
                          ),
                          const SizedBox(width: 12),
                          Expanded(
                            child: Text(
                              email,
                              style: TextStyle(
                                fontSize: AppType.s14,
                                fontWeight: FontWeight.w600,
                                color: scheme.onSurface,
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
                      onPressed: _isChecking ? null : _checkVerification,
                      style: ElevatedButton.styleFrom(
                        disabledBackgroundColor:
                            scheme.surfaceContainerHighest,
                        padding: const EdgeInsets.symmetric(vertical: AppMetrics.p14),
                      ),
                      child: _isChecking
                          ? SizedBox(
                              width: 20,
                              height: 20,
                              child: CircularProgressIndicator(
                                strokeWidth: 2,
                                color: scheme.onPrimary,
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
                          (_isResending || _cooldownSeconds > 0)
                              ? null
                              : _resendVerificationEmail,
                      style: OutlinedButton.styleFrom(
                        padding: const EdgeInsets.symmetric(vertical: AppMetrics.p14),
                      ),
                      child: Text(
                        _cooldownSeconds > 0
                            ? 'Kirim Ulang dalam 00:${_cooldownSeconds.toString().padLeft(2, '0')}'
                            : 'Kirim Ulang Email',
                        style: TextStyle(
                          color: scheme.onSurface,
                        ),
                      ),
                    ),
                  ),
                  const SizedBox(height: 24),
                  SizedBox(
                    width: double.infinity,
                    child: TextButton(
                      onPressed: _isChecking ? null : _signOut,
                      style: TextButton.styleFrom(
                        padding: const EdgeInsets.symmetric(vertical: AppMetrics.p14),
                      ),
                      child: Text(
                        'Ganti Akun',
                        style: TextStyle(
                          fontSize: AppType.s16,
                          color: scheme.onSurfaceVariant,
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
