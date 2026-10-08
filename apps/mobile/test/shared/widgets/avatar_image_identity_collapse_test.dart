// IMAGE IDENTITY under an ANIMATED avatar size.
//
// Regression lock for the Profile header flicker (foto → placeholder → foto
// while the header collapses). Root cause (audit, HIGH confidence): the
// header lerps the avatar size 96 → 40 on every scroll frame and the decode
// target used to be derived from that animated value
// (`cacheWidth: (size * 2).round()`), so each frame produced a new
// ResizeImage key → OctoImage's `Image(key: ValueKey(image))` re-instituted
// its State → frame == null → placeholder → reload → crossfade.
//
// Contract locked here:
//   visual size MAY change frame by frame,
//   image provider / decode identity MUST NOT.
//
// The proof is element identity: the `Image` element can only survive a
// rebuild while its widget (key = ValueKey(image provider)) stays `==`, so
// "same element + equal provider at every animated size" is exactly
// "the image never falls back to frame == null".

import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/profile/data/datasources/user_api_datasource.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart'
    show avatarCacheServiceProvider;
import 'package:labuda/domains/user/profile/data/services/avatar_cache_service.dart';
import 'package:labuda/shared/widgets/app_image.dart';
import 'package:labuda/shared/widgets/profile_avatar.dart';
import 'package:labuda/shared/widgets/seller_avatar.dart';

/// Decode targets pinned by the Profile header for the flying avatar:
/// expanded size 96 × 2 for the main circle, and 0.4 × that value for the
/// personal overlay inside the dual avatar — mirrors ProfileScreen's
/// `_avatarDecodeWidth`.
const int _pinnedMainDecodeWidth = 192;
const int _pinnedPersonalDecodeWidth = 77;

const String _avatarUrl = 'https://d358tu61i1wrtt.cloudfront.net/images/a.png';
const String _storeUrl = 'https://d358tu61i1wrtt.cloudfront.net/images/s.png';

/// The visual sizes the header sweeps through while collapsing
/// (96 → 40 in five representative frames).
const List<double> _collapseFrames = <double>[96, 82.4, 68.8, 55.2, 40];

Widget _wrap(Widget child) =>
    MaterialApp(home: Scaffold(body: Center(child: child)));

List<Element> _imageElements(WidgetTester tester) =>
    find.byType(Image).evaluate().toList();

List<Object> _imageProviders(WidgetTester tester) => _imageElements(tester)
    .map((Element e) => (e.widget as Image).image)
    .toList();

class _FakeAuthController extends AuthController {
  _FakeAuthController(this._state);
  final AuthState _state;

  @override
  AuthState build() => _state;
}

class _NoOpDatasource extends Fake implements UserApiDatasource {}

class _NoFetchAvatarCacheService extends AvatarCacheService {
  _NoFetchAvatarCacheService() : super(datasource: _NoOpDatasource());

  @override
  Future<String?> getUserAvatarUrl(String userId) async => null;
}

AuthUser _authUser({required String id, String? avatarUrl}) => AuthUser(
  id: id,
  createdAt: DateTime(2025),
  updatedAt: DateTime(2025),
  email: '$id@test.com',
  username: id,
  avatarUrl: avatarUrl,
  isEmailVerified: true,
  roles: const [UserRole.user],
  provider: AuthProvider.email,
);

