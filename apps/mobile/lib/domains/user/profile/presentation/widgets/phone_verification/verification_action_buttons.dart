import 'package:flutter/material.dart';
import 'package:labuda/domains/user/profile/profile.dart';
import 'package:labuda/core/src/theme/app_theme.dart';

/// Action buttons for verification dialog
class VerificationActionButtons extends StatelessWidget {
  final PhoneVerificationState state;
  final VoidCallback onSendOTP;
  final VoidCallback onVerifyOTP;

  const VerificationActionButtons({
    super.key,
    required this.state,
    required this.onSendOTP,
    required this.onVerifyOTP,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Row(
      children: [
        Expanded(
          child: OutlinedButton(
            onPressed: state.isLoading || state.isVerifying
                ? null
                : () => Navigator.of(context).pop(false),
            style: OutlinedButton.styleFrom(
              padding: const EdgeInsets.symmetric(
                horizontal: AppMetrics.p12,
                vertical: AppMetrics.p12,
              ),
            ),
            child: Text(
              'Cancel',
              style: Theme.of(
                context,
              ).textTheme.labelLarge?.copyWith(fontWeight: FontWeight.w600),
              overflow: TextOverflow.ellipsis,
            ),
          ),
        ),
        const SizedBox(width: 8),
        Expanded(
          child: ElevatedButton(
            onPressed: state.isLoading || state.isVerifying
                ? null
                : state.codeSent
                ? onVerifyOTP
                : onSendOTP,
            style: ElevatedButton.styleFrom(
              padding: const EdgeInsets.symmetric(
                horizontal: AppMetrics.p12,
                vertical: AppMetrics.p12,
              ),
            ),
            child: state.isLoading || state.isVerifying
                ? SizedBox(
                    width: 16,
                    height: 16,
                    child: CircularProgressIndicator(
                      strokeWidth: 2,
                      valueColor: AlwaysStoppedAnimation<Color>(
                        scheme.onSurfaceVariant,
                      ),
                    ),
                  )
                : Text(
                    state.codeSent ? 'Verify' : 'Send OTP',
                    style: Theme.of(context).textTheme.labelLarge?.copyWith(
                      fontWeight: FontWeight.w600,
                    ),
                    overflow: TextOverflow.ellipsis,
                  ),
          ),
        ),
      ],
    );
  }
}
