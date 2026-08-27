import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  final files = <String>[
    'lib/features/home/presentation/providers/feed_renderers.dart',
    'lib/domains/user/profile/presentation/widgets/profile_feed_tab.dart',
    'lib/domains/social/content/presentation/screens/content_detail_screen.dart',
    'lib/features/search/search/data/search_repository_impl.dart',
    'lib/features/search/search/data/mappers/search_mapper.dart',
    'lib/features/search/search/data/dto/search_dto.dart',
    'lib/shared/widgets/popup_more_options_button.dart',
  ];

  test('feed/profile/detail/search no longer branch on post/request kinds', () {
    const forbiddenTokens = [
      'FeedItemType.post',
      'FeedItemType.request',
      'PopupMoreOptionsContentType.post',
      'PopupMoreOptionsContentType.request',
      "contentType: 'post'",
      "contentType: 'request'",
    ];

    for (final relativePath in files) {
      final source = File(relativePath).readAsStringSync();
      for (final token in forbiddenTokens) {
        expect(
          source,
          isNot(contains(token)),
          reason: '$relativePath still contains $token',
        );
      }
    }
  });
}
