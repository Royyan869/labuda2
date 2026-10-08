import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/widgets/app_bottom_sheet_list_selection.dart';
import 'package:labuda/shared/widgets/app_snackbar.dart';
import 'package:labuda/generated/app_localizations.dart';

/// Reusable Language Selector Component
///
/// Features:
/// - Dropdown dengan bendera dan nama bahasa
/// - Terintegrasi dengan localization provider
/// - Responsive design untuk drawer dan settings
/// - Real-time language switching
class LanguageSelector extends ConsumerWidget {
  final bool showLeadingIcon;
  final bool isCompact;
  final EdgeInsets? padding;

  const LanguageSelector({
    super.key,
    this.showLeadingIcon = true,
    this.isCompact = false,
    this.padding,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final localizationState = ref.watch(localizationControllerProvider);
    final currentLocale = localizationState.currentLocale;

    if (isCompact) {
      return _buildCompactSelector(context, ref, currentLocale);
    }

    return _buildFullSelector(context, ref, l10n, currentLocale);
  }

  Widget _buildFullSelector(
    BuildContext context,
    WidgetRef ref,
    AppLocalizations l10n,
    SupportedLocale currentLocale,
  ) {
    final scheme = Theme.of(context).colorScheme;
    return Padding(
      padding: padding ?? EdgeInsets.zero,
      child: ListTile(
        leading: showLeadingIcon
            ? Icon(Icons.language, color: scheme.onSurfaceVariant)
            : null,
        title: Text(
          l10n.language,
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
            color: scheme.surfaceContainerHigh,
            border: Border.all(color: scheme.outlineVariant),
            borderRadius: BorderRadius.circular(AppShape.r8),
          ),
          child: DropdownButtonHideUnderline(
            child: DropdownButton<SupportedLocale>(
              value: currentLocale,
              isDense: true,
              icon: Icon(
                Icons.keyboard_arrow_down,
                color: scheme.onSurfaceVariant,
                size: AppMetrics.p16,
              ),
              items: SupportedLocale.values.map((locale) {
                return DropdownMenuItem<SupportedLocale>(
                  value: locale,
                  child: Text(
                    '${locale.flagEmoji} ${locale.displayName}',
                    style: Theme.of(
                      context,
                    ).textTheme.bodyLarge?.copyWith(color: scheme.onSurface),
                  ),
                );
              }).toList(),
              onChanged: (SupportedLocale? newLocale) {
                if (newLocale != null && newLocale != currentLocale) {
                  ref
                      .read(localizationControllerProvider.notifier)
                      .setLocale(newLocale);

                  // Show success message
                  AppSnackBar.showSuccess(
                    context,
                    l10n.languageChanged,
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
    SupportedLocale currentLocale,
  ) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      padding:
          padding ??
          const EdgeInsets.symmetric(
            horizontal: AppMetrics.p12,
            vertical: AppMetrics.p8,
          ),
      decoration: BoxDecoration(
        color: scheme.surfaceContainerHigh,
        border: Border.all(color: scheme.outlineVariant),
        borderRadius: BorderRadius.circular(AppShape.r8),
      ),
      child: DropdownButtonHideUnderline(
        child: DropdownButton<SupportedLocale>(
          value: currentLocale,
          isDense: true,
          icon: Icon(
            Icons.keyboard_arrow_down,
            color: scheme.onSurfaceVariant,
            size: AppIconSize.inlineGlyph,
          ),
          items: SupportedLocale.values.map((locale) {
            return DropdownMenuItem<SupportedLocale>(
              value: locale,
              child: Text(
                '${locale.flagEmoji} ${locale.shortName}',
                style: Theme.of(context).textTheme.bodyLarge?.copyWith(
                  color: scheme.onSurface,
                  fontWeight: FontWeight.w500,
                ),
              ),
            );
          }).toList(),
          onChanged: (SupportedLocale? newLocale) {
            if (newLocale != null && newLocale != currentLocale) {
              ref
                  .read(localizationControllerProvider.notifier)
                  .setLocale(newLocale);
            }
          },
        ),
      ),
    );
  }
}

/// Language Selector untuk Settings page
class LanguageSelectorTile extends ConsumerWidget {
  final EdgeInsets? contentPadding;

  const LanguageSelectorTile({super.key, this.contentPadding});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final scheme = Theme.of(context).colorScheme;
    final l10n = AppLocalizations.of(context)!;
    final localizationState = ref.watch(localizationControllerProvider);
    final currentLocale = localizationState.currentLocale;

    return ListTile(
      leading: Icon(
        Icons.language,
        color: scheme.onSurfaceVariant,
        size: AppIconSize.header,
      ),
      title: Text(
        l10n.language,
        style: context.typeRoles.titleCompact.copyWith(
          color: scheme.onSurface,
          fontWeight: FontWeight.w500,
        ),
      ),
      subtitle: Text(
        '${currentLocale.flagEmoji} ${currentLocale.displayName}',
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
      onTap: () => _showLanguageBottomSheet(context, ref),
    );
  }

  void _showLanguageBottomSheet(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final currentLocale = ref
        .read(localizationControllerProvider)
        .currentLocale;

    // Canonical selection builder: surface, shape, handle, scroll and safe area
    // all come from the foundation — no bespoke sheet.
    AppBottomSheetListSelection.showListSelection<SupportedLocale>(
      context: context,
      title: l10n.language,
      selectedValue: currentLocale,
      items: SupportedLocale.values
          .map(
            (locale) => ListSelectionItem<SupportedLocale>(
              title: '${locale.flagEmoji}  ${locale.displayName}',
              value: locale,
            ),
          )
          .toList(),
    ).then((locale) {
      if (locale == null || locale == currentLocale || !context.mounted) return;
      ref.read(localizationControllerProvider.notifier).setLocale(locale);
      AppSnackBar.showSuccess(
        context,
        l10n.languageChanged,
        duration: const Duration(seconds: 2),
      );
    });
  }
}
