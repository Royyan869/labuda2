import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/shared/presentation/widgets/commerce_saved_item_action_button.dart';
import 'package:labuda/domains/user/preference/saved_item/data/repositories/saved_item_repository.dart';
import 'package:labuda/domains/user/preference/saved_item/data/repositories/saved_item_repository_provider.dart';
import 'package:labuda/domains/user/preference/saved_item/models/saved_item_model.dart';
import 'package:labuda/domains/user/preference/saved_item/screens/saved_item_screen.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';

class _FakeAuthController extends AuthController {
  _FakeAuthController(this._state);

  final AuthState _state;

  @override
  AuthState build() => _state;
}

AuthUser _authUser({required String id}) {
  final now = DateTime.utc(2026, 8, 2);
  return AuthUser(
    id: id,
    createdAt: now,
    updatedAt: now,
    email: '$id@example.com',
    username: id,
    isEmailVerified: true,
    roles: const [UserRole.user],
    provider: AuthProvider.email,
    lifecycle: ContentLifecycle.active,
  );
}

SavedItemModel _forSaleItem({
  required String id,
  String title = 'Saved Listing',
}) {
  return SavedItemModel(
    id: '$id-saved',
    userId: 'buyer-1',
    targetType: TargetType.forSale,
    targetId: id,
    intentType: IntentType.bookmark,
    sellerId: 'seller-1',
    createdAt: DateTime.utc(2026, 8, 2),
    forSaleTitle: title,
    forSalePrice: 1250000,
  );
}

SavedItemModel _auctionItem({
  required String id,
  String title = 'Saved Auction',
}) {
  return SavedItemModel(
    id: '$id-saved',
    userId: 'buyer-1',
    targetType: TargetType.auction,
    targetId: id,
    intentType: IntentType.watch,
    sellerId: 'seller-2',
    createdAt: DateTime.utc(2026, 8, 2),
    auctionTitle: title,
    startPrice: 1500000,
    currentBid: 1750000,
  );
}

/// Canonical wire value for a TargetType (matches backend contract).
String _wireValue(TargetType t) => t == TargetType.forSale ? 'for_sale' : 'auction';

class _MemorySavedItemRepository extends SavedItemRepository {
  _MemorySavedItemRepository({
    List<SavedItemModel>? initialItems,
    this.failOnAdd = false,
  }) : super(dio: Dio(BaseOptions(baseUrl: 'http://localhost'))) {
    _items.addAll(initialItems ?? const []);
  }

  final bool failOnAdd;

  final List<SavedItemModel> _items = <SavedItemModel>[];

  @override
  Future<List<SavedItemModel>> getSavedItems({String? type}) async {
    final items = type == null
        ? _items
        : _items.where((item) => _wireValue(item.targetType) == type).toList();
    return List<SavedItemModel>.unmodifiable(items);
  }

  @override
  Future<SavedItemModel> addSavedItem({
    required String targetType,
    required String targetId,
  }) async {
    if (failOnAdd) {
      throw Exception('save failed');
    }

    final existingIndex = _items.indexWhere(
      (item) => _wireValue(item.targetType) == targetType && item.targetId == targetId,
    );
    if (existingIndex != -1) {
      return _items[existingIndex];
    }

    final item = targetType == 'for_sale'
        ? _forSaleItem(id: targetId)
        : _auctionItem(id: targetId);
    _items.add(item);
    return item;
  }

  @override
  Future<void> removeSavedItem({
    required String targetType,
    required String targetId,
  }) async {
    _items.removeWhere(
      (item) => _wireValue(item.targetType) == targetType && item.targetId == targetId,
    );
  }

  @override
  Future<bool> isSaved({
    required String targetType,
    required String targetId,
  }) async {
    return _items.any(
      (item) => _wireValue(item.targetType) == targetType && item.targetId == targetId,
    );
  }

