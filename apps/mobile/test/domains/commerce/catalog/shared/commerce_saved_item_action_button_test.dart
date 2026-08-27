import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/shared/presentation/widgets/commerce_saved_item_action_button.dart';
import 'package:labuda/domains/user/preference/saved_item/data/repositories/saved_item_repository.dart';
import 'package:labuda/domains/user/preference/saved_item/data/repositories/saved_item_repository_provider.dart';
import 'package:labuda/domains/user/preference/saved_item/models/saved_item_model.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';

class _FakeAuthController extends AuthController {
  _FakeAuthController(this._state);

  final AuthState _state;

  @override
  AuthState build() => _state;
}

class _FakeSavedItemRepository extends SavedItemRepository {
  _FakeSavedItemRepository({this.initialSaved = false, this.failOnAdd = false})
    : super(dio: Dio(BaseOptions(baseUrl: 'http://localhost')));

  bool initialSaved;
  bool failOnAdd;

  int isSavedCalls = 0;
  int addCalls = 0;
  int removeCalls = 0;
  String? lastTargetType;
  String? lastTargetId;

  @override
  Future<bool> isSaved({
    required String targetType,
    required String targetId,
  }) async {
    isSavedCalls += 1;
    lastTargetType = targetType;
    lastTargetId = targetId;
    return initialSaved;
  }

  @override
  Future<SavedItemModel> addSavedItem({
    required String targetType,
    required String targetId,
  }) async {
    addCalls += 1;
    lastTargetType = targetType;
    lastTargetId = targetId;
    if (failOnAdd) {
      throw Exception('save failed');
    }
    initialSaved = true;
    return SavedItemModel(
      id: 'saved-1',
      userId: 'user-1',
      targetType: targetType == 'listing'
          ? TargetType.listing
          : TargetType.auction,
      targetId: targetId,
      intentType: targetType == 'listing'
          ? IntentType.bookmark
          : IntentType.watch,
      createdAt: DateTime.utc(2026, 1, 1),
    );
  }

  @override
  Future<void> removeSavedItem({
    required String targetType,
    required String targetId,
  }) async {
    removeCalls += 1;
    lastTargetType = targetType;
    lastTargetId = targetId;
    initialSaved = false;
  }
}

AuthUser _authUser({required String id}) {
  final now = DateTime.utc(2026, 1, 1);
  return AuthUser(
    id: id,
    createdAt: now,
    updatedAt: now,
    email: '$id@example.com',
    username: id,
    isEmailVerified: true,
    roles: const [UserRole.user],
    provider: ShonaAuthProvider.email,
    lifecycle: ContentLifecycle.active,
  );
}

Widget _wrap({
  required Widget child,
  required AuthState authState,
  required SavedItemRepository savedItemRepository,
}) {
  return ProviderScope(
    overrides: [
      authControllerProvider.overrideWith(() => _FakeAuthController(authState)),
      savedItemRepositoryProvider.overrideWithValue(savedItemRepository),
    ],
    child: MaterialApp(
      home: Scaffold(body: Center(child: child)),
    ),
  );
}

