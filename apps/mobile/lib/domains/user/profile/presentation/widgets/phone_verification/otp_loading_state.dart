import 'package:flutter/material.dart';
import 'package:labuda/core/src/theme/app_theme.dart';

/// Loading state while sending OTP
class OTPLoadingState extends StatelessWidget {
  const OTPLoadingState({super.key});

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Column(
      children: [
        SizedBox(
          width: 32,
          height: 32,
          child: CircularProgressIndicator(
            valueColor: AlwaysStoppedAnimation<Color>(scheme.primary),
            strokeWidth: 3,
          ),
        ),
        const SizedBox(height: 12),
        Text(
          'Sending OTP code...',
          style: context.typeRoles.bodyDense.copyWith(
            color: scheme.onSurfaceVariant,
          ),
        ),
      ],
    );
  }
}
