import 'dart:async';
import 'dart:collection';
import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/user/profile/presentation/widgets/profile_cover.dart';
import 'package:labuda/shared/models/seller_identity_data.dart';
import 'package:labuda/shared/widgets/profile_avatar.dart';
import 'package:labuda/shared/widgets/seller_dual_avatar.dart';
import 'package:labuda/shared/widgets/stable_network_image.dart';

class _QueuedImageHttpClient implements HttpClient {
  _QueuedImageHttpClient(this._responders);

  final Map<String, Queue<_QueuedResponse>> _responders;
  final Map<String, int> requestCounts = <String, int>{};

  @override
  bool autoUncompress = true;
  @override
  Duration? connectionTimeout;
  @override
  Duration idleTimeout = const Duration(seconds: 15);
  @override
  int? maxConnectionsPerHost;
  @override
  String? userAgent;
  @override
  bool Function(X509Certificate, String, int)? badCertificateCallback;
  @override
  String Function(Uri)? findProxy;
  @override
  Future<bool> Function(Uri, String, String?)? authenticate;
  @override
  Future<bool> Function(String, int, String, String?)? authenticateProxy;

  @override
  Future<HttpClientRequest> getUrl(Uri url) async {
    final queue = _responders[url.toString()];
    if (queue == null || queue.isEmpty) {
      throw StateError('No responder for $url');
    }
    requestCounts.update(
      url.toString(),
      (value) => value + 1,
      ifAbsent: () => 1,
    );
    return _QueuedImageHttpClientRequest(queue.removeFirst());
  }

  @override
  noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _QueuedImageHttpClientRequest implements HttpClientRequest {
  _QueuedImageHttpClientRequest(this._responder);

  final _QueuedResponse _responder;

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
  Future<HttpClientResponse> close() async => _responder.response;

  @override
  noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _QueuedResponse {
  _QueuedResponse._(this.response, this.complete);

  final HttpClientResponse response;
  final void Function(List<int> bytes) complete;

  factory _QueuedResponse.pending() {
    final controller = StreamController<List<int>>();
    final response = _QueuedImageHttpClientResponse(controller.stream);
    return _QueuedResponse._(response, (bytes) {
      controller.add(bytes);
      controller.close();
    });
  }
}

class _QueuedImageHttpClientResponse extends StreamView<List<int>>
    implements HttpClientResponse {
  _QueuedImageHttpClientResponse(super.stream);

  @override
  int get statusCode => HttpStatus.ok;

  @override
  int get contentLength => -1;

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
  String get reasonPhrase => 'OK';

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
  noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _NoopHttpHeaders implements HttpHeaders {
  @override
  void add(String name, Object value, {bool preserveHeaderCase = false}) {}

  @override
  void set(String name, Object value, {bool preserveHeaderCase = false}) {}

  @override
  void remove(String name, Object value) {}

  @override
  String? value(String name) => null;

  @override
  noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

const List<int> _onePxPngBytes = <int>[
  0x89,
  0x50,
  0x4E,
  0x47,
  0x0D,
  0x0A,
  0x1A,
  0x0A,
  0x00,
  0x00,
  0x00,
  0x0D,
  0x49,
  0x48,
  0x44,
  0x52,
  0x00,
  0x00,
  0x00,
  0x01,
  0x00,
  0x00,
  0x00,
  0x01,
  0x08,
  0x04,
  0x00,
  0x00,
  0x00,
  0xB5,
  0x1C,
  0x0C,
  0x02,
  0x00,
  0x00,
  0x00,
  0x0B,
  0x49,
  0x44,
  0x41,
  0x54,
  0x78,
  0x9C,
  0x63,
  0xFC,
  0xCF,
  0xC0,
  0x00,
  0x00,
  0x04,
  0xBF,
  0x01,
  0xFE,
  0xA7,
  0x31,
  0x81,
  0x42,
  0x00,
  0x00,
  0x00,
  0x00,
  0x49,
  0x45,
  0x4E,
  0x44,
  0xAE,
  0x42,
  0x60,
  0x82,
];

Widget _wrap(Widget child) {
  return MaterialApp(
    home: Scaffold(body: Center(child: child)),
  );
}

void main() {
  testWidgets('StableNetworkImage reloadToken forces same-URL replacement', (
    tester,
  ) async {
    const imageUrl = 'https://cdn.example.com/shared.jpg';
    final first = _QueuedResponse.pending();
    final second = _QueuedResponse.pending();
    final responders = <String, Queue<_QueuedResponse>>{
      imageUrl: Queue<_QueuedResponse>.of([first, second]),
    };

    var reloadToken = 'cover-v1';

    await HttpOverrides.runZoned(() async {
      await tester.pumpWidget(
        _wrap(
          StatefulBuilder(
            builder: (context, setState) {
              return Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  SizedBox(
                    width: 48,
                    height: 48,
                    child: StableNetworkImage(
                      imageUrl: imageUrl,
                      reloadToken: reloadToken,
                      fallback: const SizedBox(width: 48, height: 48),
                    ),
                  ),
                  TextButton(
                    onPressed: () => setState(() {
                      reloadToken = 'cover-v2';
                    }),
                    child: const Text('Reload'),
                  ),
                ],
              );
            },
          ),
        ),
      );

      first.complete(_onePxPngBytes);
      await tester.pumpAndSettle();

      expect(find.byType(Image), findsOneWidget);

      PaintingBinding.instance.imageCache.evict(NetworkImage(imageUrl));
      await tester.tap(find.text('Reload'));
      await tester.pump();

      expect(
        find.descendant(
          of: find.byType(StableNetworkImage),
          matching: find.byType(Stack),
        ),
        findsOneWidget,
      );
      expect(
        find.descendant(
          of: find.byType(StableNetworkImage),
          matching: find.byType(Image),
        ),
        findsNWidgets(2),
      );

      second.complete(_onePxPngBytes);
      await tester.pumpAndSettle();

      expect(find.byType(Image), findsOneWidget);
      expect(responders[imageUrl]!.isEmpty, isTrue);
    }, createHttpClient: (_) => _QueuedImageHttpClient(responders));
  });

  testWidgets('ProfileCover reloadToken keeps previous frame during replace', (
    tester,
  ) async {
    const imageUrl = 'https://cdn.example.com/profile-cover.jpg';
    final first = _QueuedResponse.pending();
    final second = _QueuedResponse.pending();
    final responders = <String, Queue<_QueuedResponse>>{
      imageUrl: Queue<_QueuedResponse>.of([first, second]),
    };

    var reloadToken = 'profile-cover-v1';

    await HttpOverrides.runZoned(() async {
      await tester.pumpWidget(
        _wrap(
          StatefulBuilder(
            builder: (context, setState) {
              return Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  SizedBox(
                    width: 180,
                    height: 120,
                    child: ProfileCover(
                      coverPhotoUrl: imageUrl,
                      reloadToken: reloadToken,
                      height: 120,
                    ),
                  ),
                  TextButton(
                    onPressed: () => setState(() {
                      reloadToken = 'profile-cover-v2';
                    }),
                    child: const Text('Refresh cover'),
                  ),
                ],
              );
            },
          ),
        ),
      );

      first.complete(_onePxPngBytes);
      await tester.pumpAndSettle();

      expect(
        find.descendant(
          of: find.byType(ProfileCover),
          matching: find.byType(Image),
        ),
        findsOneWidget,
      );

      PaintingBinding.instance.imageCache.evict(NetworkImage(imageUrl));
      await tester.tap(find.text('Refresh cover'));
      await tester.pump();

      expect(
        find.descendant(
          of: find.byType(ProfileCover),
          matching: find.byType(Image),
        ),
        findsNWidgets(2),
      );

      second.complete(_onePxPngBytes);
      await tester.pumpAndSettle();

      expect(
        find.descendant(
          of: find.byType(ProfileCover),
          matching: find.byType(Image),
        ),
        findsOneWidget,
      );
    }, createHttpClient: (_) => _QueuedImageHttpClient(responders));
  });

