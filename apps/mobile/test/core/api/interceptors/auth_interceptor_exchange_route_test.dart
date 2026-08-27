import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/api/interceptors/auth_interceptor.dart';

class _OkAdapter implements HttpClientAdapter {
  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    return ResponseBody.fromBytes(
      utf8.encode(jsonEncode({'ok': true})),
      200,
      headers: {
        Headers.contentTypeHeader: ['application/json'],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  test('force-refreshes only the canonical firebase exchange route', () async {
    final forceRefreshCalls = <bool>[];
    final dio = Dio()..httpClientAdapter = _OkAdapter();
    dio.options.validateStatus = (_) => true;
    dio.interceptors.add(
      AuthInterceptor(
        tokenFetcher: (forceRefresh) async {
          forceRefreshCalls.add(forceRefresh);
          return 'stub-token';
        },
      ),
    );

    await dio.get<dynamic>('/api/v1/auth/firebase/exchange');
    await dio.get<dynamic>('/auth/firebase/exchange');
    await dio.get<dynamic>('/api/v1/auth/firebase/exchange/extra');

    expect(forceRefreshCalls, equals(<bool>[true, true, false]));
  });
}
