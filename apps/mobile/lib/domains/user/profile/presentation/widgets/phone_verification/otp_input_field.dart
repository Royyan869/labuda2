import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/profile/profile.dart'
    show phoneVerificationProvider;

/// OTP Input Field - Handles OTP entry and verification
class OTPInputField extends ConsumerStatefulWidget {
  final bool isDark;
  final String phoneNumber;
  final Function() onVerificationSuccess;
  final Function() onResend;

  const OTPInputField({
    super.key,
    required this.isDark,
    required this.phoneNumber,
    required this.onVerificationSuccess,
    required this.onResend,
  });

  @override
  ConsumerState<OTPInputField> createState() => _OTPInputFieldState();
}

class _OTPInputFieldState extends ConsumerState<OTPInputField> {
  final List<TextEditingController> _otpControllers = List.generate(
    6,
    (_) => TextEditingController(),
  );

  final List<FocusNode> _otpFocusNodes = List.generate(6, (_) => FocusNode());

  @override
  void dispose() {
    for (var controller in _otpControllers) {
      controller.dispose();
    }
    for (var node in _otpFocusNodes) {
      node.dispose();
    }
    super.dispose();
  }

  void _verifyOTP() async {
    final otp = _otpControllers.map((c) => c.text).join();

    if (otp.length != 6) {
      return;
    }

    final result = await ref
        .read(phoneVerificationProvider.notifier)
        .verifyOTP(otp, widget.phoneNumber);

    result.fold(
      (error) {
        // Error is already set in provider state
      },
      (_) {
        widget.onVerificationSuccess();
      },
    );
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(phoneVerificationProvider);

    return Column(
      children: [
        // OTP Input boxes
        LayoutBuilder(
          builder: (context, constraints) {
            final availableWidth = constraints.maxWidth;
            final spacing = 4.0;
            final totalSpacing = spacing * 5;
            final boxWidth = ((availableWidth - totalSpacing) / 6).clamp(
              32.0,
              40.0,
            );
            final boxHeight = boxWidth + 2;

            return Row(
              mainAxisAlignment: MainAxisAlignment.center,
              children: List.generate(6, (index) {
                return Container(
                  width: boxWidth,
                  height: boxHeight,
                  margin: EdgeInsets.only(right: index < 5 ? spacing : 0),
                  child: TextFormField(
                    controller: _otpControllers[index],
                    focusNode: _otpFocusNodes[index],
                    keyboardType: TextInputType.number,
                    textAlign: TextAlign.center,
                    style: TextStyle(
                      fontSize: 14,
                      fontWeight: FontWeight.bold,
                      color: widget.isDark
                          ? AppColors.neutralGray200
                          : AppColors.neutralGray900,
                    ),
                    inputFormatters: [
                      LengthLimitingTextInputFormatter(1),
                      FilteringTextInputFormatter.digitsOnly,
                    ],
                    decoration: InputDecoration(
                      contentPadding: EdgeInsets.zero,
                      filled: true,
                      fillColor: widget.isDark
                          ? AppColors.darkGray700.withValues(alpha: 0.5)
                          : AppColors.neutralGray50,
                      border: OutlineInputBorder(
                        borderRadius: BorderRadius.circular(6),
                        borderSide: BorderSide(
                          color: widget.isDark
                              ? AppColors.darkGray600
                              : AppColors.neutralGray300,
                        ),
                      ),
                      enabledBorder: OutlineInputBorder(
                        borderRadius: BorderRadius.circular(6),
                        borderSide: BorderSide(
                          color: widget.isDark
                              ? AppColors.darkGray600
                              : AppColors.neutralGray300,
                        ),
                      ),
                      focusedBorder: OutlineInputBorder(
                        borderRadius: BorderRadius.circular(6),
                        borderSide: const BorderSide(
                          color: AppColors.primaryRed,
                          width: 1.5,
                        ),
                      ),
                      errorBorder: OutlineInputBorder(
                        borderRadius: BorderRadius.circular(6),
                        borderSide: const BorderSide(
                          color: AppColors.statusError,
                        ),
                      ),
                    ),
                    onChanged: (value) {
                      if (value.isNotEmpty && index < 5) {
                        _otpFocusNodes[index + 1].requestFocus();
                      } else if (value.isEmpty && index > 0) {
                        _otpFocusNodes[index - 1].requestFocus();
                      }

                      if (_otpControllers.every((c) => c.text.isNotEmpty)) {
                        _verifyOTP();
                      }
                    },
                  ),
                );
              }),
            );
          },
        ),

        const SizedBox(height: 14),

        // Resend OTP link
        Wrap(
          alignment: WrapAlignment.center,
          crossAxisAlignment: WrapCrossAlignment.center,
          spacing: 4,
          children: [
            Text(
              'Tidak terima?',
              style: TextStyle(
                fontSize: 11,
                color: widget.isDark
                    ? AppColors.neutralGray400
                    : AppColors.neutralGray600,
              ),
            ),
            if (state.resendCountdown > 0)
              Text(
                'Tunggu ${state.resendCountdown}d',
                style: TextStyle(
                  fontSize: 11,
                  color: widget.isDark
                      ? AppColors.neutralGray400
                      : AppColors.neutralGray600,
                ),
              )
            else
              GestureDetector(
                onTap: state.isResending
                    ? null
                    : () {
                        for (var controller in _otpControllers) {
                          controller.clear();
                        }
                        widget.onResend();
                        _otpFocusNodes[0].requestFocus();
                      },
                child: Text(
                  state.isResending ? 'Sending...' : 'Resend',
                  style: const TextStyle(
                    fontSize: 11,
                    color: AppColors.primaryRed,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ),
          ],
        ),
      ],
    );
  }
}
