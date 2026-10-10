// METADATA — HORIZONTAL COMPOSITION ACCEPTANCE GATE.
//
// Owner-locked semantic: ONE metadata family with TWO canonical presentation
// modes (see lib/shared/widgets/metadata_view.dart):
//
//   compact → single bounded line + TextOverflow.ellipsis
//   detail  → wrapping text, horizontally bounded by its parent
//
// A standalone secondary fact (join date, last active, a timestamp, a count)
// belongs here. A label → value relationship belongs to the Label/Value
// family and is deliberately NOT covered by this gate.
//
// This suite proves the authority by BEHAVIOUR across a WIDTH x TEXT-SCALE
// matrix with realistic long fixtures:
//
//   * compact must NOT throw, must stay inside the surface, must set
//     maxLines:1 + ellipsis, and must actually truncate a long fact;
//   * detail must NOT throw, must stay inside the surface, must NOT cap lines
//     on its own, and must render taller than compact (i.e. it genuinely
//     wraps); a caller-supplied maxLines limit on detail is honoured.
//
// It also pumps the real proving consumers, proving the Owner's original
// symptom now flows through the same authority:
//
//   * ProfileAboutTab join-date + last-active rows (the proven failure);
//   * LoginSessionsScreen session timestamp row;
//   * follow UserCard followers/following counts line.
//
// It deliberately does not test unrelated families (label/value, price,
// badge, identity, notification, chips). DetailChipWidget, TimeAgoWidget,
// UserHeaderWidget, ContentMetadataSections, ProfileInfoRow and the order
// _InfoRow variants are NOT converged here and are NOT subjects of this gate.
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/social/follow/domain/entities/follow_entity.dart';
import 'package:hishumi/domains/social/follow/presentation/widgets/user_card.dart';
import 'package:hishumi/domains/user/identity/authentication/presentation/screens/login_sessions_screen.dart';
import 'package:hishumi/domains/user/profile/data/models/api/user_api_models.dart';
import 'package:hishumi/domains/user/profile/domain/entities/profile_entity.dart';
import 'package:hishumi/domains/user/profile/profile.dart'
    show ProfileAboutData, profileAboutDataProvider;
import 'package:hishumi/domains/user/profile/presentation/screens/profile_screen/profile_about_tab.dart';
import 'package:hishumi/generated/app_localizations.dart';
import 'package:hishumi/shared/shared.dart';

const List<double> _widths = <double>[320, 360, 412, 500];
const List<double> _scales = <double>[1.0, 1.3, 2.0];

/// Realistic long worst-case standalone fact.
const String _longFact =
    'Preferred contact email address for order updates, delivery coordination '
    'and return scheduling across all regions and time zones';

/// Long realistic count/stat aggregate (counts stay short individually, but
/// an aggregate metadata line must still truncate rather than overflow).
const String _longCountFact =
    '123,456,789 followers • 98,765,432 following • 1,234 mutual connections '
    'across regions';

const double _surfaceHeight = 720;

// ============================================================================
// Harness
// ============================================================================

/// A bounded horizontal slot under a fixed surface + text scale, matching the
/// horizontal composition contract gate.
Widget _surface({
  required Size surface,
  required double scale,
  required Widget subject,
}) {
  return MaterialApp(
    theme: AppTheme.lightTheme,
    localizationsDelegates: AppLocalizations.localizationsDelegates,
    supportedLocales: AppLocalizations.supportedLocales,
    home: MediaQuery(
      data: MediaQueryData(
        size: surface,
        textScaler: TextScaler.linear(scale),
      ),
      child: Scaffold(
        body: Center(
          child: SizedBox(
            width: surface.width - 24,
            height: _surfaceHeight,
            child: subject,
          ),
        ),
      ),
    ),
  );
}

Widget _harness({
  required Size surface,
  required double scale,
  required Widget subject,
}) {
  return ProviderScope(
    child: _surface(surface: surface, scale: scale, subject: subject),
  );
}

