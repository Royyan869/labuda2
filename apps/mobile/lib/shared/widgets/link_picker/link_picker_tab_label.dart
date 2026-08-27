import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Build tab label with count indicator
Widget buildLinkPickerTabLabel(String label, int count) {
  return Row(
    mainAxisSize: MainAxisSize.min,
    children: [
      Text(label),
      if (count > 0) ...[
        const SizedBox(width: 6),
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
          decoration: BoxDecoration(
            color: AppColors.primaryRed,
            borderRadius: BorderRadius.circular(10),
          ),
          child: Text(
            count.toString(),
            style: const TextStyle(
              fontSize: 10,
              fontWeight: FontWeight.bold,
              color: Colors.white,
            ),
          ),
        ),
      ],
    ],
  );
}
