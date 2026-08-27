import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/catalog/shared/shared.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/governance/seller_tier_badge.dart';
import 'package:labuda/shared/models/seller_identity_data.dart';
import 'package:labuda/shared/widgets/media_viewer_widget.dart';
import 'package:labuda/shared/widgets/seller_dual_avatar.dart';
import 'package:labuda/shared/widgets/stable_network_image.dart';

class _FakeVideoEngine implements MediaViewerVideoEngine {
  _FakeVideoEngine({required this.mediaId, required this.initializeCompleter});

  final String mediaId;
  final Completer<void> initializeCompleter;

  int initializeCalls = 0;
  int playCalls = 0;
  int pauseCalls = 0;
  int disposeCalls = 0;

  @override
  Future<void> initialize() async {
    initializeCalls += 1;
    await initializeCompleter.future;
  }

  @override
  Future<void> play() async {
    playCalls += 1;
  }

  @override
  Future<void> pause() async {
    pauseCalls += 1;
  }

  @override
  bool get isPlaying => playCalls > pauseCalls;

  @override
  Widget buildPlayer() {
    return Container(
      key: ValueKey('player-$mediaId'),
      color: Colors.black,
      child: Center(child: Text('player-$mediaId')),
    );
  }

  @override
  void dispose() {
    disposeCalls += 1;
  }
}

class _VideoHarness {
  final engines = <String, _FakeVideoEngine>{};
  final completers = <String, Completer<void>>{};

  MediaViewerVideoEngine build(MediaEntity media) {
    return engines.putIfAbsent(media.id, () {
      final completer = completers.putIfAbsent(media.id, Completer<void>.new);
      return _FakeVideoEngine(
        mediaId: media.id,
        initializeCompleter: completer,
      );
    });
  }

  void complete(String mediaId) {
    final completer = completers[mediaId];
    if (completer != null && !completer.isCompleted) {
      completer.complete();
    }
  }
}

Widget _wrap(Widget child) {
  return MaterialApp(
    theme: ThemeData.light(),
    home: Scaffold(body: child),
  );
}

String _resolveImageUrl(ImageProvider<Object> provider) {
  final resolved = provider is ResizeImage ? provider.imageProvider : provider;
  return (resolved as NetworkImage).url;
}

void _expectAppBarActionSlot(
  WidgetTester tester, {
  required String tooltip,
  required IconData icon,
  required Color expectedColor,
}) {
  final action = find.byTooltip(tooltip);
  expect(action, findsOneWidget);
  expect(tester.getSize(action), const Size(48, 48));

  final iconFinder = find.descendant(of: action, matching: find.byIcon(icon));
  expect(tester.getSize(iconFinder), const Size(20, 20));
  expect(tester.widget<Icon>(iconFinder).color, expectedColor);
}

