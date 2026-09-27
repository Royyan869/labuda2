import 'package:flutter/material.dart';

/// Build tab label with count indicator
Widget buildLinkPickerTabLabel(BuildContext context, String label, int count) {
  final scheme = Theme.of(context).colorScheme;
  return Row(
    mainAxisSize: MainAxisSize.min,
    children: [
      Text(label),
      if (count > 0) ...[
        const SizedBox(width: 6),
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
          decoration: BoxDecoration(
            color: scheme.primary,
            borderRadius: BorderRadius.circular(10),
          ),
          child: Text(
            count.toString(),
            style: TextStyle(
              fontSize: 10,
              fontWeight: FontWeight.bold,
              color: scheme.onPrimary,
            ),
          ),
        ),
      ],
    ],
  );
}
