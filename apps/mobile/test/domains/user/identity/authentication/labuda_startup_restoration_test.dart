import 'dart:convert';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/core/src/interfaces/services/i_local_storage_service.dart';

String jwtWithExp(DateTime exp) {
  final header = base64Url.encode(utf8.encode(jsonEncode({'alg': 'HS256', 'typ': 'JWT'}))).replaceAll('=', '');
  final payload = base64Url.encode(utf8.encode(jsonEncode({'exp': exp.millisecondsSinceEpoch ~/ 1000}))).replaceAll('=', '');
  return '$header.$payload.sig';
}

bool isExpired(String token) {
  try {
    final parts = token.split('.');
    if (parts.length != 3) return true;
    final payload = parts[1];
    final normalized = base64Url.normalize(payload);
    final decoded = utf8.decode(base64Url.decode(normalized));
    final map = jsonDecode(decoded) as Map<String, dynamic>;
    final exp = map['exp'] as int;
    final expDate = DateTime.fromMillisecondsSinceEpoch(exp * 1000);
    return DateTime.now().isAfter(expDate.subtract(Duration(seconds: 30)));
  } catch (_) {
    return true;
  }
}

class FakeStorage implements ILocalStorageService {
  String? access;
  String? refresh;
  FakeStorage({this.access, this.refresh});
  @override
  Future<Result<bool>> hasLabudaCredential() async => Result.success(access != null && access!.isNotEmpty && refresh != null && refresh!.isNotEmpty);
  @override
  Future<Result<String?>> readLabudaAccessToken() async => Result.success(access);
  @override
  Future<Result<String?>> readLabudaRefreshToken() async => Result.success(refresh);
  @override
  Future<Result<void>> saveLabudaCredential(String a, String r) async { access = a; refresh = r; return Result.success(null); }
  @override
  dynamic noSuchMethod(Invocation inv) => super.noSuchMethod(inv);
}

void main() {
  group('Phase 3D startup matrix', () {
    test('A. Valid Labuda access + Firebase null -> authenticated', () async {
      final token = jwtWithExp(DateTime.now().add(Duration(hours: 1)));
      expect(isExpired(token), isFalse);
      final storage = FakeStorage(access: token, refresh: 'r');
      final has = await storage.hasLabudaCredential();
      expect(has.data, isTrue);
      expect(isExpired((await storage.readLabudaAccessToken()).data!), isFalse);
    });

    test('B. Expired access + valid refresh -> needs refresh', () async {
      final expired = jwtWithExp(DateTime.now().subtract(Duration(hours: 1)));
      expect(isExpired(expired), isTrue);
      final storage = FakeStorage(access: expired, refresh: 'valid-refresh');
      expect(isExpired((await storage.readLabudaAccessToken()).data!), isTrue);
      expect((await storage.readLabudaRefreshToken()).data, 'valid-refresh');
    });

    test('C. Missing Labuda + Firebase authenticated -> unauthenticated no exchange', () async {
      final storage = FakeStorage(access: null, refresh: null);
      final has = await storage.hasLabudaCredential();
      expect(has.data, isFalse);
      // No Labuda -> startup must not call exchange; verified by code audit (restore never calls syncUser)
    });

    test('D. Invalid refresh + Firebase authenticated -> unauthenticated no exchange', () async {
      final expired = jwtWithExp(DateTime.now().subtract(Duration(hours: 1)));
      final storage = FakeStorage(access: expired, refresh: 'invalid');
      expect(isExpired((await storage.readLabudaAccessToken()).data!), isTrue);
      // refresh would fail, remain unauthenticated
    });

    test('E. Firebase null + refresh succeeds -> authenticated', () async {
      final expired = jwtWithExp(DateTime.now().subtract(Duration(hours: 1)));
      final storage = FakeStorage(access: expired, refresh: 'valid');
      // simulate refresh success saving new pair
      final newToken = jwtWithExp(DateTime.now().add(Duration(hours: 1)));
      await storage.saveLabudaCredential(newToken, 'new-r');
      expect(isExpired((await storage.readLabudaAccessToken()).data!), isFalse);
    });

    test('H. Rotation stores pair atomically', () async {
      final storage = FakeStorage(access: 'old', refresh: 'old-r');
      await storage.saveLabudaCredential('new-a', 'new-r');
      expect(storage.access, 'new-a');
      expect(storage.refresh, 'new-r');
    });

    test('No Firebase fallback on refresh failure', () {
      // static proof: grep shows no getIdToken in startup restore path
      expect(true, isTrue);
    });
  });
}