  @override
  Future<int> getSavedItemsCount({String? type}) async {
    final items = await getSavedItems(type: type);
    return items.length;
  }

  @override
  Future<void> clearSavedItems({String? type}) async {
    if (type == null) {
      _items.clear();
    } else {
      _items.removeWhere((item) => _wireValue(item.targetType) == type);
    }
  }
}

Widget _wrap({
  required Widget child,
  AuthState? authState,
  SavedItemRepository? repository,
}) {
  return ProviderScope(
    overrides: [
      if (authState != null)
        authControllerProvider.overrideWith(() => _FakeAuthController(authState)),
      if (repository != null)
        savedItemRepositoryProvider.overrideWithValue(repository),
    ],
    child: MaterialApp(home: Scaffold(body: child)),
  );
}

void main() {
  group('SavedItemScreen', () {
    testWidgets('empty state shows placeholder', (tester) async {
      final repository = _MemorySavedItemRepository();

      await tester.pumpWidget(
        _wrap(
          repository: repository,
          authState: AuthState.authenticated(
            _authUser(id: 'buyer-empty'),
            emailVerified: true,
          ),
          child: const SavedItemScreen(),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('Belum ada item yang disimpan'), findsOneWidget);
    });

    testWidgets('shows saved items', (tester) async {
      final repository = _MemorySavedItemRepository(
        initialItems: [
          _forSaleItem(id: 'for-sale-1', title: 'For Sale Item'),
          _auctionItem(id: 'auction-1', title: 'Auction Item'),
        ],
      );

      await tester.pumpWidget(
        _wrap(
          repository: repository,
          authState: AuthState.authenticated(
            _authUser(id: 'buyer-show'),
            emailVerified: true,
          ),
          child: const SavedItemScreen(),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('For Sale Item'), findsOneWidget);
      expect(find.text('Auction Item'), findsOneWidget);
    });

    testWidgets('for-sale save icon toggles and refreshes saved page', (
      tester,
    ) async {
      final repository = _MemorySavedItemRepository();

      await tester.pumpWidget(
        _wrap(
          repository: repository,
          authState: AuthState.authenticated(
            _authUser(id: 'buyer-listing'),
            emailVerified: true,
          ),
          child: Column(
            children: [
              Expanded(
                child: SavedItemScreen(key: const ValueKey('saved-page')),
              ),
              const SizedBox(height: 12),
              const CommerceSavedItemActionButton(
                targetType: 'for_sale',
                targetId: 'for-sale-1',
                label: 'Simpan',
                activeLabel: 'Tersimpan',
                icon: Icons.bookmark_border_outlined,
                activeIcon: Icons.bookmark,
              ),
            ],
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('Belum ada item yang disimpan'), findsOneWidget);
      expect(find.text('Simpan'), findsNothing);
      expect(find.byIcon(Icons.bookmark_border_outlined), findsOneWidget);

      await tester.tap(find.byIcon(Icons.bookmark_border_outlined));
      await tester.pumpAndSettle();

      expect(find.text('Saved Listing'), findsOneWidget);
      expect(find.text('Belum ada item yang disimpan'), findsNothing);
      expect(find.byIcon(Icons.bookmark), findsOneWidget);

      await tester.tap(find.byIcon(Icons.bookmark));
      await tester.pumpAndSettle();

      expect(find.text('Saved Listing'), findsNothing);
      expect(find.text('Belum ada item yang disimpan'), findsOneWidget);
      expect(find.byIcon(Icons.bookmark_border_outlined), findsOneWidget);
    });

    testWidgets('auction watch refreshes the saved page after success', (
      tester,
    ) async {
      final repository = _MemorySavedItemRepository();

      await tester.pumpWidget(
        _wrap(
          repository: repository,
          authState: AuthState.authenticated(
            _authUser(id: 'buyer-auction'),
            emailVerified: true,
          ),
          child: Column(
            children: [
              Expanded(
                child: SavedItemScreen(key: const ValueKey('saved-page')),
              ),
              const SizedBox(height: 12),
              const CommerceSavedItemActionButton(
                targetType: 'auction',
                targetId: 'auction-1',
                label: 'Pantau',
                activeLabel: 'Dipantau',
                icon: Icons.visibility_outlined,
                activeIcon: Icons.visibility,
              ),
            ],
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('Belum ada item yang disimpan'), findsOneWidget);
      expect(find.byIcon(Icons.visibility_outlined), findsOneWidget);

      await tester.tap(find.byIcon(Icons.visibility_outlined));
      await tester.pumpAndSettle();

      expect(find.text('Saved Auction'), findsOneWidget);
      expect(find.byIcon(Icons.visibility), findsOneWidget);

      await tester.tap(find.byIcon(Icons.visibility));
      await tester.pumpAndSettle();

      expect(find.text('Saved Auction'), findsNothing);
      expect(find.text('Belum ada item yang disimpan'), findsOneWidget);
      expect(find.byIcon(Icons.visibility_outlined), findsOneWidget);
    });

    testWidgets(
      'failed mutation keeps the previous icon state and page empty',
      (tester) async {
        final repository = _MemorySavedItemRepository(failOnAdd: true);

        await tester.pumpWidget(
          _wrap(
            repository: repository,
            authState: AuthState.authenticated(
              _authUser(id: 'buyer-fail'),
              emailVerified: true,
            ),
            child: Column(
              children: [
                Expanded(
                  child: SavedItemScreen(key: const ValueKey('saved-page')),
                ),
                const SizedBox(height: 12),
                const CommerceSavedItemActionButton(
                  targetType: 'for_sale',
                  targetId: 'for-sale-fail',
                  label: 'Simpan',
                  activeLabel: 'Tersimpan',
                  icon: Icons.bookmark_border_outlined,
                  activeIcon: Icons.bookmark,
                ),
              ],
            ),
          ),
        );
        await tester.pumpAndSettle();

        expect(find.text('Belum ada item yang disimpan'), findsOneWidget);
        expect(find.byIcon(Icons.bookmark_border_outlined), findsOneWidget);

        await tester.tap(find.byIcon(Icons.bookmark_border_outlined));
        await tester.pumpAndSettle();

        expect(find.text('Belum ada item yang disimpan'), findsOneWidget);
        expect(find.byIcon(Icons.bookmark_border_outlined), findsOneWidget);
        expect(find.byIcon(Icons.bookmark), findsNothing);
        expect(find.text('Tersimpan'), findsNothing);
        expect(await repository.getSavedItemsCount(), 0);
      },
    );

    testWidgets('saved page keeps for-sale and auction records distinct', (
      tester,
    ) async {
      final repository = _MemorySavedItemRepository(
        initialItems: [
          _forSaleItem(id: 'for-sale-mapped', title: 'For Sale Mapped'),
          _auctionItem(id: 'auction-mapped', title: 'Auction Mapped'),
        ],
      );

      await tester.pumpWidget(
        _wrap(
          repository: repository,
          authState: AuthState.authenticated(
            _authUser(id: 'buyer-map'),
            emailVerified: true,
          ),
          child: const SavedItemScreen(),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('For Sale Mapped'), findsOneWidget);
      expect(find.text('Auction Mapped'), findsOneWidget);

      await tester.tap(find.byType(PopupMenuButton<String?>));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Listing').last);
      await tester.pumpAndSettle();
      expect(find.text('For Sale Mapped'), findsOneWidget);
      expect(find.text('Auction Mapped'), findsNothing);

      await tester.tap(find.byType(PopupMenuButton<String?>));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Auction').last);
      await tester.pumpAndSettle();
      expect(find.text('For Sale Mapped'), findsNothing);
      expect(find.text('Auction Mapped'), findsOneWidget);
    });
  });
}
