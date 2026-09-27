/// Base Dropdown Container
///
/// Container reusable untuk dropdown wilayah
library;

import 'package:flutter/material.dart';

/// Base container untuk dropdown dengan styling konsisten
class BaseDropdownContainer extends StatelessWidget {
  final String? labelText;
  final Widget child;

  const BaseDropdownContainer({super.key, this.labelText, required this.child});

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (labelText != null) ...[
          Text(
            labelText!,
            style: TextStyle(
              fontSize: 14,
              fontWeight: FontWeight.w500,
              color: scheme.onSurface,
            ),
          ),
          const SizedBox(height: 8),
        ],
        Container(
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(12),
            border: Border.all(color: scheme.outlineVariant),
            color: scheme.surface,
          ),
          child: child,
        ),
      ],
    );
  }
}
