import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/catalog/shared/shared.dart';

Widget _host({
  required int itemCount,
  bool isLoading = false,
  Object? error,
  List<Widget>? children,
}) {
  return MaterialApp(
    home: Scaffold(
      body: CustomScrollView(
        slivers: [
          CommerceMarketplaceGrid(
            itemCount: itemCount,
            isLoading: isLoading,
            error: error,
            itemBuilder: (context, index) {
              final content = children == null
                  ? Center(child: Text('item $index'))
                  : children[index];
              return Container(
                key: ValueKey('grid-item-$index'),
                color: Colors.transparent,
                alignment: Alignment.center,
                child: content,
              );
            },
          ),
        ],
      ),
    ),
  );
}

Widget _cardHost({required ThemeData theme, required Widget child}) {
  return MaterialApp(
    theme: theme,
    home: Scaffold(
      body: ListView(
        children: [Padding(padding: const EdgeInsets.all(16), child: child)],
      ),
    ),
  );
}

class _FailingImageHttpClient implements HttpClient {
  @override
  Future<HttpClientRequest> getUrl(Uri url) async {
    return _FailingImageHttpClientRequest();
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _FailingImageHttpClientRequest implements HttpClientRequest {
  @override
  Future<HttpClientResponse> close() async {
    return _FailingImageHttpClientResponse();
  }

  @override
  bool bufferOutput = true;
  @override
  bool followRedirects = true;
  @override
  int maxRedirects = 5;
  @override
  bool persistentConnection = true;
  @override
  int contentLength = -1;
  @override
  Encoding encoding = utf8;

  @override
  HttpHeaders get headers => _NoopHttpHeaders();

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _FailingImageHttpClientResponse extends StreamView<List<int>>
    implements HttpClientResponse {
  _FailingImageHttpClientResponse() : super(const Stream<List<int>>.empty());

  @override
  int get statusCode => HttpStatus.notFound;

  @override
  int get contentLength => 0;

  @override
  HttpHeaders get headers => _NoopHttpHeaders();

  @override
  HttpClientResponseCompressionState get compressionState =>
      HttpClientResponseCompressionState.notCompressed;

  @override
  bool get isRedirect => false;

  @override
  bool get persistentConnection => false;

  @override
  String get reasonPhrase => 'Not Found';

  @override
  List<RedirectInfo> get redirects => const [];

  @override
  List<Cookie> get cookies => const [];

  @override
  X509Certificate? get certificate => null;

  @override
  HttpConnectionInfo? get connectionInfo => null;

  @override
  Future<Socket> detachSocket() => throw UnimplementedError();

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _NoopHttpHeaders implements HttpHeaders {
  @override
  void add(String name, Object value, {bool preserveHeaderCase = false}) {}

  @override
  void removeAll(String name) {}

  @override
  void set(String name, Object value, {bool preserveHeaderCase = false}) {}

  @override
  List<String>? operator [](String name) => null;

  @override
  String? value(String name) => null;

  @override
  void forEach(void Function(String name, List<String> values) action) {}

  @override
  String? get host => null;

  @override
  DateTime? get date => null;

  @override
  set host(String? value) {}

  @override
  set date(DateTime? value) {}

  @override
  bool get chunkedTransferEncoding => false;

  @override
  set chunkedTransferEncoding(bool value) {}

  @override
  int get contentLength => -1;

  @override
  set contentLength(int value) {}

  @override
  ContentType? get contentType => null;

  @override
  set contentType(ContentType? value) {}

  HttpConnectionInfo? get connectionInfo => null;

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

void main() {
  test('normalizeMarketplaceMediaReference strips transient query values', () {
    final normalized = normalizeMarketplaceMediaReference(
      'https://cdn.example.com/media/koi.jpg?X-Amz-Signature=abc&token=xyz&variant=thumb',
    );

    expect(normalized, 'media/koi.jpg?variant=thumb');
  });

  test('marketplaceMediaLogicalKey is stable across signed URL churn', () {
    final first = marketplaceMediaLogicalKey(
      entityType: 'listing',
      entityId: 'listing-1',
      mediaReference:
          'https://cdn.example.com/media/koi.jpg?X-Amz-Signature=abc&variant=thumb',
      position: 0,
    );
    final second = marketplaceMediaLogicalKey(
      entityType: 'listing',
      entityId: 'listing-1',
      mediaReference:
          'https://cdn.example.com/media/koi.jpg?X-Amz-Signature=updated&variant=thumb',
      position: 0,
    );

    expect(first, second);
  });

  testWidgets('CommerceMarketplaceGrid renders two columns on phone widths', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(500, 800));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    await tester.pumpWidget(_host(itemCount: 4));
    await tester.pumpAndSettle();

    final item0 = find.text('item 0');
    final item1 = find.text('item 1');
    final item2 = find.text('item 2');

    expect(tester.getTopLeft(item0).dy, tester.getTopLeft(item1).dy);
    expect(
      tester.getTopLeft(item2).dy,
      greaterThan(tester.getTopLeft(item0).dy),
    );
  });

  testWidgets('CommerceMarketplaceGrid renders two columns on tablet widths', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(700, 800));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    await tester.pumpWidget(_host(itemCount: 5));
    await tester.pumpAndSettle();

    final item0 = find.text('item 0');
    final item1 = find.text('item 1');
    final item2 = find.text('item 2');

    expect(tester.getTopLeft(item0).dy, tester.getTopLeft(item1).dy);
    await tester.scrollUntilVisible(item2, 400);
    expect(item2, findsOneWidget);
  });

  testWidgets('CommerceMarketplaceGrid renders two columns on desktop widths', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(1200, 800));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    await tester.pumpWidget(_host(itemCount: 6));
    await tester.pumpAndSettle();

    final item0 = find.text('item 0');
    final item1 = find.text('item 1');
    final item2 = find.text('item 2');
    final item4 = find.text('item 4');

    expect(tester.getTopLeft(item0).dy, tester.getTopLeft(item1).dy);
    await tester.scrollUntilVisible(item2, 400);
    expect(item2, findsOneWidget);
    await tester.scrollUntilVisible(item4, 400);
    expect(item4, findsOneWidget);
  });

  testWidgets(
    'CommerceMarketplaceGrid surfaces loading empty and error states',
    (tester) async {
      await tester.pumpWidget(_host(itemCount: 0, isLoading: true));
      await tester.pump();
      expect(find.byType(CircularProgressIndicator), findsOneWidget);

      await tester.pumpWidget(_host(itemCount: 0));
      await tester.pumpAndSettle();
      expect(find.text('Belum ada item untuk ditampilkan'), findsOneWidget);

      await tester.pumpWidget(_host(itemCount: 0, error: StateError('boom')));
      await tester.pumpAndSettle();
      expect(find.text('Data belum bisa dimuat.'), findsOneWidget);
      expect(find.textContaining('boom'), findsOneWidget);
    },
  );

  testWidgets(
    'CommerceMarketplaceCardShell keeps the compact 4:5 media frame',
    (tester) async {
      await tester.binding.setSurfaceSize(const Size(1200, 1800));
      addTearDown(() => tester.binding.setSurfaceSize(null));

      await tester.pumpWidget(
        _cardHost(
          theme: ThemeData.light(),
          child: CommerceMarketplaceCardShell(
            onTap: () {},
            semanticLabel: 'Showa Koi 30cm',
            media: CommerceMarketplaceCardMedia(
              imageUrl: null,
              fallback: const SizedBox.shrink(),
            ),
            badges: const [
              CommerceMarketplaceCardBadge(label: 'Listing', compact: true),
            ],
            title: 'Showa Koi 30cm',
            value: const CommerceMarketplaceCardValue(
              value: 'Rp 1.500.000',
              caption: 'Harga',
              compact: true,
            ),
            metadata: const Text('Metadata', key: ValueKey('metadata')),
          ),
        ),
      );

      final aspectRatio = tester.widget<AspectRatio>(find.byType(AspectRatio));
      expect(aspectRatio.aspectRatio, 4 / 5);

      final title = tester.widget<Text>(find.text('Showa Koi 30cm'));
      expect(title.maxLines, 2);

      expect(find.text('Metadata'), findsOneWidget);
      expect(find.byType(CommerceMarketplaceCardBadge), findsOneWidget);
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets('CommerceMarketplaceCardShell renders in light and dark themes', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(1200, 1800));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    for (final theme in [ThemeData.light(), ThemeData.dark()]) {
      await tester.pumpWidget(
        _cardHost(
          theme: theme,
          child: CommerceMarketplaceCardShell(
            onTap: () {},
            semanticLabel: 'theme check',
            media: CommerceMarketplaceCardMedia(
              imageUrl: null,
              fallback: const SizedBox.shrink(),
            ),
            title: 'Theme Check',
            value: const CommerceMarketplaceCardValue(value: 'Rp 1.000.000'),
          ),
        ),
      );

      expect(find.text('Theme Check'), findsOneWidget);
      expect(tester.takeException(), isNull);
    }
  });

  testWidgets('CommerceMarketplaceCardShell invokes its navigation callback', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(1200, 1800));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    var tapCount = 0;

    await tester.pumpWidget(
      _cardHost(
        theme: ThemeData.light(),
        child: CommerceMarketplaceCardShell(
          onTap: () => tapCount++,
          semanticLabel: 'tap me',
          media: CommerceMarketplaceCardMedia(
            imageUrl: null,
            fallback: const SizedBox.shrink(),
          ),
          title: 'Tap Me',
          value: const CommerceMarketplaceCardValue(value: 'Rp 900.000'),
        ),
      ),
    );

    await tester.ensureVisible(find.text('Tap Me'));
    await tester.tap(find.text('Tap Me'));
    await tester.pump();

    expect(tapCount, 1);
  });

  testWidgets(
    'CommerceMarketplaceCardShell fits the narrow compact home shelf',
    (tester) async {
      await tester.binding.setSurfaceSize(const Size(168, 280));
      addTearDown(() => tester.binding.setSurfaceSize(null));

      await tester.pumpWidget(
        _cardHost(
          theme: ThemeData.light(),
          child: MediaQuery(
            data: const MediaQueryData(textScaler: TextScaler.linear(1.3)),
            child: SizedBox(
              width: 168,
              height: 280,
              child: CommerceMarketplaceCardShell(
                compact: true,
                onTap: () {},
                semanticLabel: 'Showa Koi 30cm',
                contentPadding: const EdgeInsets.all(10),
                media: CommerceMarketplaceCardMedia(
                  imageUrl: null,
                  aspectRatio: 1,
                  fallback: const SizedBox.shrink(),
                ),
                badges: const [
                  CommerceMarketplaceCardBadge(label: 'Listing', compact: true),
                ],
                title: 'Showa Koi 30cm',
                value: const CommerceMarketplaceCardValue(
                  value: 'Rp 12.500.000',
                  compact: true,
                ),
              ),
            ),
          ),
        ),
      );

      await tester.pumpAndSettle();

      expect(find.text('Showa Koi 30cm'), findsOneWidget);
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets('CommerceMarketplaceCardMedia falls back when media is missing', (
    tester,
  ) async {
    await tester.pumpWidget(
      _cardHost(
        theme: ThemeData.light(),
        child: CommerceMarketplaceCardMedia(
          imageUrl: null,
          fallback: const Text('Missing media'),
        ),
      ),
    );

    expect(find.text('Missing media'), findsOneWidget);
  });

  testWidgets(
    'CommerceMarketplaceCardMedia falls back when the network image fails',
    (tester) async {
      await HttpOverrides.runZoned(() async {
        await tester.pumpWidget(
          _cardHost(
            theme: ThemeData.light(),
            child: CommerceMarketplaceCardMedia(
              imageUrl: 'https://cdn.example.com/fail.jpg',
              fallback: const Text('Failed media'),
            ),
          ),
        );
        await tester.pumpAndSettle();

        expect(find.text('Failed media'), findsOneWidget);
      }, createHttpClient: (_) => _FailingImageHttpClient());
    },
  );

  test('shared marketplace primitives do not depend on providers', () {
    final source = File(
      'lib/domains/commerce/catalog/shared/presentation/widgets/commerce_marketplace_primitives.dart',
    ).readAsStringSync();

    expect(source, isNot(contains('WidgetRef')));
    expect(source, isNot(contains('Consumer')));
    expect(source, isNot(contains('ref.')));
  });
}
