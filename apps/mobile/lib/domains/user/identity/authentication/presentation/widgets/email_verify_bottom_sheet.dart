import 'dart:async';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import '../providers/email_verification_controller.dart';
import '../providers/email_verification_state.dart';

/// Modal bottom sheet for email verification actions.
///
/// Triggered from [EmailVerificationBanner] and from [showBlockedActionGate]
/// (inline gate after a 403 EMAIL_VERIFICATION_REQUIRED response).
///
/// "Kirim Ulang" has a 60-second cooldown enforced client-side to reduce
/// Firebase send-email quota burn.
///
/// "Saya Sudah Verifikasi" calls the email-verification refresh path, which
/// reloads Firebase, then syncs the hydrated account only when verification
/// is actually confirmed.
class EmailVerifyBottomSheet {
  EmailVerifyBottomSheet._();

  static Future<void> show(BuildContext context) {
    return showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      useRootNavigator: true,
      backgroundColor: Colors.transparent,
      builder: (_) => const _EmailVerifyBottomSheetContent(),
    );
  }
}

class _EmailVerifyBottomSheetContent extends ConsumerStatefulWidget {
  const _EmailVerifyBottomSheetContent();

  @override
  ConsumerState<_EmailVerifyBottomSheetContent> createState() =>
      _EmailVerifyBottomSheetContentState();
}

class _EmailVerifyBottomSheetContentState
    extends ConsumerState<_EmailVerifyBottomSheetContent> {
  static const _cooldownSeconds = 60;

  bool _isChecking = false;
  int _cooldownRemaining = 0;
  Timer? _cooldownTimer;

  @override
  void dispose() {
    _cooldownTimer?.cancel();
    super.dispose();
  }

  bool get _canResend => _cooldownRemaining == 0;

  void _startCooldown() {
    setState(() => _cooldownRemaining = _cooldownSeconds);
    _cooldownTimer?.cancel();
    _cooldownTimer = Timer.periodic(const Duration(seconds: 1), (t) {
      if (!mounted) {
        t.cancel();
        return;
      }
      setState(() {
        _cooldownRemaining--;
        if (_cooldownRemaining <= 0) {
          _cooldownRemaining = 0;
          t.cancel();
        }
      });
    });
  }

  Future<void> _handleResend() async {
    if (!_canResend) return;
    try {
      final sent = await ref
          .read(emailVerificationControllerProvider.notifier)
          .sendVerificationEmail();
      if (!sent) {
        final state = ref.read(emailVerificationControllerProvider);
        if (state is EmailVerificationError) {
          _showError(state.message);
        } else {
          _showError('Gagal mengirim email verifikasi. Coba lagi.');
        }
        return;
      }
      if (!mounted) return;
      _startCooldown();
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text('Email verifikasi sudah dikirim ulang.'),
          backgroundColor: AppColors.statusSuccess,
        ),
      );
    } catch (_) {
      if (!mounted) return;
      _showError('Gagal mengirim email verifikasi. Coba lagi.');
    }
  }

  Future<void> _handleAlreadyVerified() async {
    if (_isChecking) return;
    setState(() => _isChecking = true);
    try {
      await ref
          .read(emailVerificationControllerProvider.notifier)
          .refreshEmailVerificationStatus();
      if (!mounted) return;
      final verificationState = ref.read(emailVerificationControllerProvider);
      if (verificationState is EmailVerificationVerified) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('Email berhasil diverifikasi!'),
            backgroundColor: AppColors.statusSuccess,
          ),
        );
        Navigator.of(context).pop();
      } else if (verificationState is EmailVerificationError) {
        _showError(verificationState.message);
      } else {
        _showError(
          'Email belum terverifikasi. Buka link verifikasi di email kamu dulu.',
        );
      }
    } catch (_) {
      _showError('Gagal memeriksa status. Coba lagi.');
    } finally {
      if (mounted) setState(() => _isChecking = false);
    }
  }

  void _showError(String message) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(message), backgroundColor: AppColors.statusError),
    );
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final email =
        ref.read(emailVerificationControllerProvider.notifier).currentEmail ??
        '-';
    final bgColor = isDark ? AppColors.darkGray800 : AppColors.neutralWhite;
    final fgColor = isDark ? AppColors.neutralWhite : AppColors.neutralGray800;
    final mutedColor = isDark
        ? AppColors.neutralGray300
        : AppColors.neutralGray600;

    final resendLabel = _canResend
        ? 'Kirim Ulang Email Verifikasi'
        : 'Kirim Ulang (${_cooldownRemaining}s)';

    return Padding(
      padding: MediaQuery.of(context).viewInsets,
      child: Container(
        decoration: BoxDecoration(
          color: bgColor,
          borderRadius: const BorderRadius.vertical(top: Radius.circular(20)),
        ),
        padding: const EdgeInsets.fromLTRB(20, 12, 20, 24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Center(
              child: Container(
                width: 44,
                height: 4,
                decoration: BoxDecoration(
                  color: AppColors.neutralGray300,
                  borderRadius: BorderRadius.circular(2),
                ),
              ),
            ),
            const SizedBox(height: 20),
            Text(
              'Verifikasi Email',
              style: TextStyle(
                fontSize: 18,
                fontWeight: FontWeight.bold,
                color: fgColor,
              ),
            ),
            const SizedBox(height: 8),
            Text(
              'Email verifikasi telah dikirim ke $email. Cek inbox dan folder spam kamu, lalu klik link verifikasi.',
              style: TextStyle(fontSize: 14, color: mutedColor, height: 1.4),
            ),
            const SizedBox(height: 20),
            ElevatedButton(
              onPressed: (_canResend && !_isChecking) ? _handleResend : null,
              style: ElevatedButton.styleFrom(
                backgroundColor: AppColors.primaryRed,
                foregroundColor: AppColors.light,
                disabledBackgroundColor: AppColors.primaryRed.withValues(
                  alpha: 0.5,
                ),
                padding: const EdgeInsets.symmetric(vertical: 14),
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(12),
                ),
              ),
              child: Text(
                resendLabel,
                style: const TextStyle(
                  fontSize: 15,
                  fontWeight: FontWeight.w600,
                ),
              ),
            ),
            const SizedBox(height: 10),
            OutlinedButton(
              onPressed: _isChecking ? null : _handleAlreadyVerified,
              style: OutlinedButton.styleFrom(
                foregroundColor: fgColor,
                side: BorderSide(
                  color: isDark
                      ? AppColors.darkGray600
                      : AppColors.neutralGray300,
                ),
                padding: const EdgeInsets.symmetric(vertical: 14),
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(12),
                ),
              ),
              child: _isChecking
                  ? const SizedBox(
                      width: 18,
                      height: 18,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : const Text(
                      'Saya Sudah Verifikasi',
                      style: TextStyle(
                        fontSize: 15,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
            ),
          ],
        ),
      ),
    );
  }
}
