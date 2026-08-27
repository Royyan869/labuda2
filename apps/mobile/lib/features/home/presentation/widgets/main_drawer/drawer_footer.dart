import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Drawer footer component dengan version info
///
/// Simple widget showing app version at the bottom of drawer.
class MainDrawerFooter extends StatelessWidget {
  const MainDrawerFooter({super.key});

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Container(
      padding: const EdgeInsets.all(16),
      child: Text(
        'Version 1.0.0',
        style: TextStyle(
          color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray500,
          fontSize: 12,
        ),
      ),
    );
  }
}
