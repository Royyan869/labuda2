import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Error message display for verification
class VerificationErrorMessage extends StatelessWidget {
  final String errorMessage;

  const VerificationErrorMessage({super.key, required this.errorMessage});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p12, vertical: AppMetrics.p8),
      decoration: BoxDecoration(
        color: context.statusColors.error.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(AppShape.r8),
        border: Border.all(color: context.statusColors.error.withValues(alpha: 0.3)),
      ),
      child: Row(
        children: [
          Icon(
            Icons.error_outline,
            size: 16,
            color: context.statusColors.error,
          ),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              errorMessage,
              style: TextStyle(
                fontSize: AppType.s12,
                color: context.statusColors.error,
              ),
            ),
          ),
        ],
      ),
    );
  }
}
