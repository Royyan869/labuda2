// ICON / ACTION INTERACTION FOUNDATION — convergence proof (Pass 2).
//
// Locks the Owner decisions applied in this pass:
//  A) a disabled icon action uses the ONE neutral disabled ink
//     (`onSurfaceVariant`), owned by `iconButtonTheme` — never a local alpha;
//  B) every icon-only user action exposes a meaningful accessibility name;
//  C) the content engagement row (like / comment / share) has ONE producer,
//     `ContentEngagementActions`, consumed by both feed and detail;
//  D) no generic icon-action wrapper (`AppIconButton`) exists;
//  E) the geometry census accurately detects raw icon sizes.
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/social/content/presentation/widgets/content_engagement_actions.dart';
import 'package:hishumi/domains/user/identity/authentication/presentation/providers/auth_controller.dart';

import '../../support/geometry_authority_gate.dart';
import '../../support/theme_authority_gate.dart';

class _UnauthenticatedController extends AuthController {
  @override
  AuthState build() => const AuthState.unauthenticated();
}

String _src(String path) => File(path).readAsStringSync();

void main() {
  // ---------------------------------------------------------------------------
  // A) Disabled icon action authority
  // ---------------------------------------------------------------------------
  group('disabled icon action authority', () {
    for (final (name, theme) in <(String, ThemeData)>[
      ('light', AppTheme.lightTheme),
      ('dark', AppTheme.darkTheme),
    ]) {
      test('$name icon-button theme owns the neutral disabled ink', () {
        final scheme = theme.colorScheme;
        final style = theme.iconButtonTheme.style;
        expect(style, isNotNull, reason: 'the icon disabled language is owned');
        final disabled = style!.foregroundColor!.resolve(
          const <WidgetState>{WidgetState.disabled},
        );
        expect(
          disabled,
          scheme.onSurfaceVariant,
          reason: '$name disabled icon action is the canonical neutral ink',
        );
        // Negative proof: never a faded brand tone.
        expect(disabled, isNot(scheme.primary));
        // Enabled ink is NOT owned here — each site's explicit colour or the
        // scheme default still applies.
        expect(
          style.foregroundColor!.resolve(const <WidgetState>{}),
          isNull,
          reason: '$name enabled icon ink stays site-owned',
        );
      });
    }

    testWidgets('a disabled IconButton renders the neutral disabled ink', (
      tester,
    ) async {
      final theme = AppTheme.lightTheme;
      await tester.pumpWidget(
        MaterialApp(
          theme: theme,
          home: Scaffold(
            body: IconButton(
              onPressed: null,
              icon: const Icon(Icons.close),
            ),
          ),
        ),
      );
      final rich = tester.widget<RichText>(
        find
            .descendant(
              of: find.byType(IconButton),
              matching: find.byType(RichText),
            )
            .first,
      );
      final span = rich.text as TextSpan;
      expect(span.style?.color, theme.colorScheme.onSurfaceVariant);
    });

    test('no arbitrary local disabled icon alpha remains', () {
      const banned = <String>[
        'onSurfaceVariant.withValues(alpha: 0.38)',
        'onSurface.withValues(alpha: 0.38)',
        'onSurface.withOpacity(0.38)',
      ];
      final offenders = <String>[];
      for (final path in themeAuthorityDartFiles()) {
        if (themeAuthorityFiles.contains(path)) continue;
        final src = File(path).readAsStringSync();
        for (final needle in banned) {
          if (src.contains(needle)) offenders.add('$path: $needle');
        }
      }
      expect(offenders, isEmpty, reason: offenders.join('\n'));
    });
  });

  // ---------------------------------------------------------------------------
  // B) Accessibility names for icon-only actions
  // ---------------------------------------------------------------------------
  group('icon-only accessibility', () {
    testWidgets('representative icon-only actions expose their names', (
      tester,
    ) async {
      final handle = tester.ensureSemantics();
      await tester.pumpWidget(
        MaterialApp(
          theme: AppTheme.lightTheme,
          home: Scaffold(
            body: Row(
              children: [
                IconButton(
                  onPressed: () {},
                  icon: const Icon(
                    Icons.arrow_back,
                    semanticLabel: 'Kembali',
                  ),
                ),
                IconButton(
                  onPressed: () {},
                  icon: const Icon(
                    Icons.delete_outline,
                    semanticLabel: 'Hapus',
                  ),
                ),
                IconButton(
                  onPressed: () {},
                  icon: const Icon(Icons.clear, semanticLabel: 'Bersihkan'),
                ),
              ],
            ),
          ),
        ),
      );
      expect(find.bySemanticsLabel('Kembali'), findsOneWidget);
      expect(find.bySemanticsLabel('Hapus'), findsOneWidget);
      expect(find.bySemanticsLabel('Bersihkan'), findsOneWidget);
      handle.dispose();
    });

    test('the canonical engagement actions expose their names', () {
      final src = _src(
        'lib/domains/social/content/presentation/widgets/'
        'content_engagement_actions.dart',
      );
      expect(src.contains("'Suka'"), isTrue);
      expect(src.contains("'Komentar'"), isTrue);
      expect(src.contains("'Bagikan'"), isTrue);
      expect(src.contains('Semantics('), isTrue);
    });
  });

  // ---------------------------------------------------------------------------
  // C) Engagement producer convergence
  // ---------------------------------------------------------------------------
  group('engagement action producer convergence', () {
    test('both feed and detail consume the one producer', () {
      for (final path in const <String>[
        'lib/features/home/presentation/providers/feed_renderers.dart',
        'lib/domains/social/content/presentation/screens/'
            'content_detail_screen.dart',
      ]) {
        final src = _src(path);
        expect(
          src.contains('ContentEngagementActions('),
          isTrue,
          reason: '$path must use the canonical engagement producer',
        );
        // No second like/comment/share row is spelled here.
        expect(
          src.contains('Icons.favorite_border'),
          isFalse,
          reason: '$path must not spell its own like glyph',
        );
        expect(
          src.contains('Icons.chat_bubble_outline') &&
              src.contains('Icons.share_outlined'),
          isFalse,
          reason: '$path must not spell its own engagement row',
        );
        // No second content-like behaviour.
        expect(
          src.contains('toggleLike') || src.contains('pushOptimisticLikeStats'),
          isFalse,
          reason: '$path must not re-implement the like mutation',
        );
      }
    });

    test('one content-like authority remains', () {
      final handler = _src(
        'lib/domains/social/content/presentation/utils/content_like_handlers.dart',
      );
      expect(handler.contains('toggleLike'), isTrue);
      expect(handler.contains('pushOptimisticLikeStats'), isTrue);
      // The handler binds identity, not a Content instance.
      expect(handler.contains('required this.targetId'), isTrue);
      expect(handler.contains('required this.targetOwnerId'), isTrue);
    });

    testWidgets('the engagement row renders like/comment/share for a guest', (
      tester,
    ) async {
      final handle = tester.ensureSemantics();
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            authControllerProvider.overrideWith(_UnauthenticatedController.new),
          ],
          child: MaterialApp(
            theme: AppTheme.lightTheme,
            home: Scaffold(
              body: ContentEngagementActions(
                targetId: 'content-1',
                targetOwnerId: 'owner-1',
                likeCount: 3,
                commentCount: 2,
                onComment: () {},
                onShare: () {},
              ),
            ),
          ),
        ),
      );
      expect(find.bySemanticsLabel('Suka, 3'), findsOneWidget);
      expect(find.bySemanticsLabel('Komentar, 2'), findsOneWidget);
      expect(find.bySemanticsLabel('Bagikan'), findsOneWidget);
      handle.dispose();
    });
  });

  // ---------------------------------------------------------------------------
  // D) No generic wrapper
  // ---------------------------------------------------------------------------
  group('no generic icon-action wrapper', () {
    test('no AppIconButton / IconAction symbol exists in lib', () {
      const banned = <String>[
        'AppIconButton',
        'class IconAction',
        'BaseIconButton',
        'PrimaryIconButton',
        'SecondaryIconButton',
        'class ActionIcon',
      ];
      final offenders = <String>[];
      for (final entity in Directory('lib').listSync(recursive: true)) {
        if (entity is! File || !entity.path.endsWith('.dart')) continue;
        final src = entity.readAsStringSync();
        for (final needle in banned) {
          if (src.contains(needle)) offenders.add('${entity.path}: $needle');
        }
      }
      expect(offenders, isEmpty, reason: offenders.join('\n'));
    });
  });

  // ---------------------------------------------------------------------------
  // E) Geometry census detects raw icon sizes
  // ---------------------------------------------------------------------------
  group('geometry census detects icon sizes', () {
    test('an identifier tail no longer hides a raw icon size', () {
      // The exact regression the lenient detector missed: `_getStatusIcon(`
      // made the census treat the block as nested and skip it.
      const probe = '''
Icon(
  _getStatusIcon(),
  size: 14,
  color: c,
)
''';
      final found = geometryViolationsIn(probe, path: 'probe.dart');
      expect(found['iconSize'], hasLength(1));
    });

    test('the icon-size backlog is zero app-wide', () {
      final census = geometryCensus(dir: 'lib');
      expect(census['iconSize'], 0);
    });
  });
}
