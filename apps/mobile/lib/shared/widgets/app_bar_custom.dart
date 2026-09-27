import 'package:flutter/material.dart';
import 'package:labuda/shared/shared.dart';

/// Custom AppBar yang konsisten dan tidak berubah warna saat scroll
///
/// Features:
/// - Mencegah scroll color change dengan surfaceTintColor
/// - Theme adaptive (dark/light)
/// - Shadow effect yang subtle
/// - Consistent design across app
/// - Support custom title dan actions
class AppBarCustom extends StatelessWidget implements PreferredSizeWidget {
  final String title;
  final List<Widget>? actions;
  final Widget? leading;
  final bool centerTitle;
  final bool showBackButton;
  final VoidCallback? onBackPressed;

  const AppBarCustom({
    super.key,
    required this.title,
    this.actions,
    this.leading,
    this.centerTitle = false,
    this.showBackButton = true,
    this.onBackPressed,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;

    return AppBar(
      title: Text(
        title,
        style: TextStyle(
          fontSize: 18,
          fontWeight: FontWeight.w600,
          color: scheme.onSurface,
        ),
      ),
      backgroundColor: scheme.surface,
      foregroundColor: scheme.onSurface,
      elevation: 0,
      centerTitle: centerTitle,
      surfaceTintColor: Colors.transparent, // CRITICAL: Prevent Material 3 tint
      scrolledUnderElevation: 0, // CRITICAL: Prevent scroll color change
      automaticallyImplyLeading: false, // Use custom back button logic
      leading:
          leading ??
          (showBackButton ? AppBackButton(onPressed: onBackPressed) : null),
      actions: actions,
    );
  }

  @override
  Size get preferredSize => const Size.fromHeight(kToolbarHeight);
}
