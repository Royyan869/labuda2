import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/widgets/app_snackbar.dart';
import 'package:labuda/generated/app_localizations.dart';

/// Reusable Theme Selector Component
///
/// Features:
/// - Dropdown dengan icon dan nama theme
/// - Terintegrasi dengan theme provider
/// - Responsive design untuk drawer dan settings
/// - Real-time theme switching
/// - Support Light, Dark, dan System mode
class ThemeSelector extends ConsumerWidget {
  final bool showLeadingIcon;
  final bool isCompact;
  final EdgeInsets? padding;

  const ThemeSelector({
    super.key,
    this.showLeadingIcon = true,
    this.isCompact = false,
    this.padding,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final themeState = ref.watch(themeControllerProvider);
    final isDark = themeState.isDarkMode(context);
    final l10n = AppLocalizations.of(context)!;
    final currentTheme = themeState.themeMode;

    if (isCompact) {
      return _buildCompactSelector(context, ref, isDark, currentTheme);
    }

    return _buildFullSelector(context, ref, isDark, l10n, currentTheme);
  }

  Widget _buildFullSelector(
    BuildContext context,
    WidgetRef ref,
    bool isDark,
    AppLocalizations? l10n,
    ThemeMode currentTheme,
  ) {
    return Padding(
      padding: padding ?? EdgeInsets.zero,
      child: ListTile(
        leading: showLeadingIcon
            ? Icon(
                _getThemeIcon(currentTheme, isDark),
                color: isDark
                    ? AppColors.neutralGray300
                    : AppColors.neutralGray600,
              )
            : null,
        title: Text(
          l10n?.theme ?? 'Theme',
          style: TextStyle(
            color: isDark ? AppColors.neutralGray200 : AppColors.neutralGray800,
            fontWeight: FontWeight.w500,
          ),
        ),
        trailing: Container(
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
          decoration: BoxDecoration(
            color: isDark ? AppColors.darkGray700 : AppColors.neutralGray100,
            border: Border.all(
              color: isDark ? AppColors.darkGray600 : AppColors.neutralGray300,
            ),
            borderRadius: BorderRadius.circular(8),
          ),
          child: DropdownButtonHideUnderline(
            child: DropdownButton<ThemeMode>(
              value: currentTheme,
              isDense: true,
              icon: Icon(
                Icons.keyboard_arrow_down,
                color: isDark
                    ? AppColors.neutralGray400
                    : AppColors.neutralGray600,
                size: 16,
              ),
              dropdownColor: isDark
                  ? AppColors.darkGray700
                  : AppColors.neutralWhite,
              items: ThemeMode.values.map((themeMode) {
                return DropdownMenuItem<ThemeMode>(
                  value: themeMode,
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(
                        themeMode.icon,
                        size: 16,
                        color: isDark
                            ? AppColors.neutralGray300
                            : AppColors.neutralGray600,
                      ),
                      const SizedBox(width: 8),
                      Text(
                        _getThemeDisplayName(themeMode, l10n),
                        style: TextStyle(
                          color: isDark
                              ? AppColors.neutralGray200
                              : AppColors.neutralGray800,
                          fontSize: 14,
                        ),
                      ),
                    ],
                  ),
                );
              }).toList(),
              onChanged: (ThemeMode? newTheme) {
                if (newTheme != null && newTheme != currentTheme) {
                  ref
                      .read(themeControllerProvider.notifier)
                      .setThemeMode(newTheme);

                  // Show success message
                  AppSnackBar.showSuccess(
                    context,
                    'Theme changed to ${_getThemeDisplayName(newTheme, l10n)}',
                    duration: const Duration(seconds: 2),
                  );
                }
              },
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildCompactSelector(
    BuildContext context,
    WidgetRef ref,
    bool isDark,
    ThemeMode currentTheme,
  ) {
    final l10n = AppLocalizations.of(context)!;

    return Container(
      padding:
          padding ?? const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray700 : AppColors.neutralGray100,
        border: Border.all(
          color: isDark ? AppColors.darkGray600 : AppColors.neutralGray300,
        ),
        borderRadius: BorderRadius.circular(8),
      ),
      child: DropdownButtonHideUnderline(
        child: DropdownButton<ThemeMode>(
          value: currentTheme,
          isDense: true,
          icon: Icon(
            Icons.keyboard_arrow_down,
            color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray600,
            size: 16,
          ),
          items: ThemeMode.values.map((themeMode) {
            return DropdownMenuItem<ThemeMode>(
              value: themeMode,
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(
                    themeMode.icon,
                    size: 16,
                    color: isDark
                        ? AppColors.neutralGray300
                        : AppColors.neutralGray600,
                  ),
                  const SizedBox(width: 6),
                  Text(
                    _getThemeDisplayName(themeMode, l10n),
                    style: TextStyle(
                      color: isDark
                          ? AppColors.neutralGray200
                          : AppColors.neutralGray800,
                      fontSize: 14,
                      fontWeight: FontWeight.w500,
                    ),
                  ),
                ],
              ),
            );
          }).toList(),
          onChanged: (ThemeMode? newTheme) {
            if (newTheme != null && newTheme != currentTheme) {
              ref.read(themeControllerProvider.notifier).setThemeMode(newTheme);
            }
          },
        ),
      ),
    );
  }

  IconData _getThemeIcon(ThemeMode themeMode, bool isDark) {
    switch (themeMode) {
      case ThemeMode.light:
        return Icons.light_mode;
      case ThemeMode.dark:
        return Icons.dark_mode;
      case ThemeMode.system:
        return isDark ? Icons.dark_mode : Icons.light_mode;
    }
  }

  String _getThemeDisplayName(ThemeMode themeMode, AppLocalizations? l10n) {
    switch (themeMode) {
      case ThemeMode.light:
        return l10n?.lightTheme ?? 'Light';
      case ThemeMode.dark:
        return l10n?.darkTheme ?? 'Dark';
      case ThemeMode.system:
        return 'System';
    }
  }
}

/// Theme Selector untuk Settings page
class ThemeSelectorTile extends ConsumerWidget {
  final EdgeInsets? contentPadding;

  const ThemeSelectorTile({super.key, this.contentPadding});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final themeState = ref.watch(themeControllerProvider);
    final isDark = themeState.isDarkMode(context);
    final l10n = AppLocalizations.of(context)!;
    final currentTheme = themeState.themeMode;

    return ListTile(
      leading: Icon(
        _getThemeIcon(currentTheme, isDark),
        color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray600,
        size: 24,
      ),
      title: Text(
        l10n.theme,
        style: TextStyle(
          color: isDark ? AppColors.neutralGray200 : AppColors.neutralGray900,
          fontSize: 16,
          fontWeight: FontWeight.w500,
        ),
      ),
      subtitle: Text(
        _getThemeDisplayName(currentTheme, l10n),
        style: TextStyle(
          color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray600,
          fontSize: 14,
        ),
      ),
      trailing: Icon(
        Icons.chevron_right,
        color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray500,
      ),
      contentPadding:
          contentPadding ??
          const EdgeInsets.symmetric(horizontal: 24, vertical: 4),
      onTap: () => _showThemeBottomSheet(context, ref),
    );
  }

  IconData _getThemeIcon(ThemeMode themeMode, bool isDark) {
    switch (themeMode) {
      case ThemeMode.light:
        return Icons.light_mode;
      case ThemeMode.dark:
        return Icons.dark_mode;
      case ThemeMode.system:
        return Icons.brightness_auto;
    }
  }

  String _getThemeDisplayName(ThemeMode themeMode, AppLocalizations? l10n) {
    switch (themeMode) {
      case ThemeMode.light:
        return l10n?.lightTheme ?? 'Light';
      case ThemeMode.dark:
        return l10n?.darkTheme ?? 'Dark';
      case ThemeMode.system:
        return 'System Default';
    }
  }

  void _showThemeBottomSheet(BuildContext context, WidgetRef ref) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final l10n = AppLocalizations.of(context)!;
    final currentTheme = ref.read(themeControllerProvider).themeMode;

    showModalBottomSheet<void>(
      context: context,
      backgroundColor: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (BuildContext context) {
        return Container(
          padding: const EdgeInsets.all(20),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // Handle bar
              Center(
                child: Container(
                  width: 40,
                  height: 4,
                  decoration: BoxDecoration(
                    color: isDark
                        ? AppColors.neutralGray600
                        : AppColors.neutralGray300,
                    borderRadius: BorderRadius.circular(2),
                  ),
                ),
              ),
              const SizedBox(height: 20),

              // Title
              Text(
                l10n.theme,
                style: TextStyle(
                  color: isDark
                      ? AppColors.neutralGray200
                      : AppColors.neutralGray900,
                  fontSize: 20,
                  fontWeight: FontWeight.bold,
                ),
              ),
              const SizedBox(height: 20),

              // Theme options
              ...ThemeMode.values.map((themeMode) {
                final isSelected = themeMode == currentTheme;
                return ListTile(
                  contentPadding: EdgeInsets.zero,
                  leading: Icon(
                    _getThemeIcon(themeMode, isDark),
                    color: isSelected
                        ? AppColors.primaryRed
                        : (isDark
                              ? AppColors.neutralGray400
                              : AppColors.neutralGray600),
                    size: 24,
                  ),
                  title: Text(
                    _getThemeDisplayName(themeMode, l10n),
                    style: TextStyle(
                      color: isDark
                          ? AppColors.neutralGray200
                          : AppColors.neutralGray900,
                      fontSize: 16,
                      fontWeight: isSelected
                          ? FontWeight.w600
                          : FontWeight.w500,
                    ),
                  ),
                  subtitle: themeMode == ThemeMode.system
                      ? Text(
                          'Follow system setting',
                          style: TextStyle(
                            color: isDark
                                ? AppColors.neutralGray500
                                : AppColors.neutralGray500,
                            fontSize: 12,
                          ),
                        )
                      : null,
                  trailing: isSelected
                      ? Icon(
                          Icons.check_circle,
                          color: AppColors.primaryRed,
                          size: 20,
                        )
                      : null,
                  onTap: () {
                    if (themeMode != currentTheme) {
                      ref
                          .read(themeControllerProvider.notifier)
                          .setThemeMode(themeMode);

                      // Show success message
                      AppSnackBar.showSuccess(
                        context,
                        'Theme changed to ${_getThemeDisplayName(themeMode, l10n)}',
                        duration: const Duration(seconds: 2),
                      );
                    }
                    Navigator.of(context).pop();
                  },
                );
              }),

              const SizedBox(height: 20),
            ],
          ),
        );
      },
    );
  }
}
