import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/system/report/domain/entities/entities.dart';
import 'package:labuda/domains/system/report/presentation/screens/report_submission_screen.dart';

/// Report Screen — full-screen entry point for reporting.
///
/// The report form is a substantial form and therefore a full screen (locked UX
/// decision), not a bottom sheet. This route parses the target and renders the
/// canonical [ReportSubmissionScreen].
class ReportScreen extends StatelessWidget {
  final String? targetType;
  final String? targetId;

  /// Display-only title of the reported target, carried by the canonical
  /// report location (`?title=`). Never used for identity.
  final String? targetTitle;

  const ReportScreen({
    super.key,
    this.targetType,
    this.targetId,
    this.targetTitle,
  });

  @override
  Widget build(BuildContext context) {
    // Parse and validate the target type and id
    final reportTargetType = targetType != null
        ? ReportTargetTypeExtension.fromString(targetType!)
        : null;

    // If parameters are invalid, show a plain message on the full screen.
    if (reportTargetType == null || targetId == null || targetId!.isEmpty) {
      return Scaffold(
        appBar: AppBar(title: const Text('Report')),
        body: Center(
          child: Padding(
            padding: const EdgeInsets.all(AppMetrics.p24),
            child: Text(
              'Unable to load report information. Please use the report '
              'button from the content menu.',
              textAlign: TextAlign.center,
              style: context.typeRoles.bodyDense.copyWith(
                color: Theme.of(context).colorScheme.onSurfaceVariant,
              ),
            ),
          ),
        ),
      );
    }

    return ReportSubmissionScreen(
      targetId: targetId!,
      targetType: reportTargetType,
      targetTitle: targetTitle,
    );
  }
}
