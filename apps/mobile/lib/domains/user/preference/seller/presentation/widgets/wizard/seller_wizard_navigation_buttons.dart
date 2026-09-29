import 'package:flutter/material.dart';
import 'package:labuda/core/src/theme/app_theme.dart';

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

  const SellerWizardNavigationButtons({
    super.key,
    required this.currentStep,
    required this.totalSteps,
    required this.isCurrentStepValid,
    required this.canSubmit,
    required this.onPrevious,
    required this.onNext,
    required this.onSubmit,
  });

  @override
  Widget build(BuildContext context) {
    // Get bottom padding for devices with gesture navigation or navigation bar
    final bottomPadding = MediaQuery.of(context).padding.bottom;
    final scheme = Theme.of(context).colorScheme;

    return Container(
      padding: EdgeInsets.only(
        left: AppMetrics.p24,
        right: AppMetrics.p24,
        top: AppMetrics.p16,
        bottom: bottomPadding > AppMetrics.p0
            ? bottomPadding + AppMetrics.p16
            : AppMetrics.p24, // Add extra padding if navigation bar exists
      ),
      decoration: BoxDecoration(
        color: scheme.surface,
        boxShadow: [
          BoxShadow(
            color: scheme.scrim.withValues(alpha: 0.1),
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
