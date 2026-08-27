import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  test(
    'CreateAuctionScreen delegates creation through auction notifier and avoids direct analytics coupling',
    () {
      final source = File(
        'lib/domains/commerce/catalog/auction/presentation/screens/create_auction_screen.dart',
      ).readAsStringSync();

      expect(source, contains('auctionNotifierProvider'));
      expect(source, contains('.createAuction('));
      expect(source, isNot(contains('coreAnalyticsRepositoryProvider')));
      expect(source, isNot(contains('analyticsTrackerProvider')));
      expect(source, isNot(contains("'content_created'")));
      expect(
        source,
        isNot(contains('domains/system/analytics/analytics.dart')),
      );
    },
  );
}
