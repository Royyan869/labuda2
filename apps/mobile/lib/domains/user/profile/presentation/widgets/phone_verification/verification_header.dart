import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/user/profile/profile.dart'
    show phoneVerificationProvider, phoneVerificationServiceProvider;
import 'package:labuda/core/src/theme/app_theme.dart';

/// Header for phone verification dialog
class VerificationHeader extends ConsumerWidget {
  final String phoneNumber;

  const VerificationHeader({
    super.key,
    required this.phoneNumber,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final scheme = Theme.of(context).colorScheme;
    final state = ref.watch(phoneVerificationProvider);
    final service = ref.read(phoneVerificationServiceProvider);
    final isTestNumber = service.isTestPhoneNumber(phoneNumber);

    return Column(
      children: [
        Container(
          width: 56,
          height: 56,
          decoration: BoxDecoration(
            color: scheme.primary.withValues(alpha: 0.1),
            shape: BoxShape.circle,
          ),
          child: Icon(
            Icons.phone_android,
            color: scheme.primary,
            size: AppIconSize.emphasis,
          ),
        ),
        const SizedBox(height: 12),
          Text(
            'Phone Number Verification',
            style: TextStyle(
              fontSize: AppType.s20,
              fontWeight: FontWeight.bold,
              color: scheme.onSurface,
            ),
        ),
        const SizedBox(height: 6),
          Text(
            state.codeSent
                ? 'Enter the 6-digit code sent to you'
                : 'We will send a verification code via SMS',
            textAlign: TextAlign.center,
            style: TextStyle(
              fontSize: AppType.s14,
              color: scheme.onSurfaceVariant,
            ),
        ),
        if (isTestNumber && !state.codeSent) ...[
          const SizedBox(height: 4),
          Text(
            '🧪 Test mode: OTP code = 123456',
            textAlign: TextAlign.center,
            style: TextStyle(
              fontSize: AppType.s12,
              fontWeight: FontWeight.w600,
              color: scheme.primary,
            ),
          ),
        ],
      ],
    );
  }
}
