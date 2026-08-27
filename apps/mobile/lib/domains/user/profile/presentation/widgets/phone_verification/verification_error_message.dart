import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Error message display for verification
class VerificationErrorMessage extends StatelessWidget {
  final String errorMessage;

  const VerificationErrorMessage({super.key, required this.errorMessage});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
      decoration: BoxDecoration(
        color: AppColors.statusError.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: AppColors.statusError.withValues(alpha: 0.3)),
      ),
      child: Row(
        children: [
          const Icon(
            Icons.error_outline,
            size: 16,
            color: AppColors.statusError,
          ),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              errorMessage,
              style: const TextStyle(
                fontSize: 12,
                color: AppColors.statusError,
              ),
            ),
          ),
        ],
      ),
    );
  }
}
