import 'package:flutter/material.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/core/core.dart';

/// Wrapper AppBar yang konsisten — chrome DARI THEME, bukan dari call site.
///
/// Wrapper ini sengaja TIDAK menyetel warna/tint/elevasi: seluruh chrome
/// (surface datar, tanpa tint, tanpa shadow, tetap 0 saat scroll) warisan
/// `appBarTheme` di AppTheme. Yang dipilih hanya judul, leading kustom, dan
/// actions — lihat test/core/theme/app_bar_authority_contract_test.dart.
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
          fontSize: AppType.s20,
          fontWeight: FontWeight.w600,
          color: scheme.onSurface,
        ),
      ),
      centerTitle: centerTitle,
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
