import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/system/report/domain/entities/entities.dart';
import 'package:labuda/domains/system/report/presentation/providers/report_providers.dart';
import 'package:labuda/domains/system/report/presentation/widgets/report_description_field.dart';
import 'package:labuda/domains/system/report/presentation/widgets/report_reason_selector.dart';
import 'package:labuda/domains/system/report/presentation/dialogs/report_confirmation_dialog.dart';
import 'package:go_router/go_router.dart';

/// Report Submission Dialog
///
/// Full-featured bottom sheet dialog for submitting content reports.
/// All resource types are backend-supported; dialog always enables submission.
class ReportSubmissionDialog extends ConsumerStatefulWidget {
  final String targetId;
  final ReportTargetType targetType;
  final String? targetTitle;

  const ReportSubmissionDialog({
    super.key,
    required this.targetId,
    required this.targetType,
    this.targetTitle,
  });

  /// Show the report submission dialog
  ///
  /// Returns true if report was submitted successfully, false otherwise.
  static Future<bool?> show(
    BuildContext context, {
    required String targetId,
    required ReportTargetType targetType,
    String? targetTitle,
  }) {
    return showModalBottomSheet<bool>(
      context: context,
      isScrollControlled: true,
      backgroundColor: Colors.transparent,
      builder: (context) => ReportSubmissionDialog(
        targetId: targetId,
        targetType: targetType,
        targetTitle: targetTitle,
      ),
    );
  }

  @override
  ConsumerState<ReportSubmissionDialog> createState() =>
      _ReportSubmissionDialogState();
}

