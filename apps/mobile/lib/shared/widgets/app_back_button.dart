import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';

/// Reusable Back Button dengan styling konsisten
///
/// Features:
/// - Adaptive color berdasarkan theme
/// - Custom onPressed atau auto go_router back navigation
/// - Consistent size dan padding
/// - GUIDELINES compliant navigation using go_router
class AppBackButton extends StatelessWidget {
  final VoidCallback? onPressed;
  final Color? color;

  const AppBackButton({super.key, this.onPressed, this.color});

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return IconButton(
      onPressed:
          onPressed ??
          () {
            // Try Navigator.pop first (for Navigator.push screens)
            // If fails, try GoRouter.pop (for GoRouter screens)
            if (Navigator.of(context).canPop()) {
              Navigator.of(context).pop();
            } else {
              context.pop();
            }
          },
      icon: Icon(
        Icons.arrow_back,
        color:
            color ??
            (isDark ? AppColors.neutralGray300 : AppColors.neutralGray700),
      ),
      tooltip: 'Back',
    );
  }
}
