import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/shared/widgets/empty_state.dart';

/// CANONICAL contract for the empty-state foundation.
///
/// SEMANTIC OWNERSHIP: Empty = the request SUCCEEDED and the collection has
/// zero items. It is never Loading (spinner / hourglass) and never Error
/// (error icon / retry) — those live in `LoadingIndicator` and
/// `PageErrorState`. This test locks that purity on behavior: the surface
/// renders the collection-empty vocabulary and nothing else.

Future<void> _pump(WidgetTester tester, Widget child) {
  return tester.pumpWidget(MaterialApp(home: Scaffold(body: child)));
}

void main() {
  group('EmptyState — successful collection empty', () {
    testWidgets('renders the title and the supporting subtitle', (
      tester,
    ) async {
      await _pump(
        tester,
        const EmptyState(
          title: 'Belum ada pesanan',
          subtitle: 'Pesanan kamu akan muncul di sini',
        ),
      );

      expect(find.text('Belum ada pesanan'), findsOneWidget);
      expect(find.text('Pesanan kamu akan muncul di sini'), findsOneWidget);
    });

    testWidgets('renders the default empty icon in the neutral ink', (
      tester,
    ) async {
      await _pump(tester, const EmptyState(title: 'Belum ada pesanan'));

      final icon = tester.widget<Icon>(find.byIcon(Icons.inbox_outlined));
      expect(icon.size, isNotNull);
      // Empty is never painted with the error role.
      final scheme = Theme.of(tester.element(find.byType(EmptyState)));
      expect(icon.color, scheme.colorScheme.onSurfaceVariant);
      expect(icon.color, isNot(scheme.colorScheme.error));
    });

    testWidgets('renders a caller-supplied icon over the default', (
      tester,
    ) async {
      await _pump(
        tester,
        const EmptyState(
          title: 'Belum ada pesanan',
          icon: Icons.shopping_bag_outlined,
        ),
      );

      expect(find.byIcon(Icons.shopping_bag_outlined), findsOneWidget);
      expect(find.byIcon(Icons.inbox_outlined), findsNothing);
    });

    testWidgets(
      'renders no error surface, no loading state and no retry action',
      (tester) async {
        await _pump(
          tester,
          const EmptyState(
            title: 'Belum ada pesanan',
            subtitle: 'Pesanan kamu akan muncul di sini',
          ),
        );

        // Error state vocabulary (PageErrorState owns it).
        expect(find.byIcon(Icons.error_outline), findsNothing);
        expect(find.text('Terjadi Kesalahan'), findsNothing);
        expect(find.text('Coba Lagi'), findsNothing);
        expect(find.text('Try Again'), findsNothing);
        expect(find.byType(ElevatedButton), findsNothing);

        // Loading state vocabulary (LoadingIndicator owns it).
        expect(find.byType(CircularProgressIndicator), findsNothing);
        expect(find.byIcon(Icons.hourglass_empty_outlined), findsNothing);
      },
    );

    testWidgets('renders the caller action only when label and callback are '
        'supplied', (tester) async {
      await _pump(
        tester,
        const EmptyState(title: 'Belum ada promosi', actionLabel: null),
      );
      expect(find.byType(FilledButton), findsNothing);

      await _pump(
        tester,
        const EmptyState(
          title: 'Belum ada promosi',
          actionLabel: 'Buat Promosi',
          onAction: _noop,
        ),
      );
      expect(find.widgetWithText(FilledButton, 'Buat Promosi'), findsOneWidget);
    });
  });

  group('EmptyState — ONE primary action (first-use & search/filter)', () {
    testWidgets('first-use: the single primary action fires once', (
      tester,
    ) async {
      var taps = 0;
      await _pump(
        tester,
        EmptyState(
          title: 'Belum Ada For Sale',
          subtitle: 'Mulai buat For Sale untuk menjual produk Anda',
          actionLabel: 'Buat For Sale',
          onAction: () => taps++,
        ),
      );

      // Exactly one primary action — never two.
      expect(find.byType(FilledButton), findsOneWidget);
      await tester.tap(find.byType(FilledButton));
      expect(taps, 1);
    });

    testWidgets('search/filter: the reset action fires', (tester) async {
      var resets = 0;
      await _pump(
        tester,
        EmptyState(
          title: 'Tidak Ada Hasil',
          subtitle: 'Coba kata kunci atau filter yang lain.',
          actionLabel: 'Atur Ulang',
          onAction: () => resets++,
        ),
      );

      expect(find.byType(FilledButton), findsOneWidget);
      await tester.tap(find.widgetWithText(FilledButton, 'Atur Ulang'));
      expect(resets, 1);
    });
  });

  group('EmptyState — accessibility', () {
    testWidgets('state text and the action are discoverable semantics', (
      tester,
    ) async {
      final handle = tester.ensureSemantics();

      await _pump(
        tester,
        const EmptyState(
          title: 'Belum ada pesanan',
          subtitle: 'Mulai berbelanja dari koleksi Koi terbaik',
          actionLabel: 'Jelajahi Marketplace',
          onAction: _noop,
        ),
      );

      // The semantic nodes a screen reader actually sees, in traversal
      // order (the state container merges title + message into one node).
      final nodes = tester.semantics.simulatedAccessibilityTraversal().toList();

      final stateLabels = nodes
          .map((node) => node.label)
          .where((label) => label.contains('Belum ada pesanan'))
          .toList();
      expect(stateLabels, hasLength(1));
      expect(
        stateLabels.single,
        contains('Mulai berbelanja dari koleksi Koi terbaik'),
      );

      // The CTA is a semantic BUTTON with an accessible label.
      final buttonNodes = nodes
          .where((node) => node.label == 'Jelajahi Marketplace')
          .toList();
      expect(buttonNodes, hasLength(1));
      expect(
        buttonNodes.single.getSemanticsData().flagsCollection.isButton,
        isTrue,
      );

      handle.dispose();
    });

    testWidgets('one semantic container, no live region (static state)', (
      tester,
    ) async {
      await _pump(tester, const EmptyState(title: 'Belum ada pesanan'));

      // The state is wrapped in one semantic container…
      expect(
        find.descendant(
          of: find.byType(EmptyState),
          matching: find.byWidgetPredicate(
            (widget) => widget is Semantics && widget.container == true,
          ),
        ),
        findsOneWidget,
      );
      // …and, unlike PageErrorState, it does NOT announce itself.
      expect(
        find.byWidgetPredicate(
          (widget) => widget is Semantics && widget.properties.liveRegion == true,
        ),
        findsNothing,
      );
    });
  });
}

void _noop() {}
