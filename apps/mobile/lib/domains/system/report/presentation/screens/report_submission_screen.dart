import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/system/report/domain/entities/entities.dart';
import 'package:labuda/domains/system/report/presentation/providers/report_providers.dart';
import 'package:labuda/domains/system/report/presentation/widgets/report_description_field.dart';
import 'package:labuda/domains/system/report/presentation/widgets/report_reason_selector.dart';
import 'package:labuda/shared/shared.dart';

/// Report Submission Screen
///
/// The report form is a SUBSTANTIAL form, so per the locked UX decision it lives
/// on a full screen, not inside a bottom sheet. Consumers open it with [open]
/// (a normal push that resolves `true` when a report was submitted).
class ReportSubmissionScreen extends ConsumerStatefulWidget {
  final String targetId;
  final ReportTargetType targetType;
  final String? targetTitle;

  const ReportSubmissionScreen({
    super.key,
    required this.targetId,
    required this.targetType,
    this.targetTitle,
  });

  @override
  ConsumerState<ReportSubmissionScreen> createState() =>
      _ReportSubmissionScreenState();
}

class _ReportSubmissionScreenState
    extends ConsumerState<ReportSubmissionScreen> {
  ReportReasonType? _selectedReason;
  String _description = '';
  bool _isSubmitting = false;

  bool get _canSubmit =>
      _selectedReason != null && !_isSubmitting && widget.targetType.isEnabled;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Report')),
      // Canonical body-level bottom-inset authority (SAFE-AREA-18): the ONE
      // `SafeArea` consumes the live system bottom inset for the whole body.
      // The scroll view's explicit `p24` padding below is DESIGN spacing only
      // — an explicit `ScrollView.padding` never inherits MediaQuery padding.
      body: SafeArea(
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(AppMetrics.p24),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // Warning if not V1 supported
              if (!widget.targetType.isEnabled) ...[
                _buildComingSoonWarning(context),
                const SizedBox(height: AppMetrics.p24),
              ],

              // Target info
              _buildTargetInfo(context),
              const SizedBox(height: AppMetrics.p24),

              // Reason selector
              ReportReasonSelector(
                selectedReason: _selectedReason ?? ReportReasonType.other,
                onReasonSelected: (reason) {
                  setState(() => _selectedReason = reason);
                },
                isEnabled: widget.targetType.isEnabled && !_isSubmitting,
              ),
              const SizedBox(height: AppMetrics.p24),

              // Description field
              ReportDescriptionField(
                initialValue: _description,
                onChanged: (value) {
                  setState(() => _description = value);
                },
                isEnabled: widget.targetType.isEnabled && !_isSubmitting,
              ),
              const SizedBox(height: AppMetrics.p24),

              // Submit button
              SizedBox(
                width: double.infinity,
                child: FilledButton(
                  onPressed: _canSubmit ? _handleSubmit : null,
                  style: FilledButton.styleFrom(
                    padding: const EdgeInsets.symmetric(
                      vertical: AppMetrics.p16,
                    ),
                  ),
                  child: _isSubmitting
                      ? SizedBox(
                          width: AppIconSize.action,
                          height: AppIconSize.action,
                          child: CircularProgressIndicator(
                            strokeWidth: 2,
                            color: Theme.of(context).colorScheme.onPrimary,
                          ),
                        )
                      : Text(
                          'Submit Report',
                          style: Theme.of(context).textTheme.labelLarge
                              ?.copyWith(fontWeight: FontWeight.w600),
                        ),
                ),
              ),
            ],
          ),
        ),
      ),
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
          Icon(
            Icons.info_outline,
            color: Theme.of(context).colorScheme.secondary,
            size: AppIconSize.action,
          ),
          const SizedBox(width: AppMetrics.p12),
          Expanded(
            child: Text(
              widget.targetType.isV1Supported
                  ? 'This report will be reviewed and may result in content removal.'
                  : 'This report will be reviewed by our team. Enforcement requires manual review.',
              style: context.typeRoles.bodyDense.copyWith(
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
          Icon(
            _getIconForTargetType(),
            color: Theme.of(context).colorScheme.secondary,
            size: AppIconSize.action,
          ),
          const SizedBox(width: AppMetrics.p12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Reporting ${widget.targetType.displayName}',
                  style: context.typeRoles.labelMicro.copyWith(
                    color: Theme.of(context).colorScheme.onSurfaceVariant,
                  ),
                ),
                if (widget.targetTitle != null) ...[
                  const SizedBox(height: 2),
                  Text(
                    widget.targetTitle!,
                    style: context.typeRoles.titleCompact.copyWith(
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
      // Acknowledge over the form, then close back to the source screen.
      // Caller-owned notice content on the canonical AppDialog.info surface
      // (framework-bounded scroll + single Got-it action owned there).
      final isHarassment =
          _selectedReason == ReportReasonType.harassmentOrAbuse;
      await AppDialog.info(
        context: context,
        title: 'Report Submitted',
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // Success indication.
            Center(
              child: Container(
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
            ),
            const SizedBox(height: 16),
            // Thanks message.
            Text(
              'Thank you for helping keep our community safe.',
              style: context.typeRoles.bodyDense.copyWith(
                color: Theme.of(context).colorScheme.onSurfaceVariant,
              ),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 16),
            // What happens next.
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
                    style: context.typeRoles.titleCompact.copyWith(
                      fontWeight: FontWeight.w600,
                      color: Theme.of(context).colorScheme.onSurface,
                    ),
                  ),
                  const SizedBox(height: 8),
                  Row(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Icon(
                        Icons.remove_red_eye_outlined,
                        size: AppIconSize.inlineGlyph,
                        color: Theme.of(context).colorScheme.secondary,
                      ),
                      const SizedBox(width: 8),
                      Expanded(
                        child: Text(
                          'Our team will review this ${widget.targetType.displayName.toLowerCase()}',
                          style: context.typeRoles.bodyDense.copyWith(
                            color: Theme.of(
                              context,
                            ).colorScheme.onSurfaceVariant,
                          ),
                        ),
                      ),
                    ],
                  ),
                  const SizedBox(height: 4),
                  Row(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Icon(
                        Icons.schedule_outlined,
                        size: AppIconSize.inlineGlyph,
                        color: Theme.of(context).colorScheme.secondary,
                      ),
                      const SizedBox(width: 8),
                      Expanded(
                        child: Text(
                          'This usually takes 24-48 hours',
                          style: context.typeRoles.bodyDense.copyWith(
                            color: Theme.of(
                              context,
                            ).colorScheme.onSurfaceVariant,
                          ),
                        ),
                      ),
                    ],
                  ),
                  const SizedBox(height: 4),
                  Row(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Icon(
                        Icons.shield_outlined,
                        size: AppIconSize.inlineGlyph,
                        color: Theme.of(context).colorScheme.secondary,
                      ),
                      const SizedBox(width: 8),
                      Expanded(
                        child: Text(
                          'We\'ll take action if it violates our guidelines',
                          style: context.typeRoles.bodyDense.copyWith(
                            color: Theme.of(
                              context,
                            ).colorScheme.onSurfaceVariant,
                          ),
                        ),
                      ),
                    ],
                  ),
                ],
              ),
            ),
            // Block suggestion for harassment.
            if (isHarassment) ...[
              const SizedBox(height: 16),
              Container(
                padding: const EdgeInsets.all(AppMetrics.p12),
                decoration: BoxDecoration(
                  color: Theme.of(
                    context,
                  ).colorScheme.primary.withValues(alpha: 0.1),
                  borderRadius: BorderRadius.circular(AppShape.r8),
                  border: Border.all(
                    color: Theme.of(
                      context,
                    ).colorScheme.primary.withValues(alpha: 0.3),
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
                        style: context.typeRoles.bodyDense.copyWith(
                          color:
                              Theme.of(context).colorScheme.onSurfaceVariant,
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ],
          ],
        ),
        closeLabel: 'Got it',
        barrierDismissible: false,
      );
      if (mounted) Navigator.of(context).pop(true);
    } else if (mounted) {
      final state = ref.read(reportActionsNotifierProvider);
      AppSnackBar.showError(
        context,
        state.error ?? 'Gagal mengirim laporan. Coba lagi.',
      );
    }
  }
}
