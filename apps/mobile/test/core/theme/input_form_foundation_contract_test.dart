// INPUT / FORM FOUNDATION — convergence contract (Pass 2, owner-locked).
//
// Positive proof: the theme owns every ordinary field state (normal/focused/
// disabled/error/focused-error + label/helper/error/icon), the search factory
// is one flat filled borderless spec, and selection controls resolve their
// selected/disabled inks from the theme.
//
// Negative proof: the competing producers the audit found are gone — no second
// generic field wrapper, no second password producer, no local editable-search
// decoration, no local selection colours, no duplicate username or
// confirm-password rule, no vestigial validation model.
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/widgets/app_text_field.dart';

import '../../support/theme_authority_gate.dart';

const _authPasswordField =
    'lib/domains/user/identity/authentication/presentation/shared/widgets/auth_password_field.dart';
const _authFormState =
    'lib/domains/user/identity/authentication/presentation/shared/models/auth_form_state.dart';
const _signUp =
    'lib/domains/user/identity/authentication/presentation/screens/sign_up_screen.dart';
const _signIn =
    'lib/domains/user/identity/authentication/presentation/screens/sign_in_screen.dart';
const _security =
    'lib/domains/user/profile/presentation/screens/security_screen.dart';
const _sellerUpgrade =
    'lib/domains/user/preference/seller/presentation/screens/seller_upgrade_wizard_screen.dart';

/// Every ordinary flat editable search field must consume the one factory.
const _searchConsumers = <String>[
  'lib/features/search/search/presentation/widgets/global_search_bar.dart',
  'lib/shared/widgets/app_bottom_sheet_list_selection.dart',
  'lib/domains/social/follow/presentation/screens/follow_list_screen.dart',
  'lib/domains/chat/chat/presentation/screens/new_chat_screen.dart',
  'lib/shared/widgets/user_search_bottom_sheet.dart',
  'lib/domains/commerce/catalog/for_sale/presentation/screens/for_sale_list_screen.dart',
  'lib/domains/chat/chat/presentation/screens/chat_list_screen.dart',
  'lib/domains/system/support/presentation/screens/help_center_screen.dart',
];

/// Deliberate non-consumers: a map overlay, a tappable navigation pill, the
/// composer, and domain pickers are NOT flat editable search fields.
const _searchExceptions = <String>[
  'lib/shared/widgets/map_picker/map_picker_widgets.dart',
  'lib/features/home/presentation/widgets/main_app_bar.dart',
];

String _read(String path) => File(path).readAsStringSync();

