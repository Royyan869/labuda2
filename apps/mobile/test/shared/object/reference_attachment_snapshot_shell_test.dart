/// REFERENCE ATTACHMENT SNAPSHOT SHELL CONTRACT
///
/// Pins the shape of the fallback card a communication row renders when the
/// server has NOT resolved a canonical `resource_projection` for it:
/// - it renders the transport snapshot only (type badge, title, thumbnail);
/// - it renders NO money and NO availability claim — Commerce truth belongs to
///   the projection envelope, never to a cached snapshot;
/// - navigation is identity-only and stays available even when the snapshot's
///   flags say the target is gone (the target surface answers with truth and
///   stays fail-closed on its own).
///
/// Before the purge this card resolved live data per item through
/// `objectPreviewProvider`, i.e. one detail-endpoint call per shared message.
library;

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/shared/attachment/entities/share_reference.dart';
import 'package:hishumi/shared/object/presentation/widgets/object_preview_card.dart';

Widget _wrap(ShareReference reference, {VoidCallback? onTap}) {
  return MaterialApp(
    home: Scaffold(
      body: ObjectPreviewCard(reference: reference, onTap: onTap),
    ),
  );
}

/// Every `Text` string currently in the tree.
List<String> _renderedTexts(WidgetTester tester) {
  return tester
      .widgetList<Text>(find.byType(Text))
      .map((text) => text.data ?? '')
      .toList();
}

ShareReference _forSaleReference({
  String title = 'Kohaku 50cm',
  bool isAvailable = true,
  bool isSold = false,
  bool isDeleted = false,
}) {
  return ShareReference.forSale(
    forSaleId: 'sale-1',
    title: title,
    imageUrl: 'https://example.com/sale.jpg',
    isAvailable: isAvailable,
    isSold: isSold,
    isDeleted: isDeleted,
  );
}

void main() {
  testWidgets('renders the transport identity: type badge, title, thumbnail', (
    tester,
  ) async {
    await tester.pumpWidget(_wrap(_forSaleReference()));

    expect(find.text('Produk Dijual'), findsOneWidget);
    expect(find.text('Kohaku 50cm'), findsOneWidget);
    expect(find.byType(Image), findsOneWidget);
    expect(find.byIcon(Icons.chevron_right), findsOneWidget);
  });

  testWidgets(
    'a snapshot claiming the target is gone does not block navigation',
    (tester) async {
      var tapped = false;

      await tester.pumpWidget(
        _wrap(
          _forSaleReference(isAvailable: false, isDeleted: true),
          onTap: () => tapped = true,
        ),
      );

      await tester.tap(find.byType(InkWell));
      await tester.pump();

      expect(
        tapped,
        isTrue,
        reason:
            'navigation is identity-only: the target surface decides truth, '
            'so a stale snapshot flag never becomes a client-side verdict',
      );
    },
  );

  testWidgets('renders no money and no availability claim from the snapshot', (
    tester,
  ) async {
    await tester.pumpWidget(
      _wrap(
        ShareReference.forSale(
          forSaleId: 'sale-2',
          title: 'Sanke 45cm',
          isAvailable: false,
          isSold: true,
        ),
      ),
    );

    final texts = _renderedTexts(tester);
    expect(texts, contains('Sanke 45cm'));
    expect(
      texts.where((text) => text.contains('Rp')),
      isEmpty,
      reason: 'money comes only from the canonical projection envelope',
    );
    for (final claim in const ['Terjual', 'Tersedia', 'Ditarik', 'Dihapus']) {
      expect(
        texts,
        isNot(contains(claim)),
        reason: 'availability claims belong to the projection envelope',
      );
    }
  });

  testWidgets('resolver proof: the type caption is the theme role', (
    tester,
  ) async {
    // THE proof an analyzer cannot give. A `fontSize:` literal and a role can
    // render the same pixel TODAY and diverge on the next ladder retune; what
    // must hold is that the rendered style IS the theme's `labelSmall` —
    // caption/badge on the canonical mapping documented on AppTheme — with only
    // the call site's own decisions layered on top (semibold, brand colour).
    // Migration proves this per slice; a restated size would never show here.
    final theme = AppTheme.lightTheme;
    late TextTheme resolvedTheme;
    await tester.pumpWidget(
      MaterialApp(
        theme: theme,
        home: Builder(
          builder: (context) {
            // Read the role the way the widget does: `Theme.of()` merges the
            // englishLike-2021 geometry on top of the raw ThemeData, so
            // comparing against `theme.textTheme` would compare against a
            // colour/family-only style and pass on `null == null`.
            resolvedTheme = Theme.of(context).textTheme;
            return Scaffold(
              body: ObjectPreviewCard(reference: _forSaleReference()),
            );
          },
        ),
      ),
    );

    final role = resolvedTheme.labelSmall!;
    final resolved = tester.widget<Text>(find.text('Produk Dijual')).style!;

    expect(
      role.fontSize,
      isNotNull,
      reason: 'geometry must be resolved from the widget tree, not the raw theme',
    );
    expect(resolved.fontSize, role.fontSize, reason: 'size comes from the role');
    expect(
      resolved.height,
      role.height,
      reason: 'line height comes from the role',
    );
    expect(resolved.letterSpacing, role.letterSpacing);
    expect(resolved.fontWeight, FontWeight.w600, reason: "call site's weight");
    expect(
      resolved.color,
      theme.colorScheme.primary,
      reason: "call site's colour",
    );
  });
}
