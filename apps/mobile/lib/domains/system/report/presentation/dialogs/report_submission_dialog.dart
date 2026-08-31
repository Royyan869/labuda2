import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/system/report/domain/entities/entities.dart';
import 'package:labuda/domains/system/report/presentation/providers/report_providers.dart';
import 'package:labuda/domains/system/report/presentation/widgets/report_description_field.dart';
import 'package:labuda/domains/system/report/presentation/widgets/report_reason_selector.dart';
import 'package:labuda/domains/system/report/presentation/dialogs/report_confirmation_dialog.dart';
import 'package:labuda/domains/user/identity/authentication/presentation/widgets/blocked_action_gate.dart';
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
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Container(
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
        borderRadius: const BorderRadius.vertical(top: Radius.circular(20)),
      ),
      child: SafeArea(
        child: Padding(
          padding: EdgeInsets.only(
            left: 20,
            right: 20,
            top: 20,
            bottom: MediaQuery.of(context).viewInsets.bottom + 20,
          ),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // Header
              _buildHeader(context, isDark),
              const SizedBox(height: 20),

              // Warning if not V1 supported
              if (!widget.targetType.isEnabled) ...[
                _buildComingSoonWarning(context, isDark),
                const SizedBox(height: 20),
              ],

              // Target info
              _buildTargetInfo(context, isDark),
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
                    backgroundColor: AppColors.primaryRed,
                    foregroundColor: AppColors.neutralWhite,
                    padding: const EdgeInsets.symmetric(vertical: 16),
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(12),
                    ),
                  ),
                  child: _isSubmitting
                      ? const SizedBox(
                          width: 20,
                          height: 20,
                          child: CircularProgressIndicator(
                            strokeWidth: 2,
                            color: AppColors.neutralWhite,
                          ),
                        )
                      : const Text(
                          'Submit Report',
                          style: TextStyle(
                            fontSize: 16,
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

  Widget _buildHeader(BuildContext context, bool isDark) {
    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      children: [
        Text(
          'Report Content',
          style: TextStyle(
            fontSize: 20,
            fontWeight: FontWeight.w600,
            color: isDark ? AppColors.neutralWhite : AppColors.neutralGray900,
          ),
        ),
        IconButton(
          onPressed: () => context.pop(),
          icon: Icon(
            Icons.close,
            color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray600,
          ),
        ),
      ],
    );
  }

  Widget _buildComingSoonWarning(BuildContext context, bool isDark) {
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: AppColors.primaryBlue.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: AppColors.primaryBlue),
      ),
      child: Row(
        children: [
          Icon(Icons.info_outline, color: AppColors.primaryBlue, size: 20),
          const SizedBox(width: 12),
          Expanded(
            child: Text(
              widget.targetType.isV1Supported
                  ? 'This report will be reviewed and may result in content removal.'
                  : 'This report will be reviewed by our team. Enforcement requires manual review.',
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
    );
  }

  Widget _buildTargetInfo(BuildContext context, bool isDark) {
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray700 : AppColors.neutralGray100,
        borderRadius: BorderRadius.circular(8),
      ),
      child: Row(
        children: [
          Icon(_getIconForTargetType(), color: AppColors.primaryBlue, size: 20),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Reporting ${widget.targetType.displayName}',
                  style: TextStyle(
                    fontSize: 12,
                    color: AppColors.neutralGray500,
                  ),
                ),
                if (widget.targetTitle != null) ...[
                  const SizedBox(height: 2),
                  Text(
                    widget.targetTitle!,
                    style: TextStyle(
                      fontSize: 14,
                      fontWeight: FontWeight.w500,
                      color: isDark
                          ? AppColors.neutralWhite
                          : AppColors.neutralGray900,
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

    // Pre-flight gate: backend will reject report submission for unverified
    // users (touches T&S system → BLOCKED per email-gating-matrix doctrine).
    final authState = ref.read(authControllerProvider);
    if (authState is AuthStateAuthenticated && !authState.emailVerified) {
      await showBlockedActionGate(
        context,
        actionDescription: 'membuat laporan',
      );
      return;
    }

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
