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

    return Dialog(
      backgroundColor: Theme.of(context).colorScheme.surface,
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
                color: Theme.of(context).colorScheme.onSurface,
              ),
            ),
            const SizedBox(height: 8),

            // Message
            Text(
              'Thank you for helping keep our community safe.',
              style: TextStyle(fontSize: 14, color: Theme.of(context).colorScheme.onSurfaceVariant),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 16),

            // What happens next
            Container(
              padding: const EdgeInsets.all(16),
              decoration: BoxDecoration(
                color: Theme.of(context).colorScheme.surfaceContainer,
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
                      color: Theme.of(context).colorScheme.onSurface,
                    ),
                  ),
                  const SizedBox(height: 8),
                  _buildExpectationItem(context,
                    icon: Icons.remove_red_eye_outlined,
                    text:
                        'Our team will review this ${targetType.displayName.toLowerCase()}',
                  ),
                  const SizedBox(height: 4),
                  _buildExpectationItem(context,
                    icon: Icons.schedule_outlined,
                    text: 'This usually takes 24-48 hours',
                  ),
                  const SizedBox(height: 4),
                  _buildExpectationItem(context,
                    icon: Icons.shield_outlined,
                    text: 'We\'ll take action if it violates our guidelines',
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
                  color: Theme.of(context).colorScheme.primary.withValues(alpha: 0.1),
                  borderRadius: BorderRadius.circular(8),
                  border: Border.all(
                    color: Theme.of(context).colorScheme.primary.withValues(alpha: 0.3),
                  ),
                ),
                child: Row(
                  children: [
                    Icon(
                      Icons.block,
                      color: Theme.of(context).colorScheme.primary,
                      size: 20,
                    ),
                    const SizedBox(width: 12),
                    Expanded(
                      child: Text(
                        'Want to block this user to prevent further contact?',
                        style: TextStyle(
                          fontSize: 13,
                          color: Theme.of(context).colorScheme.onSurfaceVariant,
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
                  backgroundColor: Theme.of(context).colorScheme.secondary,
                  foregroundColor: Theme.of(context).colorScheme.onPrimary,
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

  Widget _buildExpectationItem(
    BuildContext context, {
    required IconData icon,
    required String text,
  }) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Icon(icon, size: 16, color: Theme.of(context).colorScheme.secondary),
        const SizedBox(width: 8),
        Expanded(
          child: Text(
            text,
            style: TextStyle(
              fontSize: 13,
              color: Theme.of(context).colorScheme.onSurfaceVariant,
            ),
          ),
        ),
      ],
    );
  }
}
