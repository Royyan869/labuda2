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
    final l10n = AppLocalizations.of(context)!;
    final currentTheme = themeState.themeMode;

    if (isCompact) {
      return _buildCompactSelector(context, ref, currentTheme);
    }

    return _buildFullSelector(context, ref, l10n, currentTheme);
  }

  Widget _buildFullSelector(
    BuildContext context,
    WidgetRef ref,
    AppLocalizations? l10n,
    ThemeMode currentTheme,
  ) {
    final scheme = Theme.of(context).colorScheme;
    return Padding(
      padding: padding ?? EdgeInsets.zero,
      child: ListTile(
        leading: showLeadingIcon
            ? Icon(
                currentTheme.icon,
                color: scheme.onSurfaceVariant,
              )
            : null,
        title: Text(
          l10n?.theme ?? 'Theme',
          style: TextStyle(
            color: scheme.onSurface,
            fontWeight: FontWeight.w500,
          ),
        ),
        trailing: Container(
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
          decoration: BoxDecoration(
            color: scheme.surfaceContainerHighest,
            border: Border.all(color: scheme.outlineVariant),
            borderRadius: BorderRadius.circular(8),
          ),
          child: DropdownButtonHideUnderline(
            child: DropdownButton<ThemeMode>(
              value: currentTheme,
              isDense: true,
              icon: Icon(
                Icons.keyboard_arrow_down,
                color: scheme.onSurfaceVariant,
                size: 16,
              ),
              dropdownColor: scheme.surfaceContainerHigh,
              items: ThemeMode.values.map((themeMode) {
                return DropdownMenuItem<ThemeMode>(
                  value: themeMode,
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(
                        themeMode.icon,
                        size: 16,
                        color: scheme.onSurfaceVariant,
                      ),
                      const SizedBox(width: 8),
                      Text(
                        _getThemeDisplayName(themeMode, l10n),
                        style: TextStyle(
                          color: scheme.onSurface,
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
    ThemeMode currentTheme,
  ) {
    final l10n = AppLocalizations.of(context)!;
    final scheme = Theme.of(context).colorScheme;

    return Container(
      padding:
          padding ?? const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
      decoration: BoxDecoration(
        color: scheme.surfaceContainerHighest,
        border: Border.all(color: scheme.outlineVariant),
        borderRadius: BorderRadius.circular(8),
      ),
      child: DropdownButtonHideUnderline(
        child: DropdownButton<ThemeMode>(
          value: currentTheme,
          isDense: true,
          icon: Icon(
            Icons.keyboard_arrow_down,
            color: scheme.onSurfaceVariant,
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
                    color: scheme.onSurfaceVariant,
                  ),
                  const SizedBox(width: 6),
                  Text(
                    _getThemeDisplayName(themeMode, l10n),
                    style: TextStyle(
                      color: scheme.onSurface,
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
    final scheme = Theme.of(context).colorScheme;
    final l10n = AppLocalizations.of(context)!;
    final currentTheme = themeState.themeMode;

    return ListTile(
      leading: Icon(
        currentTheme.icon,
        color: scheme.onSurfaceVariant,
        size: 24,
      ),
      title: Text(
        l10n.theme,
        style: TextStyle(
          color: scheme.onSurface,
          fontSize: 16,
          fontWeight: FontWeight.w500,
        ),
      ),
      subtitle: Text(
        _getThemeDisplayName(currentTheme, l10n),
        style: TextStyle(
          color: scheme.onSurfaceVariant,
          fontSize: 14,
        ),
      ),
      trailing: Icon(
        Icons.chevron_right,
        color: scheme.onSurfaceVariant,
      ),
      contentPadding:
          contentPadding ??
          const EdgeInsets.symmetric(horizontal: 24, vertical: 4),
      onTap: () => showThemeSelectionSheet(context, ref),
    );
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
}

/// Canonical theme picker sheet — single authority.
///
/// Replaces the copy-pasted sheet that lived in `welcome_screen.dart`
/// (deleted): one bottom sheet, scheme-driven, no brightness branches.
void showThemeSelectionSheet(BuildContext context, WidgetRef ref) {
  final scheme = Theme.of(context).colorScheme;
  final l10n = AppLocalizations.of(context)!;
  final currentTheme = ref.read(themeControllerProvider).themeMode;

  String name(ThemeMode mode) {
    switch (mode) {
      case ThemeMode.light:
        return l10n.lightTheme;
      case ThemeMode.dark:
        return l10n.darkTheme;
      case ThemeMode.system:
        return 'System Default';
    }
  }

  showModalBottomSheet<void>(
    context: context,
    backgroundColor: scheme.surfaceContainerHigh,
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
                  color: scheme.outlineVariant,
                  borderRadius: BorderRadius.circular(2),
                ),
              ),
            ),
            const SizedBox(height: 20),

            // Title
            Text(
              l10n.theme,
              style: TextStyle(
                color: scheme.onSurface,
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
                  themeMode.icon,
                  color: isSelected
                      ? scheme.primary
                      : scheme.onSurfaceVariant,
                  size: 24,
                ),
                title: Text(
                  name(themeMode),
                  style: TextStyle(
                    color: scheme.onSurface,
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
                          color: scheme.onSurfaceVariant,
                          fontSize: 12,
                        ),
                      )
                    : null,
                trailing: isSelected
                    ? Icon(
                        Icons.check_circle,
                        color: scheme.primary,
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
                      'Theme changed to ${name(themeMode)}',
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