class _ReportSubmissionDialogState
    extends ConsumerState<ReportSubmissionDialog> {
  ReportReasonType? _selectedReason;
  String _description = '';
  bool _isSubmitting = false;

  bool get _canSubmit =>
      _selectedReason != null && !_isSubmitting && widget.targetType.isEnabled;

  @override
  Widget build(BuildContext context) {

    return Container(
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.surface,
        borderRadius: const BorderRadius.vertical(top: Radius.circular(AppShape.r20)),
      ),
      child: SafeArea(
        child: Padding(
          padding: EdgeInsets.only(
            left: AppMetrics.p20,
            right: AppMetrics.p20,
            top: AppMetrics.p20,
            bottom: MediaQuery.of(context).viewInsets.bottom + AppMetrics.p20,
          ),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // Header
              _buildHeader(context),
              const SizedBox(height: 20),

              // Warning if not V1 supported
              if (!widget.targetType.isEnabled) ...[
                _buildComingSoonWarning(context),
                const SizedBox(height: 20),
              ],

              // Target info
              _buildTargetInfo(context),
              const SizedBox(height: 24),

              // Reason selector
              ReportReasonSelector(
                selectedReason: _selectedReason ?? ReportReasonType.other,
                onReasonSelected: (reason) {
                  setState(() => _selectedReason = reason);
                },
                isEnabled: widget.targetType.isEnabled && !_isSubmitting,
              ),
              const SizedBox(height: 24),

              // Description field
              ReportDescriptionField(
                initialValue: _description,
                onChanged: (value) {
                  setState(() => _description = value);
                },
                isEnabled: widget.targetType.isEnabled && !_isSubmitting,
              ),
              const SizedBox(height: 24),

              // Submit button
              SizedBox(
                width: double.infinity,
                child: FilledButton(
                  onPressed: _canSubmit ? _handleSubmit : null,
                  style: FilledButton.styleFrom(
                    backgroundColor: Theme.of(context).colorScheme.primary,
                    foregroundColor: Theme.of(context).colorScheme.onPrimary,
                    padding: const EdgeInsets.symmetric(vertical: AppMetrics.p16),
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(AppShape.r12),
                    ),
                  ),
                  child: _isSubmitting
                      ? SizedBox(
                          width: 20,
                          height: 20,
                          child: CircularProgressIndicator(
                            strokeWidth: 2,
                            color: Theme.of(context).colorScheme.onPrimary,
                          ),
                        )
                      : const Text(
                          'Submit Report',
                          style: TextStyle(
                            fontSize: AppType.s16,
                            fontWeight: FontWeight.w600,
                          ),
                        ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildHeader(BuildContext context) {
    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      children: [
        Text(
          'Report Content',
          style: TextStyle(
            fontSize: AppType.s20,
            fontWeight: FontWeight.w600,
            color: Theme.of(context).colorScheme.onSurface,
          ),
        ),
        IconButton(
          onPressed: () => context.pop(),
          icon: Icon(
            Icons.close,
            color: Theme.of(context).colorScheme.onSurfaceVariant,
          ),
        ),
      ],
    );
  }

  Widget _buildComingSoonWarning(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(AppMetrics.p12),
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.secondary.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(AppShape.r8),
        border: Border.all(color: Theme.of(context).colorScheme.secondary),
      ),
      child: Row(
        children: [
          Icon(Icons.info_outline, color: Theme.of(context).colorScheme.secondary, size: 20),
          const SizedBox(width: 12),
          Expanded(
            child: Text(
              widget.targetType.isV1Supported
                  ? 'This report will be reviewed and may result in content removal.'
                  : 'This report will be reviewed by our team. Enforcement requires manual review.',
              style: TextStyle(
                fontSize: AppType.s13,
                color: Theme.of(context).colorScheme.onSurfaceVariant,
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildTargetInfo(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(AppMetrics.p12),
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.surfaceContainer,
        borderRadius: BorderRadius.circular(AppShape.r8),
      ),
      child: Row(
        children: [
          Icon(_getIconForTargetType(), color: Theme.of(context).colorScheme.secondary, size: 20),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Reporting ${widget.targetType.displayName}',
                  style: TextStyle(
                    fontSize: AppType.s12,
                    color: Theme.of(context).colorScheme.onSurfaceVariant,
                  ),
                ),
                if (widget.targetTitle != null) ...[
                  const SizedBox(height: 2),
                  Text(
                    widget.targetTitle!,
                    style: TextStyle(
                      fontSize: AppType.s14,
                      fontWeight: FontWeight.w500,
                      color: Theme.of(context).colorScheme.onSurface,
                    ),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                ],
              ],
            ),
          ),
        ],
      ),
    );
  }

  IconData _getIconForTargetType() {
    switch (widget.targetType) {
      case ReportTargetType.content:
        return Icons.article_outlined;
      case ReportTargetType.comment:
        return Icons.comment_outlined;
      case ReportTargetType.user:
        return Icons.person_outlined;
      case ReportTargetType.forSale:
        return Icons.shopping_bag_outlined;
      case ReportTargetType.auction:
        return Icons.gavel_outlined;
    }
  }

  Future<void> _handleSubmit() async {
    if (_selectedReason == null) return;

    // D2 HARD GATE (design scope v2): no client-side email-verification
    // preflight — every authenticated user is already verified. The backend
    // stays authoritative for any EMAIL_VERIFICATION_REQUIRED rejection on
    // the submission result.
    setState(() => _isSubmitting = true);

    final request = CreateReportRequest(
      subjectId: widget.targetId,
      subjectType: widget.targetType,
      targetTitle: widget.targetTitle,
      reason: _selectedReason!,
      description: _description.isEmpty ? null : _description,
    );

    final notifier = ref.read(reportActionsNotifierProvider.notifier);
    final success = await notifier.submitReport(request);

    setState(() => _isSubmitting = false);

    if (success && mounted) {
      // Close dialog and show confirmation
      context.pop(true);
      await ReportConfirmationDialog.show(
        context,
        targetType: widget.targetType,
        reason: _selectedReason!,
        isHarassment: _selectedReason == ReportReasonType.harassmentOrAbuse,
      );
    } else if (mounted) {
      // Show error
      final state = ref.read(reportActionsNotifierProvider);
      context.showErrorSnackBar(
        state.error ?? 'Failed to submit report. Please try again.',
      );
    }
  }
}
