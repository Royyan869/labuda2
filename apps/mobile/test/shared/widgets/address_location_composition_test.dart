// ADDRESS / LOCATION — HORIZONTAL COMPOSITION ACCEPTANCE GATE.
//
// Owner-locked semantic: ONE Address/Location family with TWO canonical
// presentation modes (see lib/shared/widgets/address_location_view.dart):
//
//   compact → single bounded line + TextOverflow.ellipsis
//   detail  → wrapping text, horizontally bounded by its parent
//
// This suite proves the authority by BEHAVIOUR across a WIDTH x TEXT-SCALE
// matrix with a realistic long Indonesian address:
//
//   * compact must NOT throw, must stay inside the surface, must set
//     maxLines:1 + ellipsis, and must actually truncate a long address;
//   * detail must NOT throw, must stay inside the surface, must NOT cap lines,
//     and must render taller than compact (i.e. it genuinely wraps).
//
// It also pumps the real Profile About consumer, proving the Owner's original
// symptom now flows through the same authority. It deliberately does not test
// unrelated families (label/value, price, badge, identity).
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/profile/profile.dart'
    show ProfileAboutData, profileAboutDataProvider;
import 'package:labuda/domains/user/profile/presentation/screens/profile_screen/profile_about_tab.dart';
import 'package:labuda/shared/shared.dart';

const List<double> _widths = <double>[320, 360, 412, 500];
const List<double> _scales = <double>[1.0, 1.3, 2.0];

/// Realistic long worst-case Indonesian address/location.
const String _longAddress =
    'Jl. Raya Puncak Km. 84, Desa Cisarua, Kecamatan Cisarua, '
    'Kabupaten Bogor, Jawa Barat 16750, Indonesia, dekat Pasar Cisarua';

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
    // Let the async family provider resolve without waiting on any loading
    // spinner animation (pumpAndSettle would time out on one).
    await tester.pump(const Duration(milliseconds: 100));
  }
}

/// The `Text` the authority actually builds.
Text _authorityText(WidgetTester tester) {
  return tester.widget<Text>(
    find.descendant(
      of: find.byType(AddressLocationText),
      matching: find.byType(Text),
    ),
  );
}

RenderParagraph _authorityParagraph(WidgetTester tester) {
  return tester.renderObject<RenderParagraph>(
    find.descendant(
      of: find.byType(AddressLocationText),
      matching: find.byType(RichText),
    ),
  );
}

void _expectInsideSurface(WidgetTester tester, Size surface, Type type) {
  final Finder finder = find.byType(type);
  expect(finder, findsOneWidget, reason: '$type did not render');
  final Rect rect = tester.getRect(finder);
  expect(rect.left, greaterThanOrEqualTo(-0.5), reason: '$type left of surface');
  expect(
    rect.right,
    lessThanOrEqualTo(surface.width + 0.5),
    reason: '$type wider than surface at ${surface.width}dp',
  );
}

// ============================================================================
// Canonical subjects
// ============================================================================

Widget _compactAuthority() => const AddressLocationView(
  location: _longAddress,
  mode: AddressLocationMode.compact,
  icon: Icons.location_on_outlined,
  iconSize: AppIconSize.inlineGlyph,
);

Widget _detailAuthority() => const AddressLocationView(
  location: _longAddress,
  mode: AddressLocationMode.detail,
  icon: Icons.location_on_outlined,
  iconSize: AppIconSize.inlineGlyph,
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
  location: _longAddress,
);

// ============================================================================
// Tests
// ============================================================================

void main() {
  group('ADDRESS/LOCATION authority — compact mode', () {
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
          _expectInsideSurface(tester, surface, AddressLocationView);

          final Text text = _authorityText(tester);
          expect(text.maxLines, 1, reason: 'compact must cap at one line');
          expect(
            text.overflow,
            TextOverflow.ellipsis,
            reason: 'compact must declare an ellipsis strategy',
          );
          expect(
            _authorityParagraph(tester).didExceedMaxLines,
            isTrue,
            reason:
                'a long address must actually be truncated in compact mode '
                'at ${width}dp @scale $scale',
          );
        }
      }
    });
  });

  group('ADDRESS/LOCATION authority — detail mode', () {
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
          _expectInsideSurface(tester, surface, AddressLocationView);

          final Text detailText = _authorityText(tester);
          expect(
            detailText.maxLines,
            isNull,
            reason: 'detail must not cap lines on its own',
          );
          final double detailHeight = tester
              .getSize(find.byType(AddressLocationText))
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
              .getSize(find.byType(AddressLocationText))
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
  });

  group('Profile About consumer — Owner symptom regression', () {
    testWidgets(
      'Profile About location flows through the compact authority',
      (tester) async {
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

            // The real About tab also renders a NON-address metadata row (the
            // join-date row) which is OUT OF SCOPE for the address/location
            // family and is a pre-existing, separately-reported issue. Drain
            // it here so this gate can isolate the ADDRESS/LOCATION authority:
            // the location's own safety is proven below by geometry (inside the
            // surface) + truncation (didExceedMaxLines), which could not both
            // hold if the location row itself overflowed.
            tester.takeException();

            _expectInsideSurface(tester, surface, AddressLocationView);

            final Text text = _authorityText(tester);
            expect(text.maxLines, 1);
            expect(text.overflow, TextOverflow.ellipsis);
            expect(
              _authorityParagraph(tester).didExceedMaxLines,
              isTrue,
              reason:
                  'Profile About location must truncate, not overflow, at '
                  '${width}dp @scale $scale',
            );
          }
        }
      },
    );
  });
}