void main() {
  group('form-field state authority (theme)', () {
    for (final mode in <String, ThemeData>{
      'light': AppTheme.lightTheme,
      'dark': AppTheme.darkTheme,
    }.entries) {
      test('${mode.key}: error / focused-error / disabled borders are owned', () {
        final scheme = mode.value.colorScheme;
        final d = mode.value.inputDecorationTheme;

        final errorBorder = d.errorBorder! as OutlineInputBorder;
        expect(errorBorder.borderSide.color, scheme.error);
        expect(errorBorder.borderRadius, AppShape.containerRadius);

        final focusedError = d.focusedErrorBorder! as OutlineInputBorder;
        expect(focusedError.borderSide.color, scheme.error);
        expect(focusedError.borderSide.width, AppMetrics.focusedBorderWidth);

        final disabled = d.disabledBorder! as OutlineInputBorder;
        expect(
          disabled.borderSide.color,
          scheme.onSurface.withValues(alpha: 0.12),
        );
      });

      test('${mode.key}: label / helper / error / icon states are owned', () {
        final scheme = mode.value.colorScheme;
        final d = mode.value.inputDecorationTheme;
        expect(d.labelStyle?.color, scheme.onSurfaceVariant);
        expect(d.helperStyle?.color, scheme.onSurfaceVariant);
        expect(d.errorStyle?.color, scheme.error);
        expect(d.prefixIconColor, scheme.onSurfaceVariant);
        expect(d.suffixIconColor, scheme.onSurfaceVariant);
      });

      test('${mode.key}: selection controls resolve their inks from theme', () {
        final scheme = mode.value.colorScheme;
        final theme = mode.value;
        expect(
          theme.checkboxTheme.fillColor?.resolve({WidgetState.selected}),
          scheme.primary,
        );
        expect(
          theme.checkboxTheme.fillColor?.resolve({WidgetState.disabled}),
          scheme.onSurface.withValues(alpha: 0.38),
        );
        expect(
          theme.radioTheme.fillColor?.resolve({WidgetState.selected}),
          scheme.primary,
        );
        expect(
          theme.switchTheme.trackColor?.resolve({WidgetState.selected}),
          scheme.primary,
        );
        expect(
          theme.switchTheme.thumbColor?.resolve({WidgetState.selected}),
          scheme.onPrimary,
        );
      });

      test('${mode.key}: searchDecoration is one filled borderless spec', () {
        final scheme = mode.value.colorScheme;
        final d = AppTheme.searchDecoration(scheme, hintText: 'x');
        expect(d.filled, isTrue);
        expect(d.fillColor, scheme.surfaceContainerHigh);
        final border = d.border! as OutlineInputBorder;
        expect(border.borderSide, BorderSide.none);
        expect(
          border.borderRadius,
          const BorderRadius.all(Radius.circular(AppShape.r12)),
        );
        expect(d.contentPadding, AppMetrics.inputPadding);
      });
    }
  });

  group('positive widget proof: field states', () {
    testWidgets('readOnly shows the value without the disabled look', (
      tester,
    ) async {
      final controller = TextEditingController(text: 'immutable');
      addTearDown(controller.dispose);
      await tester.pumpWidget(
        MaterialApp(
          theme: AppTheme.lightTheme,
          home: Scaffold(
            body: AppTextField(
              controller: controller,
              labelText: 'Username',
              readOnly: true,
            ),
          ),
        ),
      );
      final field = tester.widget<TextField>(find.byType(TextField));
      expect(field.readOnly, isTrue);
      expect(field.enabled, isTrue, reason: 'read-only is not disabled');
    });

    testWidgets('disabled field reports enabled:false', (tester) async {
      await tester.pumpWidget(
        MaterialApp(
          theme: AppTheme.lightTheme,
          home: const Scaffold(
            body: AppTextField(labelText: 'Locked', enabled: false),
          ),
        ),
      );
      final field = tester.widget<TextField>(find.byType(TextField));
      expect(field.enabled, isFalse);
    });

    testWidgets('server error rides the canonical error slot', (tester) async {
      await tester.pumpWidget(
        MaterialApp(
          theme: AppTheme.lightTheme,
          home: const Scaffold(
            body: AppTextField(labelText: 'Email', errorText: 'sudah terdaftar'),
          ),
        ),
      );
      expect(find.text('sudah terdaftar'), findsOneWidget);
    });
  });

  group('negative proof: one generic field producer', () {
    test('no second generic wrapper class and no AuthTextField file', () {
      final offenders = <String>[];
      for (final path in themeAuthorityDartFiles()) {
        if (_read(path).contains('class AuthTextField')) offenders.add(path);
      }
      expect(offenders, isEmpty, reason: offenders.join('\n'));
      expect(
        File(
          'lib/domains/user/identity/authentication/presentation/shared/widgets/auth_text_field.dart',
        ).existsSync(),
        isFalse,
      );
    });

    test('no AppTextField.password / isPassword path survives lib-wide', () {
      // `isPasswordVisible` is the auth password field's visibility flag and is
      // legitimate; the purged path is the bare `isPassword` named parameter
      // that AppTextField used to own.
      final purged = RegExp(r'isPassword(?!Visible)');
      final offenders = <String>[];
      for (final path in themeAuthorityDartFiles()) {
        final code = _read(path)
            .split('\n')
            .where((l) => !l.trimLeft().startsWith('//'))
            .join('\n');
        if (code.contains('AppTextField.password') || purged.hasMatch(code)) {
          offenders.add(path);
        }
      }
      expect(offenders, isEmpty, reason: offenders.join('\n'));
    });
  });

  group('negative proof: password authority', () {
    test('one password producer, canonical Indonesian toggle semantics', () {
      final src = _read(_authPasswordField);
      expect(src.contains('Tampilkan kata sandi'), isTrue);
      expect(src.contains('Sembunyikan kata sandi'), isTrue);
      expect(src.contains('Hide password'), isFalse);
      expect(src.contains('Show password'), isFalse);
    });
  });

  group('negative proof: search visual authority', () {
    test('every flat editable search field consumes the factory', () {
      for (final path in _searchConsumers) {
        expect(
          _read(path).contains('searchDecoration('),
          isTrue,
          reason: '$path must consume AppTheme.searchDecoration',
        );
      }
    });

    test('exceptions do not consume the search factory', () {
      for (final path in _searchExceptions) {
        expect(
          _read(path).contains('searchDecoration('),
          isFalse,
          reason: '$path is not a flat editable search field',
        );
      }
    });

    test('the dead zero-consumer UserSearchBar is gone', () {
      expect(
        File(
          'lib/domains/social/follow/presentation/widgets/user_search_bar.dart',
        ).existsSync(),
        isFalse,
      );
      expect(
        File('lib/domains/social/follow/follow.dart')
            .readAsStringSync()
            .contains('user_search_bar'),
        isFalse,
      );
    });
  });

  group('negative proof: selection colours are theme-owned', () {
    test('no selection-control block restates a theme-owned colour', () {
      // Selection controls only: a Slider handle or a custom action button's
      // `activeColor` is a different concern and stays out of this rule.
      const ctors = <String>[
        'Checkbox(',
        'Switch(',
        'Radio(',
        'CheckboxListTile(',
        'SwitchListTile(',
        'RadioListTile(',
      ];
      final restated = RegExp(
        r'(active|inactive)(Thumb|Track)?Color:\s*(scheme|Theme\.of)',
      );
      final offenders = <String>[];
      for (final path in themeAuthorityDartFiles()) {
        if (path == 'lib/core/src/theme/app_theme.dart') continue;
        final src = _read(path);
        for (final ctor in ctors) {
          var from = 0;
          while (true) {
            final idx = src.indexOf(ctor, from);
            if (idx < 0) break;
            var depth = 0;
            var end = idx + ctor.length - 1;
            for (; end < src.length; end++) {
              final ch = src[end];
              if (ch == '(') depth++;
              if (ch == ')') {
                depth--;
                if (depth == 0) break;
              }
            }
            final block = src.substring(idx, end + 1);
            if (restated.hasMatch(block)) {
              final line = '\n'.allMatches(src.substring(0, idx)).length + 1;
              offenders.add('$path:$line');
            }
            from = idx + 1;
          }
        }
      }
      expect(offenders, isEmpty, reason: offenders.join('\n'));
    });

    test('the coin switch keeps its legitimate domain accent', () {
      expect(
        _read(
          'lib/domains/commerce/transaction/checkout/presentation/widgets/checkout_coin_section.dart',
        ).contains('activeTrackColor: AppColors.coinPrimary'),
        isTrue,
      );
    });
  });

  group('negative proof: canonical validation', () {
    test('seller upgrade username uses CanonicalUsernameValidator', () {
      expect(
        _read(_sellerUpgrade).contains(
          'CanonicalUsernameValidator.normalizeAndValidate',
        ),
        isTrue,
      );
    });

    test('one confirm-password rule for every consumer', () {
      expect(_read(_signUp).contains('CanonicalPasswordMatch'), isTrue);
      expect(_read(_security).contains('CanonicalPasswordMatch'), isTrue);
      expect(
        _read(_authPasswordField).contains('CanonicalPasswordMatch.matches'),
        isTrue,
      );
    });

    test('vestigial form-validation model is purged', () {
      final src = _read(_authFormState);
      expect(src.contains('FormFieldValidation'), isFalse);
      expect(src.contains('FieldValidationStatus'), isFalse);
    });
  });

  group('owner decisions applied', () {
    test('server field errors ride the field error slot', () {
      final src = _read(_signUp);
      expect(src.contains('errorText: _backendEmailError'), isTrue);
      expect(src.contains('errorText: _backendUsernameError'), isTrue);
    });

    test('the whole auth form locks while submitting', () {
      expect(_read(_signIn).contains('enabled: !isAuthLoading'), isTrue);
      expect(_read(_signUp).contains('enabled: !_controller.isLoading'), isTrue);
    });
  });
}
