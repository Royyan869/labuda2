import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:hishumi/shared/shared.dart';
import 'package:hishumi/domains/user/profile/profile.dart'
    show phoneVerificationProvider, phoneVerificationServiceProvider;
import 'package:hishumi/domains/user/profile/presentation/widgets/phone_verification/verification_header.dart';
import 'package:hishumi/domains/user/profile/presentation/widgets/phone_verification/phone_display.dart';
import 'package:hishumi/domains/user/profile/presentation/widgets/phone_verification/otp_loading_state.dart';
import 'package:hishumi/domains/user/profile/presentation/widgets/phone_verification/otp_input_field.dart';
import 'package:hishumi/domains/user/profile/presentation/widgets/phone_verification/verification_error_message.dart';
import 'package:hishumi/domains/user/profile/presentation/widgets/phone_verification/verification_action_buttons.dart';
import 'package:hishumi/core/src/theme/app_theme.dart';

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
    final screenWidth = MediaQuery.of(context).size.width;
    final dialogWidth = screenWidth > 400 ? 360.0 : screenWidth * 0.85;

    return Consumer(
      builder: (context, ref, child) {
        final state = ref.watch(phoneVerificationProvider);
        final service = ref.read(phoneVerificationServiceProvider);
        final formattedPhone = service.formatPhoneNumber(widget.phoneNumber);

        return Dialog(
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(AppShape.r16),
          ),
          insetPadding: const EdgeInsets.symmetric(
            horizontal: AppMetrics.p24,
            vertical: AppMetrics.p24,
          ),
          child: SingleChildScrollView(
            child: Container(
              width: dialogWidth,
              constraints: BoxConstraints(maxWidth: screenWidth * 0.9),
              padding: const EdgeInsets.all(AppMetrics.p16),
              decoration: BoxDecoration(
                 color: Theme.of(context).colorScheme.surfaceContainerHigh,
                borderRadius: BorderRadius.circular(AppShape.r16),
              ),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  VerificationHeader(
                     phoneNumber: widget.phoneNumber,
                  ),
                  const SizedBox(height: 16),
                   PhoneDisplay(phoneNumber: formattedPhone),
                  const SizedBox(height: 20),
                  if (state.isLoading && !state.codeSent)
                     const OTPLoadingState()
                  else if (state.codeSent)
                     OTPInputField(
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
