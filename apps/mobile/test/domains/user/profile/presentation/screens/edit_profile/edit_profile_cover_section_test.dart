import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/user/profile/presentation/screens/edit_profile/edit_profile_cover_section.dart';

class _StaticImageHttpOverrides extends HttpOverrides {
  _StaticImageHttpOverrides(this._bytes);

  final List<int> _bytes;

  @override
  HttpClient createHttpClient(SecurityContext? context) {
    return _StaticImageHttpClient(_bytes);
  }
}

class _StaticImageHttpClient implements HttpClient {
  _StaticImageHttpClient(this._bytes);

  final List<int> _bytes;

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
    return _StaticImageHttpClientRequest(_bytes);
  }

  @override
  noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _StaticImageHttpClientRequest implements HttpClientRequest {
  _StaticImageHttpClientRequest(this._bytes);

  final List<int> _bytes;

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
  Future<HttpClientResponse> close() async {
    return _StaticImageHttpClientResponse(_bytes);
  }

  @override
  noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _StaticImageHttpClientResponse extends StreamView<List<int>>
    implements HttpClientResponse {
  _StaticImageHttpClientResponse(this._bytes)
    : super(Stream<List<int>>.value(_bytes));

  final List<int> _bytes;

  @override
  int get statusCode => HttpStatus.ok;

  @override
  int get contentLength => _bytes.length;

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
  Future<Socket> detachSocket() {
    throw UnimplementedError('detachSocket is not used in this test');
  }

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

void main() {
  setUpAll(() {
    HttpOverrides.global = _StaticImageHttpOverrides(
      base64Decode(
        'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9W1Xzj8AAAAASUVORK5CYII=',
      ),
    );
  });

  tearDownAll(() {
    HttpOverrides.global = null;
  });

  testWidgets('renders provided cover image when a cover url exists', (
    tester,
  ) async {
    const coverUrl = 'https://cdn.example.com/covers/user-1.jpg?sig=cover';

    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(
          body: EditProfileCoverSection(
            userId: 'user-1',
            coverPhotoUrl: coverUrl,
            isCoverMarkedForRemoval: false,
            onChangeCover: _noop,
            onRemoveCover: _noop,
          ),
        ),
      ),
    );

    final coverWidget = tester.widget<EditProfileCoverSection>(
      find.byType(EditProfileCoverSection),
    );
    expect(coverWidget.coverPhotoUrl, coverUrl);
    expect(find.text('Cover Photo'), findsOneWidget);
    expect(find.text('Remove Cover'), findsOneWidget);
  });

  testWidgets('renders placeholder when cover is null', (tester) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(
          body: EditProfileCoverSection(
            userId: 'user-1',
            coverPhotoUrl: null,
            isCoverMarkedForRemoval: false,
            onChangeCover: _noop,
            onRemoveCover: _noop,
          ),
        ),
      ),
    );

    final coverWidget = tester.widget<EditProfileCoverSection>(
      find.byType(EditProfileCoverSection),
    );
    expect(coverWidget.coverPhotoUrl, isNull);
    expect(find.text('Cover Photo'), findsOneWidget);
    expect(find.text('Remove Cover'), findsNothing);
  });
}

void _noop() {}
