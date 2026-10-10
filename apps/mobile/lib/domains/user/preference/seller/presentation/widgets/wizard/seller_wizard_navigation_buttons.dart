import 'package:flutter/material.dart';
import 'package:hishumi/shared/widgets/bottom_action_bar.dart';

/// Navigation buttons for Seller Wizard — content only. Chrome (surface,
/// separator, Safe Area, keyboard inset, button height, disabled language) is
/// owned by [BottomActionBar].
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
    final isLast = currentStep == totalSteps - 1;
    return BottomActionBar(
      secondary: currentStep > 0
          ? BottomBarAction(label: 'Kembali', onPressed: onPrevious)
          : null,
      primary: BottomBarAction(
        label: currentStep == 0
            ? 'Lanjut Lengkapi Data'
            : isLast
            ? 'Bayar Sekarang'
            : 'Lanjut',
        onPressed: isLast
            ? (canSubmit ? onSubmit : null)
            : (isCurrentStepValid ? onNext : null),
      ),
    );
  }
}
