import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_localizations/flutter_localizations.dart';

import 'core/core.dart';
import 'shared/shared.dart';
import 'generated/app_localizations.dart';
import 'domains/system/notification/notification.dart';

class LabudaApp extends ConsumerWidget {
  const LabudaApp({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    // Watch theme state from provider
    final themeState = ref.watch(themeControllerProvider);
    final themeMode = themeState.themeMode;

    // Watch localization state from provider
    final localizationState = ref.watch(localizationControllerProvider);
    final currentLocale = localizationState.locale;

    // Watch router provider - this is the KEY CHANGE
    // Router rebuilds when auth state changes, triggering redirect re-evaluation
    final routerConfig = ref.watch(goRouterProvider);

    return MaterialApp.router(
      title: 'LABUDA - Social Commerce Koi',
      theme: AppTheme.lightTheme,
      darkTheme: AppTheme.darkTheme,
      themeMode: themeMode,
      routerConfig: routerConfig,
      debugShowCheckedModeBanner: false,

      // Wrap router builder with NavigationScope, KeyboardDismissWrapper, and NotificationInitializer.
      // D2 HARD GATE (design scope v2): no email-verification banner — every
      // authenticated user has already proven a verified email before the
      // exchange, so a persistent "verify your email" surface is dead UI.
      builder: (context, child) {
        return NotificationInitializer(
          child: KeyboardDismissWrapper(
            child: NavigationScope(
              child: child ?? const SizedBox(),
            ),
          ),
        );
      },

      // Localization configuration
      localizationsDelegates: const [
        AppLocalizations.delegate,
        GlobalMaterialLocalizations.delegate,
        GlobalWidgetsLocalizations.delegate,
        GlobalCupertinoLocalizations.delegate,
      ],
      supportedLocales: LocalizationHelper.supportedLocales,
      locale: currentLocale,

      // Locale resolution strategy
      localeResolutionCallback: (locale, supportedLocales) {
        if (locale != null) {
          // Check if the locale is supported
          for (final supportedLocale in supportedLocales) {
            if (supportedLocale.languageCode == locale.languageCode) {
              return supportedLocale;
            }
          }
        }

        // Fallback to Indonesian
        return const Locale('id', 'ID');
      },
    );
  }
}
