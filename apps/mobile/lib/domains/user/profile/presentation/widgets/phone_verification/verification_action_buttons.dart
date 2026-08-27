import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/profile/profile.dart';

/// Action buttons for verification dialog
class VerificationActionButtons extends StatelessWidget {
  final bool isDark;
  final PhoneVerificationState state;
  final VoidCallback onSendOTP;
  final VoidCallback onVerifyOTP;

  const VerificationActionButtons({
    super.key,
    required this.isDark,
    required this.state,
    required this.onSendOTP,
    required this.onVerifyOTP,
  });

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Expanded(
          child: OutlinedButton(
            onPressed: state.isLoading || state.isVerifying
                ? null
                : () => Navigator.of(context).pop(false),
            style: OutlinedButton.styleFrom(
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 12),
              side: BorderSide(
                color: isDark
                    ? AppColors.neutralGray600
                    : AppColors.neutralGray400,
              ),
            ),
            child: const Text(
              'Cancel',
              style: TextStyle(fontSize: 14, fontWeight: FontWeight.w600),
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
              backgroundColor: AppColors.primaryRed,
              foregroundColor: Colors.white,
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 12),
              disabledBackgroundColor: AppColors.primaryRed.withValues(
                alpha: 0.5,
              ),
            ),
            child: state.isLoading || state.isVerifying
                ? const SizedBox(
                    width: 16,
                    height: 16,
                    child: CircularProgressIndicator(
                      strokeWidth: 2,
                      valueColor: AlwaysStoppedAnimation<Color>(Colors.white),
                    ),
                  )
                : Text(
                    state.codeSent ? 'Verify' : 'Send OTP',
                    style: const TextStyle(
                      fontSize: 14,
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
