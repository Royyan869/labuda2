import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/domains/user/profile/profile.dart'
    show phoneVerificationProvider, phoneVerificationServiceProvider;
import 'package:labuda/domains/user/profile/presentation/widgets/phone_verification/verification_header.dart';
import 'package:labuda/domains/user/profile/presentation/widgets/phone_verification/phone_display.dart';
import 'package:labuda/domains/user/profile/presentation/widgets/phone_verification/otp_loading_state.dart';
import 'package:labuda/domains/user/profile/presentation/widgets/phone_verification/otp_input_field.dart';
import 'package:labuda/domains/user/profile/presentation/widgets/phone_verification/verification_error_message.dart';
import 'package:labuda/domains/user/profile/presentation/widgets/phone_verification/verification_action_buttons.dart';

/// Dialog untuk verifikasi nomor telepon dengan OTP
class PhoneVerificationDialog extends ConsumerStatefulWidget {
  final String phoneNumber;
  final Function() onVerificationSuccess;
  final Function(String error)? onVerificationFailed;

  const PhoneVerificationDialog({
    super.key,
    required this.phoneNumber,
    required this.onVerificationSuccess,
    this.onVerificationFailed,
  });

  /// Show phone verification dialog
  static Future<bool> show({
    required BuildContext context,
    required String phoneNumber,
    Function()? onSuccess,
  }) async {
    final result = await showDialog<bool>(
      context: context,
      barrierDismissible: false,
      builder: (context) => PhoneVerificationDialog(
        phoneNumber: phoneNumber,
        onVerificationSuccess: () {
          Navigator.of(context).pop(true);
          onSuccess?.call();
        },
        onVerificationFailed: (error) {
          AppSnackBar.showError(context, error);
        },
      ),
    );
    return result ?? false;
  }

  @override
  ConsumerState<PhoneVerificationDialog> createState() =>
      _PhoneVerificationDialogState();
}

class _PhoneVerificationDialogState
    extends ConsumerState<PhoneVerificationDialog> {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _sendOTP();
    });
  }

  void _sendOTP() async {
    final result = await ref
        .read(phoneVerificationProvider.notifier)
        .sendOTP(widget.phoneNumber);

    result.fold(
      (error) {
        widget.onVerificationFailed?.call(error);
      },
      (_) {
        final state = ref.read(phoneVerificationProvider);
        if (state.isVerified) {
          widget.onVerificationSuccess();
        }
      },
    );
  }

  void _resendOTP() async {
    final state = ref.read(phoneVerificationProvider);
    if (state.resendCountdown > 0) return;

    final result = await ref
        .read(phoneVerificationProvider.notifier)
        .resendOTP(widget.phoneNumber);

    result.fold(
      (error) {
        if (mounted) {
          AppSnackBar.showError(context, error);
        }
      },
      (_) {
        if (mounted) {
          AppSnackBar.showSuccess(context, 'OTP code resent successfully');
        }
      },
    );
  }

  void _verifyOTP() {
    // Verification is handled in OTPInputField
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final screenWidth = MediaQuery.of(context).size.width;
    final dialogWidth = screenWidth > 400 ? 360.0 : screenWidth * 0.85;

    return Consumer(
      builder: (context, ref, child) {
        final state = ref.watch(phoneVerificationProvider);
        final service = ref.read(phoneVerificationServiceProvider);
        final formattedPhone = service.formatPhoneNumber(widget.phoneNumber);

        return Dialog(
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(16),
          ),
          insetPadding: const EdgeInsets.symmetric(
            horizontal: 20,
            vertical: 24,
          ),
          child: SingleChildScrollView(
            child: Container(
              width: dialogWidth,
              constraints: BoxConstraints(maxWidth: screenWidth * 0.9),
              padding: const EdgeInsets.all(16),
              decoration: BoxDecoration(
                color: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
                borderRadius: BorderRadius.circular(16),
              ),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  VerificationHeader(
                    phoneNumber: widget.phoneNumber,
                    isDark: isDark,
                  ),
                  const SizedBox(height: 16),
                  PhoneDisplay(phoneNumber: formattedPhone, isDark: isDark),
                  const SizedBox(height: 20),
                  if (state.isLoading && !state.codeSent)
                    OTPLoadingState(isDark: isDark)
                  else if (state.codeSent)
                    OTPInputField(
                      isDark: isDark,
                      phoneNumber: widget.phoneNumber,
                      onVerificationSuccess: widget.onVerificationSuccess,
                      onResend: _resendOTP,
                    ),
                  if (state.errorMessage != null) ...[
                    const SizedBox(height: 12),
                    VerificationErrorMessage(errorMessage: state.errorMessage!),
                  ],
                  const SizedBox(height: 16),
                  VerificationActionButtons(
                    isDark: isDark,
                    state: state,
                    onSendOTP: _sendOTP,
                    onVerifyOTP: _verifyOTP,
                  ),
                ],
              ),
            ),
          ),
        );
      },
    );
  }
}
