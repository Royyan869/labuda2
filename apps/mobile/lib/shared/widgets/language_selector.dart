import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
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
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final l10n = AppLocalizations.of(context)!;
    final localizationState = ref.watch(localizationControllerProvider);
    final currentLocale = localizationState.currentLocale;

    if (isCompact) {
      return _buildCompactSelector(context, ref, isDark, currentLocale);
    }

    return _buildFullSelector(context, ref, isDark, l10n, currentLocale);
  }

  Widget _buildFullSelector(
    BuildContext context,
    WidgetRef ref,
    bool isDark,
    AppLocalizations l10n,
    SupportedLocale currentLocale,
  ) {
    return Padding(
      padding: padding ?? EdgeInsets.zero,
      child: ListTile(
        leading: showLeadingIcon
            ? Icon(
                Icons.language,
                color: isDark
                    ? AppColors.neutralGray300
                    : AppColors.neutralGray600,
              )
            : null,
        title: Text(
          l10n.language,
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
            child: DropdownButton<SupportedLocale>(
              value: currentLocale,
              isDense: true,
              icon: Icon(
                Icons.keyboard_arrow_down,
                color: isDark
                    ? AppColors.neutralGray400
                    : AppColors.neutralGray600,
                size: 16,
              ),
              items: SupportedLocale.values.map((locale) {
                return DropdownMenuItem<SupportedLocale>(
                  value: locale,
                  child: Text(
                    '${locale.flagEmoji} ${locale.displayName}',
                    style: TextStyle(
                      color: isDark
                          ? AppColors.neutralGray200
                          : AppColors.neutralGray800,
                      fontSize: 14,
                    ),
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
    bool isDark,
    SupportedLocale currentLocale,
  ) {
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
        child: DropdownButton<SupportedLocale>(
          value: currentLocale,
          isDense: true,
          icon: Icon(
            Icons.keyboard_arrow_down,
            color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray600,
            size: 16,
          ),
          items: SupportedLocale.values.map((locale) {
            return DropdownMenuItem<SupportedLocale>(
              value: locale,
              child: Text(
                '${locale.flagEmoji} ${locale.shortName}',
                style: TextStyle(
                  color: isDark
                      ? AppColors.neutralGray200
                      : AppColors.neutralGray800,
                  fontSize: 14,
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
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final l10n = AppLocalizations.of(context)!;
    final localizationState = ref.watch(localizationControllerProvider);
    final currentLocale = localizationState.currentLocale;

    return ListTile(
      leading: Icon(
        Icons.language,
        color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray600,
        size: 24,
      ),
      title: Text(
        l10n.language,
        style: TextStyle(
          color: isDark ? AppColors.neutralGray200 : AppColors.neutralGray900,
          fontSize: 16,
          fontWeight: FontWeight.w500,
        ),
      ),
      subtitle: Text(
        '${currentLocale.flagEmoji} ${currentLocale.displayName}',
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
      onTap: () => _showLanguageBottomSheet(context, ref),
    );
  }

  void _showLanguageBottomSheet(BuildContext context, WidgetRef ref) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final l10n = AppLocalizations.of(context)!;
    final currentLocale = ref
        .read(localizationControllerProvider)
        .currentLocale;

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
                l10n.language,
                style: TextStyle(
                  color: isDark
                      ? AppColors.neutralGray200
                      : AppColors.neutralGray900,
                  fontSize: 20,
                  fontWeight: FontWeight.bold,
                ),
              ),
              const SizedBox(height: 20),

              // Language options
              ...SupportedLocale.values.map((locale) {
                final isSelected = locale == currentLocale;
                return ListTile(
                  contentPadding: EdgeInsets.zero,
                  leading: Text(
                    locale.flagEmoji,
                    style: const TextStyle(fontSize: 24),
                  ),
                  title: Text(
                    locale.displayName,
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
                  trailing: isSelected
                      ? Icon(
                          Icons.check_circle,
                          color: AppColors.primaryRed,
                          size: 20,
                        )
                      : null,
                  onTap: () {
                    if (locale != currentLocale) {
                      ref
                          .read(localizationControllerProvider.notifier)
                          .setLocale(locale);

                      // Show success message
                      AppSnackBar.showSuccess(
                        context,
                        l10n.languageChanged,
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
