// TIMEFORMATSERVICE — CANONICAL RELATIVE-TIME UNIT GATE.
//
// Owner-locked: TimeFormatService is the ONE canonical relative-time
// formatting engine (Indonesian progression):
//
//   baru saja → N menit lalu → N jam lalu → N hari lalu →
//   N minggu lalu → N bulan lalu → N tahun lalu
//
// Every test passes an explicit `now` reference — nothing here depends on
// the wall clock. Assertions pin exact strings, not merely "does not throw".
import 'package:flutter_test/flutter_test.dart';
import 'package:intl/date_symbol_data_local.dart';
import 'package:labuda/domains/system/shared/domain/services/time_format_service.dart';
import 'package:labuda/shared/utils/app_formatters.dart';

void main() {
  const service = TimeFormatService();
  // Fixed local reference; all inputs derive from it deterministically.
  final now = DateTime(2026, 3, 15, 12, 0, 0);

  group('just now / sub-minute / future', () {
    test('seconds ago is baru saja', () {
      expect(
        service.formatTimeAgo(now.subtract(const Duration(seconds: 30)), now: now),
        'baru saja',
      );
    });

    test('zero difference is baru saja', () {
      expect(service.formatTimeAgo(now, now: now), 'baru saja');
    });

    test('59 seconds is still baru saja', () {
      expect(
        service.formatTimeAgo(now.subtract(const Duration(seconds: 59)), now: now),
        'baru saja',
      );
    });

    test('future timestamps fall back to baru saja', () {
      expect(
        service.formatTimeAgo(now.add(const Duration(minutes: 5)), now: now),
        'baru saja',
      );
      expect(
        service.formatTimeAgo(now.add(const Duration(days: 2)), now: now),
        'baru saja',
      );
    });
  });

  group('minutes', () {
    test('1 minute', () {
      expect(
        service.formatTimeAgo(now.subtract(const Duration(minutes: 1)), now: now),
        '1 menit lalu',
      );
    });

    test('plural minutes share the same shape (no Indonesian plural)', () {
      expect(
        service.formatTimeAgo(now.subtract(const Duration(minutes: 5)), now: now),
        '5 menit lalu',
      );
      expect(
        service.formatTimeAgo(now.subtract(const Duration(minutes: 59)), now: now),
        '59 menit lalu',
      );
    });
  });

  group('hours', () {
    test('60 minutes transitions to 1 hour', () {
      expect(
        service.formatTimeAgo(now.subtract(const Duration(minutes: 60)), now: now),
        '1 jam lalu',
      );
    });

    test('plural hours', () {
      expect(
        service.formatTimeAgo(now.subtract(const Duration(hours: 3)), now: now),
        '3 jam lalu',
      );
      expect(
        service.formatTimeAgo(now.subtract(const Duration(hours: 23)), now: now),
        '23 jam lalu',
      );
    });
  });

  group('days', () {
    test('24 hours transitions to 1 day', () {
      expect(
        service.formatTimeAgo(now.subtract(const Duration(hours: 24)), now: now),
        '1 hari lalu',
      );
    });

    test('plural days up to 6', () {
      expect(
        service.formatTimeAgo(now.subtract(const Duration(days: 2)), now: now),
        '2 hari lalu',
      );
      expect(
        service.formatTimeAgo(
          now.subtract(const Duration(days: 6, hours: 23)),
          now: now,
        ),
        '6 hari lalu',
      );
    });
  });

  group('weeks', () {
    test('7 days transitions to 1 week (no more Month-Day fallback)', () {
      expect(
        service.formatTimeAgo(now.subtract(const Duration(days: 7)), now: now),
        '1 minggu lalu',
      );
    });

    test('plural weeks truncate down', () {
      expect(
        service.formatTimeAgo(now.subtract(const Duration(days: 14)), now: now),
        '2 minggu lalu',
      );
      expect(
        service.formatTimeAgo(now.subtract(const Duration(days: 29)), now: now),
        '4 minggu lalu',
      );
    });
  });

  group('months', () {
    test('30 days transitions to 1 month', () {
      expect(
        service.formatTimeAgo(now.subtract(const Duration(days: 30)), now: now),
        '1 bulan lalu',
      );
    });

    test('plural months truncate down', () {
      expect(
        service.formatTimeAgo(now.subtract(const Duration(days: 90)), now: now),
        '3 bulan lalu',
      );
      expect(
        service.formatTimeAgo(now.subtract(const Duration(days: 364)), now: now),
        '12 bulan lalu',
      );
    });
  });

  group('years', () {
    test('365 days transitions to 1 year', () {
      expect(
        service.formatTimeAgo(now.subtract(const Duration(days: 365)), now: now),
        '1 tahun lalu',
      );
    });

    test('plural years truncate down', () {
      expect(
        service.formatTimeAgo(now.subtract(const Duration(days: 730)), now: now),
        '2 tahun lalu',
      );
    });
  });

  group('local/UTC conversion semantics', () {
    test('mixed UTC input resolves against a local reference correctly', () {
      // now.toUtc() minus 3h, compared back against the local `now`:
      // difference() compares absolute instants, so the zone offset
      // cancels out deterministically on any machine timezone.
      final utcInput = now.toUtc().subtract(const Duration(hours: 3));
      expect(service.formatTimeAgo(utcInput, now: now), '3 jam lalu');
    });

    test('UTC day boundary input', () {
      final utcInput = now.toUtc().subtract(const Duration(days: 9));
      expect(service.formatTimeAgo(utcInput, now: now), '1 minggu lalu');
    });
  });

  group('showFullDate (existing opt-in absolute behavior)', () {
    test('returns the full calendar date', () {
      expect(
        service.formatTimeAgo(DateTime(2024, 1, 5), showFullDate: true),
        'Jan 5, 2024',
      );
    });
  });

  group('G-S proof — AppFormatters.formatDateTime local semantics', () {
    setUpAll(() async {
      await initializeDateFormatting('id_ID');
    });

    test('fixed local datetime formats canonically', () {
      expect(
        AppFormatters.formatDateTime(DateTime(2025, 11, 20, 14, 45)),
        '20 Nov 2025, 14:45',
      );
    });
  });
}