void main() {
  testWidgets('guest viewers do not see the save button', (tester) async {
    final repo = _FakeSavedItemRepository();

    await tester.pumpWidget(
      _wrap(
        child: const CommerceSavedItemActionButton(
          targetType: 'listing',
          targetId: 'listing-1',
          label: 'Simpan',
          activeLabel: 'Tersimpan',
          icon: Icons.bookmark_border_outlined,
          activeIcon: Icons.bookmark,
        ),
        authState: const AuthState.unauthenticated(),
        savedItemRepository: repo,
      ),
    );

    await tester.pumpAndSettle();

    expect(find.byTooltip('Simpan'), findsNothing);
    expect(find.text('Simpan'), findsNothing);
    expect(find.text('Tersimpan'), findsNothing);
    expect(repo.isSavedCalls, 0);
  });

  testWidgets('loads saved state and shows the active label after refresh', (
    tester,
  ) async {
    final repo = _FakeSavedItemRepository(initialSaved: true);

    await tester.pumpWidget(
      _wrap(
        child: const CommerceSavedItemActionButton(
          targetType: 'listing',
          targetId: 'listing-1',
          label: 'Simpan',
          activeLabel: 'Tersimpan',
          icon: Icons.bookmark_border_outlined,
          activeIcon: Icons.bookmark,
        ),
        authState: AuthState.authenticated(
          _authUser(id: 'buyer-1'),
          emailVerified: true,
        ),
        savedItemRepository: repo,
      ),
    );

    await tester.pumpAndSettle();

    expect(tester.getSize(find.byTooltip('Tersimpan')), const Size(48, 48));
    expect(find.text('Tersimpan'), findsNothing);
    expect(find.text('Simpan'), findsNothing);
    expect(find.byTooltip('Tersimpan'), findsOneWidget);
    expect(find.bySemanticsLabel('Tersimpan'), findsOneWidget);
    expect(repo.isSavedCalls, 1);
    expect(repo.lastTargetType, 'listing');
    expect(repo.lastTargetId, 'listing-1');
  });

  testWidgets('optimistically toggles save state and persists it', (
    tester,
  ) async {
    final repo = _FakeSavedItemRepository(initialSaved: false);

    await tester.pumpWidget(
      _wrap(
        child: const CommerceSavedItemActionButton(
          key: ValueKey('initial-save-toggle'),
          targetType: 'listing',
          targetId: 'listing-2',
          label: 'Simpan',
          activeLabel: 'Tersimpan',
          icon: Icons.bookmark_border_outlined,
          activeIcon: Icons.bookmark,
        ),
        authState: AuthState.authenticated(
          _authUser(id: 'buyer-2'),
          emailVerified: true,
        ),
        savedItemRepository: repo,
      ),
    );

    await tester.pumpAndSettle();
    expect(find.byTooltip('Simpan'), findsOneWidget);
    expect(tester.getSize(find.byTooltip('Simpan')), const Size(48, 48));

    await tester.tap(find.byTooltip('Simpan'));
    await tester.pump();
    await tester.pumpAndSettle();

    expect(find.byTooltip('Tersimpan'), findsOneWidget);
    expect(tester.getSize(find.byTooltip('Tersimpan')), const Size(48, 48));
    expect(repo.addCalls, 1);
    expect(repo.removeCalls, 0);
    expect(repo.initialSaved, isTrue);

    await tester.pumpWidget(
      _wrap(
        child: const CommerceSavedItemActionButton(
          key: ValueKey('reloaded-save-toggle'),
          targetType: 'listing',
          targetId: 'listing-2',
          label: 'Simpan',
          activeLabel: 'Tersimpan',
          icon: Icons.bookmark_border_outlined,
          activeIcon: Icons.bookmark,
        ),
        authState: AuthState.authenticated(
          _authUser(id: 'buyer-2'),
          emailVerified: true,
        ),
        savedItemRepository: repo,
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byTooltip('Tersimpan'), findsOneWidget);
    expect(find.text('Tersimpan'), findsNothing);
    expect(repo.isSavedCalls, 2);
  });

  testWidgets('rolls back the optimistic state when persistence fails', (
    tester,
  ) async {
    final repo = _FakeSavedItemRepository(initialSaved: false, failOnAdd: true);

    await tester.pumpWidget(
      _wrap(
        child: const CommerceSavedItemActionButton(
          targetType: 'auction',
          targetId: 'auction-1',
          label: 'Pantau',
          activeLabel: 'Dipantau',
          icon: Icons.visibility_outlined,
          activeIcon: Icons.visibility,
        ),
        authState: AuthState.authenticated(
          _authUser(id: 'buyer-3'),
          emailVerified: true,
        ),
        savedItemRepository: repo,
      ),
    );

    await tester.pumpAndSettle();
    expect(find.byTooltip('Pantau'), findsOneWidget);
    expect(tester.getSize(find.byTooltip('Pantau')), const Size(48, 48));

    await tester.tap(find.byTooltip('Pantau'));
    await tester.pump();
    await tester.pumpAndSettle();

    expect(find.byTooltip('Pantau'), findsOneWidget);
    expect(find.byTooltip('Dipantau'), findsNothing);
    expect(repo.addCalls, 1);
    expect(repo.initialSaved, isFalse);
  });
}
