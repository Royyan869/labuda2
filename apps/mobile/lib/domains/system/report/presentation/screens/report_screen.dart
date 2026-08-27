import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/system/report/domain/entities/entities.dart';
import 'package:labuda/domains/system/report/presentation/dialogs/report_submission_dialog.dart';

/// Report Screen - Entry point for content reporting
///
/// This screen is typically navigated to from the context menu (3-dot menu).
/// It immediately shows the report submission bottom sheet.
class ReportScreen extends StatelessWidget {
  final String? targetType;
  final String? targetId;

  const ReportScreen({super.key, this.targetType, this.targetId});

  @override
  Widget build(BuildContext context) {
    // Parse and validate the target type and id
    final reportTargetType = targetType != null
        ? ReportTargetTypeExtension.fromString(targetType!)
        : null;

    // If parameters are invalid, show error and go back
    if (reportTargetType == null || targetId == null || targetId!.isEmpty) {
      // Show dialog and pop immediately after
      WidgetsBinding.instance.addPostFrameCallback((_) {
        _showInvalidParamsDialog(context);
      });
      return const Scaffold(body: SizedBox.shrink());
    }

    // Show the report dialog immediately when screen loads
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _showReportDialog(context, reportTargetType);
    });

    return Scaffold(
      appBar: AppBar(
        title: const Text('Report'),
        leading: IconButton(
          icon: const Icon(Icons.close),
          onPressed: () => context.pop(),
        ),
      ),
      body: Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            const Icon(
              Icons.report_outlined,
              size: 64,
              color: AppColors.primaryRed,
            ),
            const SizedBox(height: 16),
            const Text(
              'Report Content',
              style: TextStyle(fontSize: 24, fontWeight: FontWeight.bold),
            ),
            const SizedBox(height: 8),
            Text(
              'Reporting ${reportTargetType.displayName}...',
              style: const TextStyle(color: AppColors.neutralGray500),
              textAlign: TextAlign.center,
            ),
          ],
        ),
      ),
    );
  }

  void _showInvalidParamsDialog(BuildContext context) {
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        icon: const Icon(Icons.error_outline, color: AppColors.error, size: 48),
        title: const Text('Invalid Report'),
        content: const Text(
          'Unable to load report information. Please use the report button from the content menu.',
        ),
        actions: [
          TextButton(
            onPressed: () => context.pop(),
            child: const Text('Go Back'),
          ),
        ],
      ),
    ).then((_) {
      if (context.mounted) context.pop();
    });
  }

  Future<void> _showReportDialog(
    BuildContext context,
    ReportTargetType targetType,
  ) async {
    final result = await ReportSubmissionDialog.show(
      context,
      targetId: targetId ?? '',
      targetType: targetType,
    );

    // Close the screen after dialog is dismissed
    if (context.mounted) {
      context.pop(result);
    }
  }
}
