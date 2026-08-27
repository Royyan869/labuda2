import 'dart:async';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import '../providers/email_verification_controller.dart';
import 'email_verify_bottom_sheet.dart';

/// Persistent banner shown above main content when the authenticated user has
/// not yet verified their email.  Renders nothing for any other auth state
/// (unauthenticated, syncing, complete-profile), so no route allowlist is needed.
///
/// Lifecycle contract:
/// - NOT dismissable — only successful verification clears it.
/// - On app resume, silently calls the email-verification refresh path so that
///   a user who verified their email in a browser/email client sees the banner
///   disappear the moment they return to the app.
/// - CTA opens [EmailVerifyBottomSheet] (resend + "I've verified" check).
class EmailVerificationBanner extends ConsumerStatefulWidget {
  const EmailVerificationBanner({super.key});

  @override
  ConsumerState<EmailVerificationBanner> createState() =>
      _EmailVerificationBannerState();
}

class _EmailVerificationBannerState
    extends ConsumerState<EmailVerificationBanner>
    with WidgetsBindingObserver {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed) {
      _maybeRefreshOnResume();
    }
  }

  void _maybeRefreshOnResume() {
    final authState = ref.read(authControllerProvider);
    if (authState is AuthStateAuthenticated && !authState.emailVerified) {
      unawaited(
        ref
            .read(emailVerificationControllerProvider.notifier)
            .refreshEmailVerificationStatus(),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final authState = ref.watch(authControllerProvider);

    if (authState is! AuthStateAuthenticated || authState.emailVerified) {
      return const SizedBox.shrink();
    }

    final isDark = Theme.of(context).brightness == Brightness.dark;
    final backgroundColor = isDark
        ? AppColors.statusWarning.withValues(alpha: 0.18)
        : const Color(0xFFFFF7ED);
    final borderColor = AppColors.statusWarning.withValues(alpha: 0.4);
    final foregroundColor = isDark
        ? AppColors.neutralWhite
        : AppColors.neutralGray800;

    return Material(
      color: backgroundColor,
      child: SafeArea(
        bottom: false,
        child: Container(
          width: double.infinity,
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
          decoration: BoxDecoration(
            border: Border(bottom: BorderSide(color: borderColor)),
          ),
          child: Row(
            children: [
              const Icon(
                Icons.mark_email_unread_outlined,
                size: 18,
                color: AppColors.statusWarning,
              ),
              const SizedBox(width: 10),
              Expanded(
                child: Text(
                  'Verifikasi email kamu untuk bisa posting, chat, bid, checkout, dan transaksi.',
                  style: TextStyle(
                    fontSize: 13,
                    fontWeight: FontWeight.w600,
                    color: foregroundColor,
                  ),
                ),
              ),
              TextButton(
                onPressed: () => EmailVerifyBottomSheet.show(context),
                style: TextButton.styleFrom(
                  padding: const EdgeInsets.symmetric(
                    horizontal: 8,
                    vertical: 4,
                  ),
                  minimumSize: const Size(0, 0),
                  tapTargetSize: MaterialTapTargetSize.shrinkWrap,
                  foregroundColor: AppColors.statusWarning,
                ),
                child: const Text(
                  'Verifikasi →',
                  style: TextStyle(fontSize: 13, fontWeight: FontWeight.w700),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
