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
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(AppShape.r16)),
      child: Padding(
        padding: const EdgeInsets.all(AppMetrics.p24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            // Success icon
            Container(
              width: 56,
              height: 56,
              decoration: BoxDecoration(
                color: context.statusColors.success.withValues(alpha: 0.1),
                shape: BoxShape.circle,
              ),
              child: Icon(
                Icons.check_circle_outline,
                color: context.statusColors.success,
                size: AppIconSize.emphasis,
              ),
            ),
            const SizedBox(height: 16),

            // Title
            Text(
              'Report Submitted',
              style: TextStyle(
                fontSize: AppType.s20,
                fontWeight: FontWeight.w600,
                color: Theme.of(context).colorScheme.onSurface,
              ),
            ),
            const SizedBox(height: 8),

            // Message
            Text(
              'Thank you for helping keep our community safe.',
              style: TextStyle(fontSize: AppType.s14, color: Theme.of(context).colorScheme.onSurfaceVariant),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 16),

            // What happens next
            Container(
              padding: const EdgeInsets.all(AppMetrics.p16),
              decoration: BoxDecoration(
                color: Theme.of(context).colorScheme.surfaceContainer,
                borderRadius: BorderRadius.circular(AppShape.r12),
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    'What happens next:',
                    style: TextStyle(
                      fontSize: AppType.s14,
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
                padding: const EdgeInsets.all(AppMetrics.p12),
                decoration: BoxDecoration(
                  color: Theme.of(context).colorScheme.primary.withValues(alpha: 0.1),
                  borderRadius: BorderRadius.circular(AppShape.r8),
                  border: Border.all(
                    color: Theme.of(context).colorScheme.primary.withValues(alpha: 0.3),
                  ),
                ),
                child: Row(
                  children: [
                    Icon(
                      Icons.block,
                      color: Theme.of(context).colorScheme.primary,
                      size: AppIconSize.action,
                    ),
                    const SizedBox(width: 12),
                    Expanded(
                      child: Text(
                        'Want to block this user to prevent further contact?',
                        style: TextStyle(
                          fontSize: AppType.s14,
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
                  padding: const EdgeInsets.symmetric(vertical: AppMetrics.p16),
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
        Icon(icon, size: AppIconSize.inlineGlyph, color: Theme.of(context).colorScheme.secondary),
        const SizedBox(width: 8),
        Expanded(
          child: Text(
            text,
            style: TextStyle(
              fontSize: AppType.s14,
              color: Theme.of(context).colorScheme.onSurfaceVariant,
            ),
          ),
        ),
      ],
    );
  }
}
