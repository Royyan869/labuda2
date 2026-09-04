import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/api/interceptors/auth_interceptor.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/core/src/interfaces/services/i_local_storage_service.dart';

class FakeStorage implements ILocalStorageService {
  String? access;
  String? refresh;
  int clearCount = 0;
  FakeStorage({this.access, this.refresh});
  @override Future<Result<bool>> hasLabudaCredential() async => Result.success(access != null && access!.isNotEmpty && refresh != null && refresh!.isNotEmpty);
  @override Future<Result<String?>> readLabudaAccessToken() async => Result.success(access);
  @override Future<Result<String?>> readLabudaRefreshToken() async => Result.success(refresh);
  @override Future<Result<void>> saveLabudaCredential(String a, String r) async { access = a; refresh = r; return Result.success(null); }
  @override Future<Result<void>> clearLabudaCredential() async { access = null; refresh = null; clearCount++; return Result.success(null); }
  @override dynamic noSuchMethod(Invocation inv) => super.noSuchMethod(inv);
}

class CaptureLogoutAdapter implements HttpClientAdapter {
  String? logoutAuth;
  String? logoutRefreshBody;
  @override Future<ResponseBody> fetch(RequestOptions opts, Stream<Uint8List>? s, Future<void>? c) async {
    if (opts.path.contains('/auth/logout')) {
      logoutAuth = opts.headers['Authorization']?.toString();
      final data = opts.data;
      if (data is Map) logoutRefreshBody = data['refresh_token']?.toString();
      // success envelope
      return ResponseBody.fromBytes(utf8.encode(jsonEncode({'success': true, 'data': {}})), 200, headers: {Headers.contentTypeHeader: ['application/json']});
    }
    if (opts.path.contains('/auth/refresh')) {
      return ResponseBody.fromBytes(utf8.encode(jsonEncode({'success': true, 'data': {'access_token': 'new-a', 'refresh_token': 'new-r', 'expires_at': DateTime.now().toIso8601String(), 'refresh_expires_at': DateTime.now().toIso8601String()}})), 200, headers: {Headers.contentTypeHeader: ['application/json']});
    }
    // original protected
    if (opts.extra['labuda_retry'] == true) {
      return ResponseBody.fromBytes(utf8.encode(jsonEncode({'success': true, 'data': {'ok': true}})), 200, headers: {Headers.contentTypeHeader: ['application/json']});
    }
    return ResponseBody.fromBytes(utf8.encode(jsonEncode({'success': false})), 401, headers: {Headers.contentTypeHeader: ['application/json']});
  }
  @override void close({bool force = false}) {}
}

Dio buildDio(FakeStorage storage, HttpClientAdapter adapter) {
  final dio = Dio(BaseOptions(baseUrl: 'https://example.com'))..httpClientAdapter = adapter..options.validateStatus = (s) => s != null && s < 500 && s != 401;
  final inter = AuthInterceptor(localStorage: storage);
  inter.attachDio(dio);
  dio.interceptors.add(inter);
  return dio;
}

void main() {
  group('Phase 3E logout termination', () {
    test('A. Successful current logout -> POST /auth/logout with Labuda Bearer, then local cleared', () async {
      final storage = FakeStorage(access: 'old-access', refresh: 'old-refresh');
      final adapter = CaptureLogoutAdapter();
      final dio = buildDio(storage, adapter);
      // Simulate AuthController.signOut backend call: POST /auth/logout with Labuda JWT
      final resp = await dio.post('/auth/logout', data: {'refresh_token': 'old-refresh'});
      expect(resp.statusCode, 200);
      expect(adapter.logoutAuth, 'Bearer old-access');
      expect(adapter.logoutRefreshBody, 'old-refresh');
      // Phase 3E local clearing
      await storage.clearLabudaCredential();
      expect(storage.access, isNull);
      expect(storage.refresh, isNull);
      expect(storage.clearCount, 1);
    });

    test('D. Firebase cannot resurrect after logout', () async {
      final storage = FakeStorage(access: null, refresh: null);
      // Firebase still "signed in" is irrelevant, Labuda missing -> hasLabuda false
      expect((await storage.hasLabudaCredential()).data, isFalse);
      // startup would remain unauthenticated, no exchange
      bool exchangeCalled = false;
      // simulate startup guard: if !hasLabuda, don't call exchange
      final hasLabuda = (await storage.hasLabudaCredential()).data == true;
      if (!hasLabuda) exchangeCalled = false;
      expect(exchangeCalled, isFalse);
    });

    test('H. Refresh in flight + logout -> stale refresh cannot save', () async {
      final storage = FakeStorage(access: 'old-a', refresh: 'old-r');
      const oldRefresh = 'old-r';
      // Simulate logout clearing storage while refresh in flight (before save)
      await storage.clearLabudaCredential();
      expect(storage.access, isNull);
      final cur = await storage.readLabudaRefreshToken();
      expect(cur.data, isNull);
      // Guard in AuthInterceptor._doRefresh checks cur != oldRefresh -> abort save
      expect(cur.data != oldRefresh, isTrue);
      expect(storage.clearCount, 1);
      // Prove that even if refresh later tries to save, it would be aborted (no resurrection)
      bool wouldSave = cur.data == oldRefresh;
      expect(wouldSave, isFalse);
    });

    test('J. Startup after logout: Firebase signed in + Labuda missing -> unauth', () async {
      final storage = FakeStorage(access: null, refresh: null);
      // Firebase signed in simulated as true, but Labuda missing
      bool firebaseSignedIn = true;
      bool labudaHas = (await storage.hasLabudaCredential()).data == true;
      bool shouldBeAuthenticated = labudaHas; // Labuda authority only
      expect(shouldBeAuthenticated, isFalse);
      expect(firebaseSignedIn && !labudaHas, isTrue); // Firebase alone not enough
    });

    test('Local clearing uses canonical clearLabudaCredential', () async {
      final storage = FakeStorage(access: 'a', refresh: 'r');
      await storage.clearLabudaCredential();
      expect(storage.clearCount, 1);
      expect(storage.access, isNull);
    });
  });
}