void main() {
  test('CommerceViewerCapabilities guest is fully closed and round-trips', () {
    const guest = CommerceViewerCapabilities.guest();

    expect(guest.isGuest, isTrue);
    expect(guest.isOwner, isFalse);
    expect(guest.isBuyer, isFalse);
    expect(guest.canManage, isFalse);
    expect(guest.canEdit, isFalse);
    expect(guest.canPromote, isFalse);
    expect(guest.canChat, isFalse);
    expect(guest.canNegotiate, isFalse);
    expect(guest.canBuy, isFalse);
    expect(guest.canBid, isFalse);
    expect(guest.canBuyNow, isFalse);
    expect(CommerceViewerCapabilities.fromJson(guest.toJson()), guest);
  });

  testWidgets('CommerceDetailRoleAwareActionBar selects children by role', (
    tester,
  ) async {
    await tester.pumpWidget(
      _wrap(
        CommerceDetailRoleAwareActionBar(
          capabilities: const CommerceViewerCapabilities(
            role: 'owner',
            canManage: true,
            canEdit: true,
            canPromote: true,
            canChat: false,
            canNegotiate: false,
            canBuy: false,
            canBid: false,
            canBuyNow: false,
          ),
          ownerChild: const Text('owner-action'),
          buyerChild: const Text('buyer-action'),
          guestChild: const Text('guest-action'),
        ),
      ),
    );

    expect(find.text('owner-action'), findsOneWidget);
    expect(find.text('buyer-action'), findsNothing);
    expect(find.text('guest-action'), findsNothing);

    await tester.pumpWidget(
      _wrap(
        CommerceDetailRoleAwareActionBar(
          capabilities: const CommerceViewerCapabilities(
            role: 'buyer',
            canManage: false,
            canEdit: false,
            canPromote: false,
            canChat: true,
            canNegotiate: true,
            canBuy: true,
            canBid: false,
            canBuyNow: false,
          ),
          ownerChild: const Text('owner-action'),
          buyerChild: const Text('buyer-action'),
          guestChild: const Text('guest-action'),
        ),
      ),
    );

    expect(find.text('owner-action'), findsNothing);
    expect(find.text('buyer-action'), findsOneWidget);
    expect(find.text('guest-action'), findsNothing);

    await tester.pumpWidget(
      _wrap(
        CommerceDetailRoleAwareActionBar(
          capabilities: const CommerceViewerCapabilities.guest(),
          ownerChild: const Text('owner-action'),
          buyerChild: const Text('buyer-action'),
          guestChild: const Text('guest-action'),
        ),
      ),
    );

    expect(find.text('owner-action'), findsNothing);
    expect(find.text('buyer-action'), findsNothing);
    expect(find.text('guest-action'), findsOneWidget);
  });

  testWidgets('CommerceDetailScaffold uses the shared surface color', (
    tester,
  ) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: ThemeData.light(),
        home: const CommerceDetailScaffold(body: Text('detail body')),
      ),
    );

    final scaffold = tester.widget<Scaffold>(find.byType(Scaffold));
    expect(scaffold.backgroundColor, ThemeData.light().colorScheme.surface);
  });

  testWidgets('CommerceDetailStickyActionBar keeps SafeArea and child', (
    tester,
  ) async {
    await tester.pumpWidget(
      _wrap(const CommerceDetailStickyActionBar(child: Text('sticky cta'))),
    );

    expect(find.byType(SafeArea), findsOneWidget);
    expect(find.text('sticky cta'), findsOneWidget);
  });

  testWidgets('CommerceDetailMediaGallery keeps controller across refresh', (
    tester,
  ) async {
    const firstUrl =
        'https://cdn.example.com/gallery/first.jpg?X-Amz-Signature=one';
    const secondUrl =
        'https://cdn.example.com/gallery/second.jpg?X-Amz-Signature=two';
    const firstUrlUpdated =
        'https://cdn.example.com/gallery/first.jpg?X-Amz-Signature=updated';
    const secondUrlUpdated =
        'https://cdn.example.com/gallery/second.jpg?X-Amz-Signature=updated';

    await tester.pumpWidget(
      _wrap(
        CommerceDetailMediaGallery(
          cacheKey: 'auction-1',
          media: [
            MediaEntity(
              id: 'first',
              originalUrl: firstUrl,
              type: MediaType.image,
              createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
            ),
            MediaEntity(
              id: 'second',
              originalUrl: secondUrl,
              type: MediaType.image,
              createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
            ),
          ],
          logicalCacheKeyBuilder: (media, index) => '${media.id}|$index',
        ),
      ),
    );

    final controllerBefore = tester
        .widget<PageView>(find.byType(PageView))
        .controller;
    expect(find.byType(StableNetworkImage), findsOneWidget);

    await tester.drag(find.byType(PageView), const Offset(-400, 0));
    await tester.pumpAndSettle();

    await tester.pumpWidget(
      _wrap(
        CommerceDetailMediaGallery(
          cacheKey: 'auction-1',
          media: [
            MediaEntity(
              id: 'first',
              originalUrl: firstUrlUpdated,
              type: MediaType.image,
              createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
            ),
            MediaEntity(
              id: 'second',
              originalUrl: secondUrlUpdated,
              type: MediaType.image,
              createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
            ),
          ],
          logicalCacheKeyBuilder: (media, index) => '${media.id}|$index',
        ),
      ),
    );
    await tester.pumpAndSettle();

    final controllerAfter = tester
        .widget<PageView>(find.byType(PageView))
        .controller;
    expect(identical(controllerBefore, controllerAfter), isTrue);

    final images = tester.widgetList<Image>(find.byType(Image)).toList();
    expect(images, hasLength(1));
    expect(_resolveImageUrl(images.single.image), secondUrlUpdated);
  });

  testWidgets('CommerceDetailMediaGallery lazily plays typed video media', (
    tester,
  ) async {
    final harness = _VideoHarness();

    await tester.pumpWidget(
      _wrap(
        CommerceDetailMediaGallery(
          cacheKey: 'auction-video',
          media: [
            MediaEntity(
              id: 'video-a',
              originalUrl: 'https://cdn.example.com/gallery/video-a.mp4',
              type: MediaType.video,
              createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
              variants: const <String, String>{
                'thumbnail':
                    'https://cdn.example.com/gallery/video-a-thumb.jpg',
              },
            ),
          ],
          logicalCacheKeyBuilder: (media, index) => '${media.id}|$index',
          videoEngineBuilder: harness.build,
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(harness.engines, isEmpty);
    expect(find.byIcon(Icons.play_arrow_rounded), findsOneWidget);

    await tester.tap(find.byIcon(Icons.play_arrow_rounded));
    await tester.pump();

    expect(harness.engines, hasLength(1));
    expect(harness.engines['video-a']!.initializeCalls, 1);

    harness.complete('video-a');
    await tester.pumpAndSettle();

    expect(find.byKey(const ValueKey('player-video-a')), findsOneWidget);
    expect(harness.engines['video-a']!.playCalls, 1);
  });

  testWidgets('CommerceDetailSellerIdentityCard renders identity and badge', (
    tester,
  ) async {
    await tester.pumpWidget(
      _wrap(
        CommerceDetailSellerIdentityCard(
          identity: const SellerIdentityData(
            userId: 'seller-2',
            username: '@@qiqijho',
            storeName: 'Qiqi Store',
            avatarUrl: 'https://example.com/avatar.jpg',
            isSeller: true,
          ),
          isDegraded: false,
          redactionLabel: 'unused',
          sellerTier: 'pro',
        ),
      ),
    );

    expect(find.text('Qiqi Store'), findsOneWidget);
    expect(find.text('@qiqijho'), findsOneWidget);
    expect(find.byType(SellerDualAvatar), findsOneWidget);
    expect(find.byType(SellerTierBadge), findsOneWidget);
  });

  testWidgets('CommerceDetailSellerIdentityCard redacts degraded sellers', (
    tester,
  ) async {
    await tester.pumpWidget(
      _wrap(
        CommerceDetailSellerIdentityCard(
          identity: const SellerIdentityData(
            userId: 'seller-3',
            username: 'seller',
            storeName: 'Redacted Farm',
            avatarUrl: null,
            isSeller: true,
          ),
          isDegraded: true,
          redactionLabel: ContentLifecycle.removed.publicRedactionLabel,
        ),
      ),
    );

    expect(
      find.text(ContentLifecycle.removed.publicRedactionLabel),
      findsOneWidget,
    );
    expect(find.byType(SellerTierBadge), findsNothing);
  });

  testWidgets('CommerceDetailShell handles loading error and not found', (
    tester,
  ) async {
    await tester.pumpWidget(
      _wrap(
        const CommerceDetailShell(
          state: CommerceDetailShellState.loading,
          loadingBuilder: Center(child: CircularProgressIndicator()),
        ),
      ),
    );
    expect(find.byType(CircularProgressIndicator), findsOneWidget);

    await tester.pumpWidget(
      _wrap(
        const CommerceDetailShell(
          state: CommerceDetailShellState.error,
          errorBuilder: Center(child: Text('Data belum bisa dimuat.')),
        ),
      ),
    );
    expect(find.text('Data belum bisa dimuat.'), findsOneWidget);

    await tester.pumpWidget(
      _wrap(
        CommerceDetailShell(
          state: CommerceDetailShellState.notFound,
          notFoundBuilder: (context) =>
              const Center(child: Text('Data tidak ditemukan')),
        ),
      ),
    );
    expect(find.text('Data tidak ditemukan'), findsOneWidget);
  });

  testWidgets('CommerceDetailAppBarActions keeps all icon slots aligned', (
    tester,
  ) async {
    for (final theme in <ThemeData>[ThemeData.light(), ThemeData.dark()]) {
      await tester.pumpWidget(
        MaterialApp(
          theme: theme,
          home: Scaffold(
            body: Center(
              child: CommerceDetailAppBarActions(
                actions: [
                  CommerceDetailAppBarActionButton(
                    icon: Icons.bookmark_border_outlined,
                    activeIcon: Icons.bookmark,
                    isActive: true,
                    tooltip: 'Tersimpan',
                    semanticsLabel: 'Tersimpan',
                    onPressed: () {},
                    activeColor: theme.colorScheme.primary,
                    inactiveColor: theme.colorScheme.onSurfaceVariant,
                  ),
                  CommerceDetailAppBarActionButton(
                    icon: Icons.share_outlined,
                    tooltip: 'Bagikan',
                    semanticsLabel: 'Bagikan',
                    onPressed: () {},
                    inactiveColor: theme.colorScheme.onSurfaceVariant,
                  ),
                  CommerceDetailAppBarActionButton(
                    icon: Icons.more_vert,
                    tooltip: 'Lainnya',
                    semanticsLabel: 'Lainnya',
                    onPressed: () {},
                    inactiveColor: theme.colorScheme.onSurfaceVariant,
                  ),
                ],
              ),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();

      _expectAppBarActionSlot(
        tester,
        tooltip: 'Tersimpan',
        icon: Icons.bookmark,
        expectedColor: theme.colorScheme.primary,
      );
      _expectAppBarActionSlot(
        tester,
        tooltip: 'Bagikan',
        icon: Icons.share_outlined,
        expectedColor: theme.colorScheme.onSurfaceVariant,
      );
      _expectAppBarActionSlot(
        tester,
        tooltip: 'Lainnya',
        icon: Icons.more_vert,
        expectedColor: theme.colorScheme.onSurfaceVariant,
      );

      final savedRect = tester.getRect(find.byTooltip('Tersimpan'));
      final shareRect = tester.getRect(find.byTooltip('Bagikan'));
      final moreRect = tester.getRect(find.byTooltip('Lainnya'));
      expect(savedRect.width, shareRect.width);
      expect(shareRect.width, moreRect.width);
      expect(savedRect.height, shareRect.height);
      expect(shareRect.height, moreRect.height);
      expect(savedRect.center.dy, shareRect.center.dy);
      expect(shareRect.center.dy, moreRect.center.dy);
      expect(shareRect.left, savedRect.right);
      expect(moreRect.left, shareRect.right);
      expect(savedRect.left, greaterThanOrEqualTo(0));
      expect(tester.takeException(), isNull);
    }
  });

  testWidgets('CommerceDetailShell keeps long content scrollable on narrow width', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(320, 640));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    const longTitle =
        'Very long shell title that should wrap without overflow on a compact phone width';
    const longValue =
        'Rp 12.345.678.900 - primary value that should remain readable across text scales';

    await tester.pumpWidget(
      MaterialApp(
        theme: ThemeData.light(),
        home: MediaQuery(
          data: const MediaQueryData(textScaler: TextScaler.linear(1.4)),
          child: CommerceDetailShell(
            appBar: AppBar(title: const Text('Detail')),
            state: CommerceDetailShellState.content,
            media: Container(
              height: 220,
              color: Colors.blueGrey,
              alignment: Alignment.center,
              child: const Text('media'),
            ),
            title: longTitle,
            primaryValue: longValue,
            sellerIdentity: const SizedBox.shrink(),
            domainSections: List<Widget>.generate(
              4,
              (index) => CommerceDetailSectionCard(
                child: SizedBox(
                  height: 72,
                  child: Align(
                    alignment: Alignment.centerLeft,
                    child: Text('section ${index + 1}'),
                  ),
                ),
              ),
            ),
            supportingSections: [
              CommerceDetailSectionCard(
                child: SizedBox(
                  height: 72,
                  child: Align(
                    alignment: Alignment.centerLeft,
                    child: Text('final section'),
                  ),
                ),
              ),
            ],
            bottomNavigationBar: CommerceDetailStickyActionBar(
              child: SizedBox(
                height: 48,
                width: double.infinity,
                child: ElevatedButton(
                  onPressed: () {},
                  child: const Text('buy'),
                ),
              ),
            ),
          ),
        ),
      ),
    );

    expect(tester.takeException(), isNull);

    await tester.scrollUntilVisible(
      find.text('final section'),
      400,
      scrollable: find.byType(Scrollable),
    );
    await tester.pumpAndSettle();

    expect(find.text('final section'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('CommerceDetailShell respects light theme surfaces', (
    tester,
  ) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: ThemeData.light(),
        home: CommerceDetailShell(
          state: CommerceDetailShellState.content,
          title: 'Light theme title',
          primaryValue: 'Rp 250.000',
          domainSections: const [SizedBox(height: 64)],
        ),
      ),
    );

    final scaffold = tester.widget<Scaffold>(find.byType(Scaffold));
    expect(scaffold.backgroundColor, ThemeData.light().colorScheme.surface);
    expect(tester.takeException(), isNull);
  });

  testWidgets('CommerceDetailShell respects dark theme surfaces', (
    tester,
  ) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: ThemeData.dark(),
        home: CommerceDetailShell(
          state: CommerceDetailShellState.content,
          title: 'Dark theme title',
          primaryValue: 'Rp 250.000',
          domainSections: const [SizedBox(height: 64)],
        ),
      ),
    );

    final scaffold = tester.widget<Scaffold>(find.byType(Scaffold));
    expect(scaffold.backgroundColor, ThemeData.dark().colorScheme.surface);
    expect(tester.takeException(), isNull);
  });
}