void main() {
  group('ProfileAvatar: pinned cacheWidth keeps the image identity stable', () {
    testWidgets('the Image element survives the whole 96 → 40 size sweep', (
      tester,
    ) async {
      Element? firstElement;
      Object? firstProvider;

      for (final double frame in _collapseFrames) {
        await tester.pumpWidget(
          _wrap(
            ProfileAvatar(
              userId: 'u1',
              size: frame,
              imageUrl: _avatarUrl,
              cacheWidth: _pinnedMainDecodeWidth,
            ),
          ),
        );
        await tester.pump();

        final images = _imageElements(tester);
        expect(images, hasLength(1), reason: 'frame $frame renders 1 image');

        if (firstElement == null) {
          firstElement = images.single;
          firstProvider = _imageProviders(tester).single;
          continue;
        }

        // Same element => the Image state was never re-instituted, so it can
        // never come back as frame == null (the placeholder flash).
        expect(
          identical(images.single, firstElement),
          isTrue,
          reason:
              'frame $frame: Image element must survive the size change '
              '(state reset = placeholder flash)',
        );
        expect(
          _imageProviders(tester).single,
          firstProvider,
          reason: 'frame $frame: image provider identity must not move',
        );
      }

      // Anti-vacuum: the visual size really is animating in this test.
      expect(
        tester.getSize(find.byType(ProfileAvatar)).width,
        _collapseFrames.last,
      );
    });
  });

  group('SellerAvatar seller path (dual) pins BOTH decode targets', () {
    testWidgets('store 192 + personal 77, identities stable while size moves', (
      tester,
    ) async {
      List<AppImage> appImages() =>
          tester.widgetList<AppImage>(find.byType(AppImage)).toList();

      List<Element>? firstElements;
      List<Object>? firstProviders;

      for (final double frame in _collapseFrames) {
        await tester.pumpWidget(
          _wrap(
            SellerAvatar(
              userId: 'u1',
              avatarUrl: _avatarUrl,
              storeImageUrl: _storeUrl,
              isSeller: true,
              size: frame,
              cacheWidth: _pinnedMainDecodeWidth,
            ),
          ),
        );
        await tester.pump();

        final images = _imageElements(tester);
        expect(images, hasLength(2), reason: 'frame $frame: store + personal');

        // Decode targets are constants for the whole animation.
        expect(
          appImages().map((AppImage a) => a.cacheWidth).toList(),
          <int>[_pinnedMainDecodeWidth, _pinnedPersonalDecodeWidth],
          reason: 'frame $frame: cacheWidth must not follow the animated size',
        );

        if (firstElements == null) {
          firstElements = List<Element>.of(images);
          firstProviders = _imageProviders(tester);
          continue;
        }

        for (int i = 0; i < images.length; i++) {
          expect(
            identical(images[i], firstElements[i]),
            isTrue,
            reason:
                'frame $frame image #$i: element must survive (state reset = '
                'placeholder flash)',
          );
          expect(
            _imageProviders(tester)[i],
            firstProviders![i],
            reason: 'frame $frame image #$i: provider identity must not move',
          );
        }
      }

      // Anti-vacuum: layout really shrank with the animation.
      expect(
        tester.getSize(find.byType(SellerAvatar)).width,
        _collapseFrames.last,
      );
    });
  });

  group('SellerAvatar non-seller path (HybridAvatar → ProfileAvatar)', () {
    testWidgets('pinned cacheWidth reaches the renderer and stays stable', (
      tester,
    ) async {
      const userId = 'principal-1';
      final user = _authUser(
        id: userId,
        avatarUrl: 'https://auth.example/me.png',
      );

      Widget buildAt(double size) => ProviderScope(
        overrides: [
          authControllerProvider.overrideWith(
            () => _FakeAuthController(
              AuthState.authenticated(user, emailVerified: true),
            ),
          ),
          avatarCacheServiceProvider.overrideWith(
            (_) => _NoFetchAvatarCacheService(),
          ),
        ],
        child: _wrap(
          SellerAvatar(
            userId: userId,
            avatarUrl: user.avatarUrl,
            isSeller: false,
            size: size,
            cacheWidth: _pinnedMainDecodeWidth,
          ),
        ),
      );

      Element? firstElement;
      Object? firstProvider;

      for (final double frame in _collapseFrames) {
        await tester.pumpWidget(buildAt(frame));
        await tester.pump();

        final images = _imageElements(tester);
        expect(images, hasLength(1), reason: 'frame $frame renders 1 image');
        expect(
          tester.widget<ProfileAvatar>(find.byType(ProfileAvatar)).cacheWidth,
          _pinnedMainDecodeWidth,
          reason:
              'frame $frame: SellerAvatar → HybridAvatar → ProfileAvatar must '
              'forward the pinned decode target',
        );

        if (firstElement == null) {
          firstElement = images.single;
          firstProvider = _imageProviders(tester).single;
          continue;
        }

        expect(
          identical(images.single, firstElement),
          isTrue,
          reason: 'frame $frame: Image element must survive the size change',
        );
        expect(
          _imageProviders(tester).single,
          firstProvider,
          reason: 'frame $frame: provider identity must not move',
        );
      }

      expect(
        tester.getSize(find.byType(SellerAvatar)).width,
        _collapseFrames.last,
      );
    });
  });

  group('anti-vacuum: the OLD size-derived derivation really does churn', () {
    testWidgets(
      'without a pinned cacheWidth the provider identity changes with size',
      (tester) async {
        Element? firstElement;
        Object? firstProvider;

        for (final double frame in _collapseFrames) {
          await tester.pumpWidget(
            _wrap(
              ProfileAvatar(userId: 'u1', size: frame, imageUrl: _avatarUrl),
            ),
          );
          await tester.pump();

          if (firstElement == null) {
            firstElement = _imageElements(tester).single;
            firstProvider = _imageProviders(tester).single;
            continue;
          }

          // This is the defect being fixed: size-derived decode target →
          // new ResizeImage key → Image element replaced → frame == null.
          expect(
            _imageProviders(tester).single,
            isNot(firstProvider),
            reason: 'size-derived cacheWidth must be detectable by this test',
          );
          expect(
            identical(_imageElements(tester).single, firstElement),
            isFalse,
            reason: 'size-derived cacheWidth must be detectable by this test',
          );
        }
      },
    );
  });

  group('ProfileScreen wiring', () {
    test('the flying avatar pins its decode width to a constant', () {
      final source = File(
        'lib/domains/user/profile/presentation/screens/profile_screen.dart',
      ).readAsStringSync();

      expect(
        source.contains('cacheWidth: _avatarDecodeWidth'),
        isTrue,
        reason:
            'the animated avatar must receive a pinned decode target, never a '
            'size-derived one',
      );
      expect(
        source.contains(
          'static final int _avatarDecodeWidth = (_avatarSize * 2).round();',
        ),
        isTrue,
        reason:
            'the pinned decode target is derived from the expanded (largest) '
            'visual size constant, not from the per-frame lerped size',
      );
      // No line that feeds AppImage's decode target may read from the
      // animated size or the lerp helper.
      final cacheWidthLines = source
          .split('\n')
          .where((String line) => line.contains('cacheWidth:'))
          .toList();
      expect(cacheWidthLines, isNotEmpty);
      for (final String line in cacheWidthLines) {
        expect(
          line.contains('currentAvatarSize') || line.contains('_lerp'),
          isFalse,
          reason: 'no decode target may be derived from the animated size: '
              '$line',
        );
      }
    });
  });
}