Future<void> _pumpWidgetAt(
  WidgetTester tester,
  Size surface,
  Widget widget, {
  bool settle = false,
}) async {
  await tester.binding.setSurfaceSize(surface);
  addTearDown(() => tester.binding.setSurfaceSize(null));

  await tester.pumpWidget(widget);
  await tester.pump();
  if (settle) {
    // Let async family providers resolve without waiting on any loading
    // spinner animation (pumpAndSettle would time out on one).
    await tester.pump(const Duration(milliseconds: 100));
    await tester.pump(const Duration(milliseconds: 100));
  }
}

/// The `Text` the authority actually builds (first match in traversal order).
Text _authorityText(WidgetTester tester, {Finder? scope}) {
  final Finder root = scope ?? find.byType(MetadataText);
  return tester.widget<Text>(
    find.descendant(of: root, matching: find.byType(Text)).first,
  );
}

RenderParagraph _authorityParagraph(WidgetTester tester, {Finder? scope}) {
  final Finder root = scope ?? find.byType(MetadataText);
  return tester.renderObject<RenderParagraph>(
    find.descendant(of: root, matching: find.byType(RichText)).first,
  );
}

void _expectInsideSurface(WidgetTester tester, Size surface, Finder finder) {
  final Rect rect = tester.getRect(finder);
  expect(rect.left, greaterThanOrEqualTo(-0.5), reason: 'left of surface');
  expect(
    rect.right,
    lessThanOrEqualTo(surface.width + 0.5),
    reason: 'wider than surface at ${surface.width}dp',
  );
}

void _expectCompactStrategy(WidgetTester tester, {Finder? scope}) {
  final Text text = _authorityText(tester, scope: scope);
  expect(text.maxLines, 1, reason: 'compact must cap at one line');
  expect(
    text.overflow,
    TextOverflow.ellipsis,
    reason: 'compact must declare an ellipsis strategy',
  );
}

// ============================================================================
// Canonical subjects
// ============================================================================

Widget _compactAuthority() => const MetadataView(
  text: _longFact,
  mode: MetadataMode.compact,
  icon: Icons.calendar_today_outlined,
  iconSize: AppIconSize.inlineGlyph,
);

Widget _compactCountAuthority() => const MetadataView(
  text: _longCountFact,
  mode: MetadataMode.compact,
  icon: Icons.people_outline,
  iconSize: AppIconSize.inlineGlyph,
);

Widget _detailAuthority() => const MetadataView(
  text: _longFact,
  mode: MetadataMode.detail,
  icon: Icons.calendar_today_outlined,
  iconSize: AppIconSize.inlineGlyph,
);

Widget _detailCappedAuthority() => const MetadataView(
  text: _longFact,
  mode: MetadataMode.detail,
  icon: Icons.calendar_today_outlined,
  iconSize: AppIconSize.inlineGlyph,
  maxLines: 2,
);

ProfileAboutData _aboutData() => ProfileAboutData(
  user: AuthUser(
    id: 'about-user',
    createdAt: DateTime(2025),
    updatedAt: DateTime(2025),
    email: 'about@test.com',
    username: 'aboutuser',
    isEmailVerified: true,
    roles: const <UserRole>[UserRole.user],
    provider: AuthProvider.email,
  ),
  profile: ProfileEntity(
    id: 'profile-about-user',
    userId: 'about-user',
    joinedAt: DateTime(2023, 9, 1),
    lastActiveAt: DateTime.now().subtract(const Duration(days: 3, hours: 5)),    stats: const ProfileStats(followersCount: 12, followingCount: 34),
    verification: const UserVerificationInfo(
      isPhoneVerified: false,
      isEmailVerified: false,
      isIdVerified: false,
      isFarmVerified: false,
      badges: <ProfileBadge>[],
    ),
  ),
  // A location renders the About section card (bio is empty); the location
  // itself flows through the CLOSED Address/Location authority, while join
  // date + last active must flow through the Metadata authority.
  location: 'Bandung, Jawa Barat',
);

