import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/features/search/search/domain/entities/search_result.dart';
import 'package:labuda/features/search/search/presentation/widgets/search_result_item.dart';

Widget _wrap(SearchResult result) {
  return ProviderScope(
    child: MaterialApp(
      home: Scaffold(body: SearchResultItem(result: result)),
    ),
  );
}

SearchResult _listingResult({required String subtitle}) {
  return SearchResult(
    id: 'l1',
    type: SearchResultType.listing,
    title: 'Showa Koi 30cm',
    subtitle: subtitle,
    metadata: {
      'sellerLifecycle': 'active',
      'sellerTrustLifecycle': 'active',
      'sellerId': 'seller-1',
    },
    createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
  );
}

SearchResult _auctionResult({required String subtitle}) {
  return SearchResult(
    id: 'a1',
    type: SearchResultType.auction,
    title: 'Sanke Auction',
    subtitle: subtitle,
    metadata: {
      'sellerLifecycle': 'active',
      'sellerTrustLifecycle': 'active',
      'sellerId': 'seller-2',
    },
    createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
  );
}

void main() {
  testWidgets('Search listing result renders @username and store_name', (
    tester,
  ) async {
    await tester.pumpWidget(
      _wrap(_listingResult(subtitle: '@yayan\nFarm Koi Nusantara')),
    );

    expect(find.text('Showa Koi 30cm'), findsOneWidget);
    expect(find.text('@yayan\nFarm Koi Nusantara'), findsOneWidget);
  });

  testWidgets('Search auction result renders @username and store_name', (
    tester,
  ) async {
    await tester.pumpWidget(
      _wrap(_auctionResult(subtitle: '@yayan\nFarm Koi Nusantara')),
    );

    expect(find.text('Sanke Auction'), findsOneWidget);
    expect(find.text('@yayan\nFarm Koi Nusantara'), findsOneWidget);
  });

  testWidgets('Store missing fallback renders only @username', (tester) async {
    await tester.pumpWidget(_wrap(_listingResult(subtitle: '@yayan')));

    expect(find.text('@yayan'), findsOneWidget);
    expect(find.text('Farm Koi Nusantara'), findsNothing);
  });
}