  testWidgets(
    'SellerDualAvatar reloadToken preserves store and personal identity',
    (tester) async {
      const storeUrl = 'https://cdn.example.com/store-logo.jpg';
      final first = _QueuedResponse.pending();
      final second = _QueuedResponse.pending();
      final responders = <String, Queue<_QueuedResponse>>{
        storeUrl: Queue<_QueuedResponse>.of([first, second]),
      };

      var reloadToken = 'store-v1';

      await HttpOverrides.runZoned(() async {
        await tester.pumpWidget(
          _wrap(
            StatefulBuilder(
              builder: (context, setState) {
                return Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    SellerDualAvatar(
                      identity: const SellerIdentityData(
                        userId: 'seller-1',
                        username: 'qiqijho',
                        storeName: 'Qiqi Store',
                        avatarUrl: null,
                        storeImageUrl: storeUrl,
                        isSeller: true,
                      ),
                      storeImageReloadToken: reloadToken,
                      size: 96,
                    ),
                    TextButton(
                      onPressed: () => setState(() {
                        reloadToken = 'store-v2';
                      }),
                      child: const Text('Refresh store'),
                    ),
                  ],
                );
              },
            ),
          ),
        );

        first.complete(_onePxPngBytes);
        await tester.pumpAndSettle();

        expect(find.byType(SellerDualAvatar), findsOneWidget);
        expect(find.byType(ProfileAvatar), findsOneWidget);
        expect(
          find.descendant(
            of: find.byType(SellerDualAvatar),
            matching: find.byType(Image),
          ),
          findsOneWidget,
        );

        PaintingBinding.instance.imageCache.evict(NetworkImage(storeUrl));
        await tester.tap(find.text('Refresh store'));
        await tester.pump();

        expect(
          find.descendant(
            of: find.byType(SellerDualAvatar),
            matching: find.byType(Image),
          ),
          findsNWidgets(2),
        );

        second.complete(_onePxPngBytes);
        await tester.pumpAndSettle();

        expect(
          find.descendant(
            of: find.byType(SellerDualAvatar),
            matching: find.byType(Image),
          ),
          findsOneWidget,
        );
      }, createHttpClient: (_) => _QueuedImageHttpClient(responders));
    },
  );
}
