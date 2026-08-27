import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Navigation buttons for Seller Wizard
/// Extracted from SellerUpgradeWizardScreen to reduce complexity
class SellerWizardNavigationButtons extends StatelessWidget {
  final int currentStep;
  final int totalSteps;
  final bool isCurrentStepValid;
  final bool canSubmit;
  final VoidCallback onPrevious;
  final VoidCallback onNext;
  final VoidCallback onSubmit;
  final bool isDark;

  const SellerWizardNavigationButtons({
    super.key,
    required this.currentStep,
    required this.totalSteps,
    required this.isCurrentStepValid,
    required this.canSubmit,
    required this.onPrevious,
    required this.onNext,
    required this.onSubmit,
    required this.isDark,
  });

  @override
  Widget build(BuildContext context) {
    // Get bottom padding for devices with gesture navigation or navigation bar
    final bottomPadding = MediaQuery.of(context).padding.bottom;

    return Container(
      padding: EdgeInsets.only(
        left: 24,
        right: 24,
        top: 16,
        bottom: bottomPadding > 0
            ? bottomPadding + 16
            : 24, // Add extra padding if navigation bar exists
      ),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
        boxShadow: [
          BoxShadow(
            color: (isDark ? Colors.black : Colors.grey).withValues(alpha: 0.1),
            blurRadius: 4,
            offset: const Offset(0, -2),
          ),
        ],
      ),
      child: Row(
        children: [
          if (currentStep > 0) ...[
            Expanded(
              child: OutlinedButton(
                onPressed: onPrevious,
                style: OutlinedButton.styleFrom(
                  side: const BorderSide(
                    color: AppColors.primaryRed,
                    width: 1.5,
                  ),
                ),
                child: const Text('Kembali'),
              ),
            ),
            const SizedBox(width: 16),
          ],
          Expanded(
            child: ElevatedButton(
              onPressed: currentStep == totalSteps - 1
                  ? (canSubmit ? onSubmit : null)
                  : (isCurrentStepValid ? onNext : null),
              child: Text(
                currentStep == 0
                    ? 'Lanjut Lengkapi Data'
                    : currentStep == totalSteps - 1
                    ? 'Bayar Sekarang'
                    : 'Lanjut',
              ),
            ),
          ),
        ],
      ),
    );
  }
}
