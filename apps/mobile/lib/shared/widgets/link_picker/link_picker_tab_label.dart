import 'package:flutter/material.dart';
import 'package:labuda/core/src/theme/app_theme.dart';

/// Build tab label with count indicator
Widget buildLinkPickerTabLabel(BuildContext context, String label, int count) {
  final scheme = Theme.of(context).colorScheme;
  return Row(
    mainAxisSize: MainAxisSize.min,
    children: [
      Text(label),
      if (count > 0) ...[
        const SizedBox(width: AppMetrics.p8),
        Container(
          padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p8, vertical: AppMetrics.p4),
          decoration: BoxDecoration(
            color: scheme.primary,
            borderRadius: BorderRadius.circular(AppShape.r10),
          ),
          child: Text(
            count.toString(),
            style: TextStyle(
              fontSize: AppType.s12,
              fontWeight: FontWeight.bold,
              color: scheme.onPrimary,
            ),
          ),
        ),
      ],
    ],
  );
}
