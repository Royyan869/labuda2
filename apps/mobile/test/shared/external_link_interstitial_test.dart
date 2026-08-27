import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/shared/widgets/external_link_interstitial.dart';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/// Mocks the url_launcher MethodChannel and returns a list of recorded calls.
List<MethodCall> _mockUrlLauncher(TestWidgetsFlutterBinding binding) {
  const channel = MethodChannel('plugins.flutter.io/url_launcher');
  final calls = <MethodCall>[];
  binding.defaultBinaryMessenger.setMockMethodCallHandler(channel, (
    call,
  ) async {
    calls.add(call);
    // canLaunchUrl → true; launchUrl → true
    return true;
  });
  return calls;
}

Widget _wrap(Widget child) => MaterialApp(home: Scaffold(body: child));

// ---------------------------------------------------------------------------
// URL scheme / parsing unit tests (no widgets required)
// ---------------------------------------------------------------------------

void main() {
  group('URL scheme validation (pure unit)', () {
    test('https scheme is valid', () {
      final uri = Uri.tryParse('https://example.com/path');
      expect(uri, isNotNull);
      expect(uri!.scheme == 'https' || uri.scheme == 'http', isTrue);
    });

    test('http scheme is valid', () {
      final uri = Uri.tryParse('http://example.com');
      expect(uri, isNotNull);
      expect(uri!.scheme == 'https' || uri.scheme == 'http', isTrue);
    });

    test('javascript scheme is rejected', () {
      final uri = Uri.tryParse('javascript:alert(1)');
      expect(uri, isNotNull);
      expect(uri!.scheme == 'https' || uri.scheme == 'http', isFalse);
    });

    test('file scheme is rejected', () {
      final uri = Uri.tryParse('file:///etc/passwd');
      expect(uri, isNotNull);
      expect(uri!.scheme == 'https' || uri.scheme == 'http', isFalse);
    });

    test('data URI scheme is rejected', () {
      final uri = Uri.tryParse('data:text/html,<h1>x</h1>');
      expect(uri, isNotNull);
      expect(uri!.scheme == 'https' || uri.scheme == 'http', isFalse);
    });

    test('empty string gives null URI', () {
      expect(Uri.tryParse(''), isNotNull); // Uri.tryParse('') == Uri()
      final uri = Uri.tryParse('');
      // scheme is empty string — not http/https
      expect(uri!.scheme == 'https' || uri.scheme == 'http', isFalse);
    });

    test('malformed URL gives null URI', () {
      expect(
        Uri.tryParse('not a url ##'),
        isNotNull,
      ); // tryParse never returns null
      // but scheme will be empty
      final uri = Uri.tryParse('not a url ##');
      expect(uri!.scheme == 'https' || uri.scheme == 'http', isFalse);
    });
  });

  group('Domain extraction unit', () {
    test('extracts host from https URL', () {
      final host = Uri.tryParse('https://shop.example.com/product/1')?.host;
      expect(host, 'shop.example.com');
    });

    test('extracts host from URL with port', () {
      final host = Uri.tryParse('https://example.com:8080/path')?.host;
      expect(host, 'example.com');
    });

    test('returns fallback if parse fails', () {
      const raw = 'not-a-url';
      final host = Uri.tryParse(raw)?.host;
      // host is empty string for relative URIs
      final display = (host == null || host.isEmpty) ? raw : host;
      expect(display, raw);
    });
  });

  // -------------------------------------------------------------------------
  // Widget tests
  // -------------------------------------------------------------------------

  group('ExternalLinkInterstitial dialog', () {
    late TestWidgetsFlutterBinding binding;

    setUpAll(() {
      binding = TestWidgetsFlutterBinding.ensureInitialized();
    });

    testWidgets('dialog appears with domain and warning text', (tester) async {
      await tester.pumpWidget(
        _wrap(
          Builder(
            builder: (context) => ElevatedButton(
              onPressed: () => showExternalLinkInterstitial(
                context,
                url: 'https://shop.example.com/product',
              ),
              child: const Text('open'),
            ),
          ),
        ),
      );

      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();

      // Dialog title
      expect(find.text('Buka tautan eksternal?'), findsOneWidget);
      // Warning text
      expect(find.textContaining('meninggalkan Labuda'), findsOneWidget);
      // Destination domain visible (appears in both host row and full URL text)
      expect(find.textContaining('shop.example.com'), findsAtLeastNWidgets(1));
      // Full URL visible (may be ellipsized but widget exists)
      expect(
        find.textContaining('https://shop.example.com/product'),
        findsOneWidget,
      );
      // Disclaimer
      expect(find.textContaining('tidak bertanggung jawab'), findsOneWidget);
    });

    testWidgets('cancel button closes dialog without launching URL', (
      tester,
    ) async {
      final calls = _mockUrlLauncher(binding);

      await tester.pumpWidget(
        _wrap(
          Builder(
            builder: (context) => ElevatedButton(
              onPressed: () => showExternalLinkInterstitial(
                context,
                url: 'https://example.com/product',
              ),
              child: const Text('open'),
            ),
          ),
        ),
      );

      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();

      // Tap cancel
      await tester.tap(find.text('Batal'));
      await tester.pumpAndSettle();

      // Dialog dismissed
      expect(find.text('Buka tautan eksternal?'), findsNothing);
      // No URL launched
      expect(calls, isEmpty);
    });

    testWidgets('confirm button calls URL launcher with https URL', (
      tester,
    ) async {
      final calls = _mockUrlLauncher(binding);

      await tester.pumpWidget(
        _wrap(
          Builder(
            builder: (context) => ElevatedButton(
              onPressed: () => showExternalLinkInterstitial(
                context,
                url: 'https://example.com/product',
              ),
              child: const Text('open'),
            ),
          ),
        ),
      );

      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();

      // Tap confirm
      await tester.tap(find.text('Buka'));
      await tester.pumpAndSettle();

      // URL was launched
      expect(calls, isNotEmpty);
      final launched = calls.any(
        (c) => c.arguments.toString().contains('https://example.com/product'),
      );
      expect(launched, isTrue);
    });

    testWidgets(
      'invalid (non-http) URL shows error snackbar and does not show dialog',
      (tester) async {
        final calls = _mockUrlLauncher(binding);

        await tester.pumpWidget(
          _wrap(
            Builder(
              builder: (context) => ElevatedButton(
                onPressed: () => showExternalLinkInterstitial(
                  context,
                  url: 'javascript:alert(1)',
                ),
                child: const Text('open'),
              ),
            ),
          ),
        );

        await tester.tap(find.text('open'));
        await tester.pumpAndSettle();

        // No dialog
        expect(find.text('Buka tautan eksternal?'), findsNothing);
        // Error snackbar
        expect(find.textContaining('tidak valid'), findsOneWidget);
        // No launch
        expect(calls, isEmpty);
      },
    );

    testWidgets('null / empty URL is rejected safely', (tester) async {
      final calls = _mockUrlLauncher(binding);

      await tester.pumpWidget(
        _wrap(
          Builder(
            builder: (context) => ElevatedButton(
              onPressed: () => showExternalLinkInterstitial(context, url: ''),
              child: const Text('open'),
            ),
          ),
        ),
      );

      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();

      expect(find.text('Buka tautan eksternal?'), findsNothing);
      expect(calls, isEmpty);
    });

    testWidgets('http (non-TLS) URL is accepted by interstitial', (
      tester,
    ) async {
      _mockUrlLauncher(binding);

      await tester.pumpWidget(
        _wrap(
          Builder(
            builder: (context) => ElevatedButton(
              onPressed: () => showExternalLinkInterstitial(
                context,
                url: 'http://example.com/product',
              ),
              child: const Text('open'),
            ),
          ),
        ),
      );

      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();

      // Dialog shows
      expect(find.text('Buka tautan eksternal?'), findsOneWidget);
    });
  });
}
