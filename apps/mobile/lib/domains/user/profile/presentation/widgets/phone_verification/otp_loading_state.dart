import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Loading state while sending OTP
class OTPLoadingState extends StatelessWidget {
  final bool isDark;

  const OTPLoadingState({super.key, required this.isDark});

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        const SizedBox(
          width: 32,
          height: 32,
          child: CircularProgressIndicator(
            valueColor: AlwaysStoppedAnimation<Color>(AppColors.primaryRed),
            strokeWidth: 3,
          ),
        ),
        const SizedBox(height: 12),
        Text(
          'Sending OTP code...',
          style: TextStyle(
            fontSize: 13,
            color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray600,
          ),
        ),
      ],
    );
  }
}
