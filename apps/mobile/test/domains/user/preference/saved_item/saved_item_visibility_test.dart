import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/domains/user/preference/saved_item/data/repositories/saved_item_repository.dart';
import 'package:hishumi/domains/user/preference/saved_item/data/repositories/saved_item_repository_provider.dart';
import 'package:hishumi/domains/user/preference/saved_item/models/saved_item_model.dart';
import 'package:hishumi/domains/user/preference/saved_item/screens/saved_item_screen.dart';
import 'package:hishumi/generated/app_localizations.dart';

SavedItemModel _model({
  required TargetType targetType,
  required String targetId,
  String? forSaleStatus,
  String? auctionStatus,
}) {
  return SavedItemModel(
    id: '$targetId-saved',
    userId: 'buyer-1',
    targetType: targetType,
    targetId: targetId,
    intentType: targetType == TargetType.forSale
        ? IntentType.bookmark
        : IntentType.watch,
    createdAt: DateTime.utc(2026, 8, 2),
    forSaleTitle: targetType == TargetType.forSale ? 'Saved ForSale' : null,
    forSalePrice: targetType == TargetType.forSale ? 100000 : null,
    forSaleStatus: forSaleStatus,
    auctionTitle: targetType == TargetType.auction ? 'Saved Auction' : null,
    startPrice: targetType == TargetType.auction ? 250000 : null,
    auctionStatus: auctionStatus,
  );
}

class _MemorySavedItemRepository extends SavedItemRepository {
  _MemorySavedItemRepository({List<SavedItemModel>? initialItems})
    : super(dio: Dio(BaseOptions(baseUrl: 'http://localhost'))) {
    _items.addAll(initialItems ?? const []);
  }

  final List<SavedItemModel> _items = <SavedItemModel>[];

  @override
  Future<List<SavedItemModel>> getSavedItems({String? type}) async {
    return List<SavedItemModel>.unmodifiable(_items);
  }

  @override
  Future<void> removeSavedItem({
    required String targetType,
    required String targetId,
  }) async {
    _items.removeWhere((item) => item.targetId == targetId);
  }

  @override
  Future<int> getSavedItemsCount({String? type}) async => _items.length;
}

Widget _wrap(SavedItemRepository repository) {
  return ProviderScope(
    overrides: [
      savedItemRepositoryProvider.overrideWithValue(repository),
    ],
    child: const MaterialApp(
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      locale: Locale('id'),
      home: Scaffold(body: SavedItemScreen()),
    ),
  );
}

void main() {
  test('SavedItemModel has no terminal Saved presentation state', () {
    final model = _model(
      targetType: TargetType.forSale,
      targetId: 'fs-1',
      forSaleStatus: 'active',
    );
    expect(model.toJson(), isNot(contains('terminal_label')));
  });

  testWidgets('Saved projects only canonical active items', (tester) async {
    await tester.pumpWidget(
      _wrap(
        _MemorySavedItemRepository(
          initialItems: [
            _model(
              targetType: TargetType.forSale,
              targetId: 'fs-active',
              forSaleStatus: 'active',
            ),
          ],
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Saved ForSale'), findsOneWidget);
    expect(find.byIcon(Icons.info_outline), findsNothing);
  });

  testWidgets('Saved does not render terminal labels or grace explainer', (
    tester,
  ) async {
    await tester.pumpWidget(
      _wrap(
        _MemorySavedItemRepository(
          initialItems: [
            _model(
              targetType: TargetType.forSale,
              targetId: 'fs-sold',
              forSaleStatus: 'sold',
            ),
            _model(
              targetType: TargetType.auction,
              targetId: 'au-ended',
              auctionStatus: 'ended',
            ),
          ],
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Terjual'), findsNothing);
    expect(find.text('Lelang Berakhir'), findsNothing);
    expect(find.textContaining('24 jam'), findsNothing);
  });

  testWidgets('valid empty result remains distinct from loading/error', (
    tester,
  ) async {
    await tester.pumpWidget(_wrap(_MemorySavedItemRepository()));
    await tester.pumpAndSettle();

    expect(find.byType(CircularProgressIndicator), findsNothing);
    expect(find.text('Disimpan'), findsOneWidget);
  });
}
