import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/shared/widgets/app_bottom_sheet_list_selection.dart';
import 'package:hishumi/shared/widgets/app_snackbar.dart';
import 'package:hishumi/generated/app_localizations.dart';

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
            ? Icon(currentTheme.icon, color: scheme.onSurfaceVariant)
            : null,
        title: Text(
          l10n?.theme ?? 'Theme',
          style: TextStyle(
            color: scheme.onSurface,
            fontWeight: FontWeight.w500,
          ),
        ),
        trailing: Container(
          padding: const EdgeInsets.symmetric(
            horizontal: AppMetrics.p12,
            vertical: AppMetrics.p8,
          ),
          decoration: BoxDecoration(
            color: scheme.surfaceContainerHighest,
            border: Border.all(color: scheme.outlineVariant),
            borderRadius: BorderRadius.circular(AppShape.r8),
          ),
          child: DropdownButtonHideUnderline(
            child: DropdownButton<ThemeMode>(
              value: currentTheme,
              isDense: true,
              icon: Icon(
                Icons.keyboard_arrow_down,
                color: scheme.onSurfaceVariant,
                size: AppMetrics.p16,
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
                        size: AppMetrics.p16,
                        color: scheme.onSurfaceVariant,
                      ),
                      const SizedBox(width: AppMetrics.p8),
                      Text(
                        _getThemeDisplayName(themeMode, l10n),
                        style: Theme.of(context).textTheme.bodyLarge?.copyWith(
                          color: scheme.onSurface,
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
                    'Tema diubah ke ${_getThemeDisplayName(newTheme, l10n)}',
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
          padding ??
          const EdgeInsets.symmetric(
            horizontal: AppMetrics.p12,
            vertical: AppMetrics.p8,
          ),
      decoration: BoxDecoration(
        color: scheme.surfaceContainerHighest,
        border: Border.all(color: scheme.outlineVariant),
        borderRadius: BorderRadius.circular(AppShape.r8),
      ),
      child: DropdownButtonHideUnderline(
        child: DropdownButton<ThemeMode>(
          value: currentTheme,
          isDense: true,
          icon: Icon(
            Icons.keyboard_arrow_down,
            color: scheme.onSurfaceVariant,
            size: AppIconSize.inlineGlyph,
          ),
          items: ThemeMode.values.map((themeMode) {
            return DropdownMenuItem<ThemeMode>(
              value: themeMode,
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(
                    themeMode.icon,
                    size: AppIconSize.inlineGlyph,
                    color: scheme.onSurfaceVariant,
                  ),
                  const SizedBox(width: 6),
                  Text(
                    _getThemeDisplayName(themeMode, l10n),
                    style: Theme.of(context).textTheme.bodyLarge?.copyWith(
                      color: scheme.onSurface,
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
        size: AppIconSize.header,
      ),
      title: Text(
        l10n.theme,
        style: context.typeRoles.titleCompact.copyWith(
          color: scheme.onSurface,
          fontWeight: FontWeight.w500,
        ),
      ),
      subtitle: Text(
        _getThemeDisplayName(currentTheme, l10n),
        style: context.typeRoles.bodyDense.copyWith(
          color: scheme.onSurfaceVariant,
        ),
      ),
      trailing: Icon(Icons.chevron_right, color: scheme.onSurfaceVariant),
      contentPadding:
          contentPadding ??
          const EdgeInsets.symmetric(
            horizontal: AppMetrics.p24,
            vertical: AppMetrics.p4,
          ),
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

  // Canonical selection builder: surface, shape, handle, scroll and safe area
  // all come from the foundation — no bespoke sheet.
  AppBottomSheetListSelection.showListSelection<ThemeMode>(
    context: context,
    title: l10n.theme,
    selectedValue: currentTheme,
    items: ThemeMode.values
        .map(
          (themeMode) => ListSelectionItem<ThemeMode>(
            title: name(themeMode),
            subtitle:
                themeMode == ThemeMode.system ? 'Follow system setting' : null,
            icon: themeMode.icon,
            value: themeMode,
          ),
        )
        .toList(),
  ).then((themeMode) {
    if (themeMode == null || themeMode == currentTheme || !context.mounted) {
      return;
    }
    ref.read(themeControllerProvider.notifier).setThemeMode(themeMode);
    AppSnackBar.showSuccess(
      context,
      'Tema diubah ke ${name(themeMode)}',
      duration: const Duration(seconds: 2),
    );
  });
}
