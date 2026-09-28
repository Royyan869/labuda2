import 'package:flutter/material.dart';
import 'package:labuda/core/src/theme/app_theme.dart';

/// Drawer footer component dengan version info
///
/// Simple widget showing app version at the bottom of drawer.
class MainDrawerFooter extends StatelessWidget {
  const MainDrawerFooter({super.key});

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;

    return Container(
      padding: const EdgeInsets.all(AppMetrics.p16),
      child: Text(
        'Version 1.0.0',
        style: TextStyle(
          color: scheme.onSurfaceVariant,
          fontSize: AppType.s12,
        ),
      ),
    );
  }
}
