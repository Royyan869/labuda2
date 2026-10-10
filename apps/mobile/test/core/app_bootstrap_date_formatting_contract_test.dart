// APPLICATION BOOTSTRAP — DATE-FORMATTING INITIALIZATION CONTRACT.
//
// Canonical ownership: `initializeAppDateFormatting()` in lib/main.dart is
// the ONE production initializer for the intl date symbols required by
// AppFormatters. Without it, every AppFormatters date path throws
// LocaleDataException (proven by chat suites that had to self-initialize).
//
// This gate proves the bootstrap contract, not intl itself: it invokes the
// production startup step (never intl directly) and then asserts the
// dependent production formatting behavior end to end.
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/main.dart' show initializeAppDateFormatting;
import 'package:hishumi/shared/utils/app_formatters.dart';

void main() {
  group('bootstrap date-formatting contract', () {
    test(
      'canonical startup step establishes symbols before AppFormatters executes',
      () async {
        await initializeAppDateFormatting();

        expect(
          AppFormatters.formatDate(DateTime(2024, 1, 15)),
          '15 Jan 2024',
        );
        expect(
          AppFormatters.formatDateTime(DateTime(2024, 1, 15, 14, 30)),
          '15 Jan 2024, 14:30',
        );
        expect(
          AppFormatters.formatShortDate(DateTime(2024, 1, 15)),
          '15/01/2024',
        );
        expect(
          AppFormatters.formatTime(DateTime(2024, 1, 15, 14, 30)),
          '14:30',
        );
      },
    );

    test('startup step is idempotent', () async {
      await initializeAppDateFormatting();
      await initializeAppDateFormatting();

      expect(
        AppFormatters.formatDate(DateTime(2024, 1, 15)),
        '15 Jan 2024',
      );
    });
  });
}
