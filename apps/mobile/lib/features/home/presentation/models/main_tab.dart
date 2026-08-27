import 'package:flutter/material.dart';

/// Tab model untuk main screen navigation
///
/// Simple model untuk mendefinisikan tab di main navigation.
/// Setiap tab memiliki label, icon, dan halaman yang ditampilkan.
class MainTab {
  final String label;
  final IconData icon;
  final IconData selectedIcon;
  final Widget page;

  const MainTab({
    required this.label,
    required this.icon,
    required this.selectedIcon,
    required this.page,
  });
}
