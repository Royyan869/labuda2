import 'dart:async';
import 'dart:collection';
import 'dart:convert';
import 'dart:io';

class QueuedImageHttpClient implements HttpClient {
  QueuedImageHttpClient(this._responders);

  final Map<String, Queue<QueuedImageResponseSpec>> _responders;
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
    return QueuedImageHttpClientRequest(queue.removeFirst());
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class QueuedImageHttpClientRequest implements HttpClientRequest {
  QueuedImageHttpClientRequest(this._spec);

  final QueuedImageResponseSpec _spec;

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
  Future<HttpClientResponse> close() async => _spec.response;

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class QueuedImageResponseSpec {
  QueuedImageResponseSpec.success(this.bytes)
    : statusCode = HttpStatus.ok;

  QueuedImageResponseSpec.failure({this.statusCode = HttpStatus.notFound})
    : bytes = const <int>[];

  final int statusCode;
  final List<int> bytes;

  HttpClientResponse get response => _QueuedImageHttpClientResponse(
    statusCode: statusCode,
    stream: bytes.isEmpty
        ? const Stream<List<int>>.empty()
        : Stream<List<int>>.value(bytes),
  );
}

const List<int> onePxPngBytes = <int>[
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

class _QueuedImageHttpClientResponse extends StreamView<List<int>>
    implements HttpClientResponse {
  _QueuedImageHttpClientResponse({
    required Stream<List<int>> stream,
    required this.statusCode,
  }) : super(stream);

  @override
  final int statusCode;

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
  String get reasonPhrase => statusCode == HttpStatus.ok ? 'OK' : 'ERROR';

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
