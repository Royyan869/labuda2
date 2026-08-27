import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/system/report/domain/entities/entities.dart';
import 'package:go_router/go_router.dart';

/// Report Confirmation Dialog
///
/// Shows after a report is successfully submitted.
/// Sets user expectations and offers block option for harassment cases.
class ReportConfirmationDialog extends StatelessWidget {
  final ReportTargetType targetType;
  final ReportReasonType reason;
  final bool isHarassment;

  const ReportConfirmationDialog({
    super.key,
    required this.targetType,
    required this.reason,
    this.isHarassment = false,
  });

  /// Show the confirmation dialog
  static Future<void> show(
    BuildContext context, {
    required ReportTargetType targetType,
    required ReportReasonType reason,
    bool isHarassment = false,
  }) {
    return showDialog(
      context: context,
      barrierDismissible: false,
      builder: (context) => ReportConfirmationDialog(
        targetType: targetType,
        reason: reason,
        isHarassment: isHarassment,
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Dialog(
      backgroundColor: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            // Success icon
            Container(
              width: 56,
              height: 56,
              decoration: BoxDecoration(
                color: AppColors.success.withValues(alpha: 0.1),
                shape: BoxShape.circle,
              ),
              child: const Icon(
                Icons.check_circle_outline,
                color: AppColors.success,
                size: 32,
              ),
            ),
            const SizedBox(height: 16),

            // Title
            Text(
              'Report Submitted',
              style: TextStyle(
                fontSize: 20,
                fontWeight: FontWeight.w600,
                color: isDark
                    ? AppColors.neutralWhite
                    : AppColors.neutralGray900,
              ),
            ),
            const SizedBox(height: 8),

            // Message
            Text(
              'Thank you for helping keep our community safe.',
              style: TextStyle(fontSize: 14, color: AppColors.neutralGray500),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 16),

            // What happens next
            Container(
              padding: const EdgeInsets.all(16),
              decoration: BoxDecoration(
                color: isDark
                    ? AppColors.darkGray700
                    : AppColors.neutralGray100,
                borderRadius: BorderRadius.circular(12),
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    'What happens next:',
                    style: TextStyle(
                      fontSize: 14,
                      fontWeight: FontWeight.w600,
                      color: isDark
                          ? AppColors.neutralWhite
                          : AppColors.neutralGray900,
                    ),
                  ),
                  const SizedBox(height: 8),
                  _buildExpectationItem(
                    icon: Icons.remove_red_eye_outlined,
                    text:
                        'Our team will review this ${targetType.displayName.toLowerCase()}',
                    isDark: isDark,
                  ),
                  const SizedBox(height: 4),
                  _buildExpectationItem(
                    icon: Icons.schedule_outlined,
                    text: 'This usually takes 24-48 hours',
                    isDark: isDark,
                  ),
                  const SizedBox(height: 4),
                  _buildExpectationItem(
                    icon: Icons.shield_outlined,
                    text: 'We\'ll take action if it violates our guidelines',
                    isDark: isDark,
                  ),
                ],
              ),
            ),

            // Block suggestion for harassment
            if (isHarassment) ...[
              const SizedBox(height: 16),
              Container(
                padding: const EdgeInsets.all(12),
                decoration: BoxDecoration(
                  color: AppColors.primaryRed.withValues(alpha: 0.1),
                  borderRadius: BorderRadius.circular(8),
                  border: Border.all(
                    color: AppColors.primaryRed.withValues(alpha: 0.3),
                  ),
                ),
                child: Row(
                  children: [
                    const Icon(
                      Icons.block,
                      color: AppColors.primaryRed,
                      size: 20,
                    ),
                    const SizedBox(width: 12),
                    Expanded(
                      child: Text(
                        'Want to block this user to prevent further contact?',
                        style: TextStyle(
                          fontSize: 13,
                          color: isDark
                              ? AppColors.neutralGray300
                              : AppColors.neutralGray700,
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ],

            const SizedBox(height: 24),

            // Close button
            SizedBox(
              width: double.infinity,
              child: FilledButton(
                onPressed: () => context.pop(),
                style: FilledButton.styleFrom(
                  backgroundColor: AppColors.primaryBlue,
                  foregroundColor: AppColors.neutralWhite,
                  padding: const EdgeInsets.symmetric(vertical: 14),
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(10),
                  ),
                ),
                child: const Text('Got it'),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildExpectationItem({
    required IconData icon,
    required String text,
    required bool isDark,
  }) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Icon(icon, size: 16, color: AppColors.primaryBlue),
        const SizedBox(width: 8),
        Expanded(
          child: Text(
            text,
            style: TextStyle(
              fontSize: 13,
              color: isDark
                  ? AppColors.neutralGray300
                  : AppColors.neutralGray700,
            ),
          ),
        ),
      ],
    );
  }
}
