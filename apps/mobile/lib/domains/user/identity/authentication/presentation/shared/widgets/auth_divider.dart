import 'package:flutter/material.dart';
import 'package:labuda/core/src/theme/app_theme.dart';

/// Authentication divider with "or" text
///
/// Provides consistent divider styling for auth forms.
/// Replaces duplicated divider code from deprecated auth widgets (2026-01 cleanup).
///
/// Example usage:
/// ```dart
/// const AuthDivider()  // Shows "--- or ---"
///
/// AuthDivider(text: 'OR')  // Custom text
///
/// AuthDivider(text: 'Continue with', margin: EdgeInsets.only(top: AppMetrics.p16))
/// ```
class AuthDivider extends StatelessWidget {
  final String text;
  final EdgeInsetsGeometry? margin;

  const AuthDivider({super.key, this.text = 'or', this.margin});

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;

    return Padding(
      padding: margin ?? const EdgeInsets.symmetric(vertical: AppMetrics.p24),
      child: Row(
        children: [
          Expanded(
            child: Divider(
              color: scheme.outlineVariant,
              thickness: 1,
            ),
          ),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p16),
            child: Text(
              text,
              style: Theme.of(context).textTheme.bodySmall?.copyWith(
                color: scheme.onSurfaceVariant,
                fontWeight: FontWeight.w500,
              ),
            ),
          ),
          Expanded(
            child: Divider(
              color: scheme.outlineVariant,
              thickness: 1,
            ),
          ),
        ],
      ),
    );
  }
}
