import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

/// PASS_18C: locks the create-auction screen's timing UI parity with the
/// owner-approved policy (immediate/scheduled start + 1-7 day duration
/// presets), following the same source-text contract convention already
/// used by create_auction_screen_analytics_contract_test.dart for this
/// screen (no existing widget-pump harness exists to extend).
void main() {
  test(
    'CreateAuctionScreen offers start-mode + duration presets, not free start/end pickers',
    () {
      final source = File(
        'lib/domains/commerce/catalog/auction/presentation/screens/create_auction_screen.dart',
      ).readAsStringSync();

      // Start-mode selector present, defaulting to immediate start.
      expect(source, contains('_AuctionStartMode.now'));
      expect(source, contains('_AuctionStartMode.scheduled'));
      expect(source, contains('SegmentedButton<String>'));

      // Duration presets present (1/3/5/7 days = 24/72/120/168 hours).
      expect(source, contains('_DurationPreset(\'1 hari\', 24)'));
      expect(source, contains('_DurationPreset(\'3 hari\', 72)'));
      expect(source, contains('_DurationPreset(\'5 hari\', 120)'));
      expect(source, contains('_DurationPreset(\'7 hari\', 168)'));

      // The old free end-time picker and unconsumed isScheduled flag must be
      // gone — regression guard against silently reintroducing the gap
      // PASS_18C closed.
      expect(source, isNot(contains('_pickEndTime')));
      expect(source, isNot(contains('_endTime')));
      expect(source, isNot(contains('isScheduled:')));
    },
  );
}
