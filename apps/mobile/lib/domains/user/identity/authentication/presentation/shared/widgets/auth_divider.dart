import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

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
/// AuthDivider(text: 'Continue with', margin: EdgeInsets.only(top: 16))
/// ```
class AuthDivider extends StatelessWidget {
  final String text;
  final EdgeInsetsGeometry? margin;

  const AuthDivider({super.key, this.text = 'or', this.margin});

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Padding(
      padding: margin ?? const EdgeInsets.symmetric(vertical: 24),
      child: Row(
        children: [
          Expanded(
            child: Divider(
              color: isDark ? AppColors.darkGray600 : AppColors.neutralGray300,
              thickness: 1,
            ),
          ),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 16),
            child: Text(
              text,
              style: Theme.of(context).textTheme.bodySmall?.copyWith(
                color: isDark
                    ? AppColors.neutralGray500
                    : AppColors.neutralGray500,
                fontWeight: FontWeight.w500,
              ),
            ),
          ),
          Expanded(
            child: Divider(
              color: isDark ? AppColors.darkGray600 : AppColors.neutralGray300,
              thickness: 1,
            ),
          ),
        ],
      ),
    );
  }
}