class _FakeAuthRepository extends Fake implements IAuthRepository {
  @override
  Future<Result<List<AuthSessionDto>>> getActiveSessions() async {
    return Result.success(<AuthSessionDto>[
      AuthSessionDto(
        familyId: 'family-1',
        deviceName:
            'Pixel 8 Pro primary device with a very long custom device name '
            'for narrow width testing',
        platform: 'android',
        appVersion: '3.2.1',
        issuedAt: DateTime(2024, 1, 10, 9, 30),
        expiresAt: DateTime(2026, 1, 10, 9, 30),
        lastUsedAt: DateTime(2025, 11, 20, 14, 45),
      ),
    ]);
  }
}

FollowableUser _followUser() => const FollowableUser(
  id: 'user-1',
  username: 'verylongusername_koi_master_nusantara_premium_collection',
  userType: UserType.seller,
  followersCount: 1234567,
  followingCount: 9876543,
);

// ============================================================================
// Tests
// ============================================================================

void main() {
  group('METADATA authority — compact mode', () {
    testWidgets('stays inside the surface and truncates to one line', (
      tester,
    ) async {
      for (final double width in _widths) {
        for (final double scale in _scales) {
          final Size surface = Size(width, _surfaceHeight);
          await _pumpWidgetAt(
            tester,
            surface,
            _harness(
              surface: surface,
              scale: scale,
              subject: _compactAuthority(),
            ),
          );

          expect(
            tester.takeException(),
            isNull,
            reason: 'compact overflow at ${width}dp @scale $scale',
          );
          _expectInsideSurface(tester, surface, find.byType(MetadataView));
          _expectCompactStrategy(tester);
          expect(
            _authorityParagraph(tester).didExceedMaxLines,
            isTrue,
            reason:
                'a long fact must actually be truncated in compact mode '
                'at ${width}dp @scale $scale',
          );
        }
      }
    });

    testWidgets('long count/stat aggregate truncates instead of overflowing', (
      tester,
    ) async {
      for (final double width in _widths) {
        for (final double scale in _scales) {
          final Size surface = Size(width, _surfaceHeight);
          await _pumpWidgetAt(
            tester,
            surface,
            _harness(
              surface: surface,
              scale: scale,
              subject: _compactCountAuthority(),
            ),
          );

          expect(
            tester.takeException(),
            isNull,
            reason: 'compact count overflow at ${width}dp @scale $scale',
          );
          _expectInsideSurface(tester, surface, find.byType(MetadataView));
          _expectCompactStrategy(tester);
          expect(
            _authorityParagraph(tester).didExceedMaxLines,
            isTrue,
            reason:
                'a long count aggregate must actually be truncated in '
                'compact mode at ${width}dp @scale $scale',
          );
        }
      }
    });
  });

  group('METADATA authority — detail mode', () {
    testWidgets('stays inside the surface and wraps beyond one line', (
      tester,
    ) async {
      for (final double width in _widths) {
        for (final double scale in _scales) {
          final Size surface = Size(width, _surfaceHeight);

          await _pumpWidgetAt(
            tester,
            surface,
            _harness(
              surface: surface,
              scale: scale,
              subject: _detailAuthority(),
            ),
          );
          expect(
            tester.takeException(),
            isNull,
            reason: 'detail overflow at ${width}dp @scale $scale',
          );
          _expectInsideSurface(tester, surface, find.byType(MetadataView));

          final Text detailText = _authorityText(tester);
          expect(
            detailText.maxLines,
            isNull,
            reason: 'detail must not cap lines on its own',
          );
          final double detailHeight = tester
              .getSize(find.byType(MetadataText))
              .height;

          await _pumpWidgetAt(
            tester,
            surface,
            _harness(
              surface: surface,
              scale: scale,
              subject: _compactAuthority(),
            ),
          );
          final double compactHeight = tester
              .getSize(find.byType(MetadataText))
              .height;

          expect(
            detailHeight,
            greaterThan(compactHeight),
            reason:
                'detail must render taller than compact (wrap) at '
                '${width}dp @scale $scale',
          );
        }
      }
    });

    testWidgets('caller-supplied maxLines constrains detail intentionally', (
      tester,
    ) async {
      final Size surface = const Size(360, _surfaceHeight);
      await _pumpWidgetAt(
        tester,
        surface,
        _harness(
          surface: surface,
          scale: 1.0,
          subject: _detailCappedAuthority(),
        ),
      );

      expect(tester.takeException(), isNull);
      final Text text = _authorityText(tester);
      expect(text.maxLines, 2);
      expect(text.overflow, TextOverflow.ellipsis);
    });
  });

  group('Profile About consumer — Owner symptom regression', () {
    testWidgets('join date + last active flow through the compact authority', (
      tester,
    ) async {
      final ProfileAboutData data = _aboutData();

      for (final double width in _widths) {
        for (final double scale in _scales) {
          final Size surface = Size(width, _surfaceHeight);
          await _pumpWidgetAt(
            tester,
            surface,
            ProviderScope(
              overrides: [
                profileAboutDataProvider(
                  'about-user',
                ).overrideWith((ref) async => data),
              ],
              child: _surface(
                surface: surface,
                scale: scale,
                subject: const ProfileAboutTab(userId: 'about-user'),
              ),
            ),
            settle: true,
          );

          // The proven failure: these rows used to own bare unconstrained
          // Rows and overflowed. They must now pass with NO drained
          // exception — unlike the address gate, nothing is drained here.
          expect(
            tester.takeException(),
            isNull,
            reason: 'Profile About metadata overflow at ${width}dp @scale $scale',
          );
          expect(
            find.byType(MetadataView),
            findsNWidgets(2),
            reason: 'join date + last active must both use the authority',
          );
          final Finder views = find.byType(MetadataView);
          for (var i = 0; i < 2; i++) {
            _expectInsideSurface(tester, surface, views.at(i));
            _expectCompactStrategy(tester, scope: views.at(i));
          }
        }
      }
    });
  });

  group('Login sessions consumer — timestamp proof', () {
    testWidgets('session timestamp row flows through the compact authority', (
      tester,
    ) async {
      for (final double width in _widths) {
        for (final double scale in _scales) {
          final Size surface = Size(width, _surfaceHeight);
          await _pumpWidgetAt(
            tester,
            surface,
            ProviderScope(
              overrides: [
                authRepositoryProvider.overrideWithValue(
                  _FakeAuthRepository(),
                ),
              ],
              child: _surface(
                surface: surface,
                scale: scale,
                subject: const LoginSessionsScreen(),
              ),
            ),
            settle: true,
          );

          expect(
            tester.takeException(),
            isNull,
            reason: 'session timestamp overflow at ${width}dp @scale $scale',
          );
          expect(
            find.byType(MetadataView),
            findsOneWidget,
            reason: 'the session date row must use the authority',
          );
          _expectInsideSurface(tester, surface, find.byType(MetadataView));
          _expectCompactStrategy(tester);
        }
      }
    });
  });

  group('Follow UserCard consumer — count/stat proof', () {
    testWidgets('followers/following line flows through the compact authority', (
      tester,
    ) async {
      for (final double width in _widths) {
        for (final double scale in _scales) {
          final Size surface = Size(width, _surfaceHeight);
          await _pumpWidgetAt(
            tester,
            surface,
            _harness(
              surface: surface,
              scale: scale,
              subject: UserCard(
                user: _followUser(),
                showFollowButton: false,
              ),
            ),
          );

          expect(
            tester.takeException(),
            isNull,
            reason: 'follow counts overflow at ${width}dp @scale $scale',
          );
          expect(
            find.byType(MetadataText),
            findsOneWidget,
            reason: 'the follow counts line must use the authority',
          );
          _expectInsideSurface(tester, surface, find.byType(MetadataText));
          _expectCompactStrategy(tester);
        }
      }
    });
  });
}
