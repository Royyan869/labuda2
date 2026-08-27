import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/profile/profile.dart'
    show phoneVerificationProvider, phoneVerificationServiceProvider;

/// Header for phone verification dialog
class VerificationHeader extends ConsumerWidget {
  final String phoneNumber;
  final bool isDark;

  const VerificationHeader({
    super.key,
    required this.phoneNumber,
    required this.isDark,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final state = ref.watch(phoneVerificationProvider);
    final service = ref.read(phoneVerificationServiceProvider);
    final isTestNumber = service.isTestPhoneNumber(phoneNumber);

    return Column(
      children: [
        Container(
          width: 56,
          height: 56,
          decoration: BoxDecoration(
            color: AppColors.primaryRed.withValues(alpha: 0.1),
            shape: BoxShape.circle,
          ),
          child: const Icon(
            Icons.phone_android,
            color: AppColors.primaryRed,
            size: 28,
          ),
        ),
        const SizedBox(height: 12),
        Text(
          'Phone Number Verification',
          style: TextStyle(
            fontSize: 18,
            fontWeight: FontWeight.bold,
            color: isDark ? AppColors.neutralGray200 : AppColors.neutralGray900,
          ),
        ),
        const SizedBox(height: 6),
        Text(
          state.codeSent
              ? 'Enter the 6-digit code sent to you'
              : 'We will send a verification code via SMS',
          textAlign: TextAlign.center,
          style: TextStyle(
            fontSize: 13,
            color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray600,
          ),
        ),
        if (isTestNumber && !state.codeSent) ...[
          const SizedBox(height: 4),
          const Text(
            '🧪 Test mode: OTP code = 123456',
            textAlign: TextAlign.center,
            style: TextStyle(
              fontSize: 11,
              fontWeight: FontWeight.w600,
              color: AppColors.primaryRed,
            ),
          ),
        ],
      ],
    );
  }
}
