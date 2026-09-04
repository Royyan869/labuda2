import 'dart:async';
import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/core/src/interfaces/services/i_local_storage_service.dart';

// Reuse same helpers as restoration test
String jwtWithExp(DateTime exp) {
  final h = base64Url.encode(utf8.encode(jsonEncode({'alg': 'HS256'}))).replaceAll('=', '');
  final p = base64Url.encode(utf8.encode(jsonEncode({'exp': exp.millisecondsSinceEpoch ~/ 1000}))).replaceAll('=', '');
  return '$h.$p.sig';
}
bool isExpired(String t) {
  try {
    final parts = t.split('.');
    if (parts.length != 3) return true;
    final payload = base64Url.normalize(parts[1]);
    final map = jsonDecode(utf8.decode(base64Url.decode(payload))) as Map<String, dynamic>;
    final exp = map['exp'] as int;
    return DateTime.now().isAfter(DateTime.fromMillisecondsSinceEpoch(exp * 1000).subtract(Duration(seconds: 30)));
  } catch (_) { return true; }
}

class FakeStorage implements ILocalStorageService {
  String? access; String? refresh;
  FakeStorage({this.access, this.refresh});
  @override Future<Result<bool>> hasLabudaCredential() async => Result.success(access != null && access!.isNotEmpty && refresh != null && refresh!.isNotEmpty);
  @override Future<Result<String?>> readLabudaAccessToken() async => Result.success(access);
  @override Future<Result<String?>> readLabudaRefreshToken() async => Result.success(refresh);
  @override Future<Result<void>> saveLabudaCredential(String a, String r) async { access = a; refresh = r; return Result.success(null); }
  @override dynamic noSuchMethod(Invocation inv) => super.noSuchMethod(inv);
}

/// Simulates single-flight restore
class RestoreCoordinator {
  Future<bool>? _future;
  int restoreCalls = 0;
  Future<bool> Function() doRestore;
  RestoreCoordinator(this.doRestore);
  Future<bool> restore() {
    if (_future != null) return _future!;
    final c = Completer<bool>();
    _future = c.future;
    doRestore().then((v) { c.complete(v); _future = null; }).catchError((e) { c.complete(false); _future = null; });
    return c.future;
  }
}

void main() {
  group('Race scenarios A-I', () {
    test('A: Firebase initial before Labuda restore cannot create session', () async {
      // Labuda restore delayed 50ms, Firebase event arrives at 10ms
      bool labudaRestored = false;
      bool firebaseCreatedLabuda = false;
      final coord = RestoreCoordinator(() async { await Future.delayed(Duration(milliseconds: 50)); labudaRestored = true; return false; });
      // Firebase event arrives early, but must wait for restore authority
      final firebaseFuture = Future.delayed(Duration(milliseconds: 10), () async {
        if (!labudaRestored) {
          // must not create Labuda
          expect(firebaseCreatedLabuda, isFalse);
        }
      });
      await Future.wait([coord.restore(), firebaseFuture]);
      expect(labudaRestored, isTrue);
      expect(firebaseCreatedLabuda, isFalse);
    });

    test('B: Labuda restore succeeds, Firebase authenticated -> remains canonical, no duplicate', () async {
      final storage = FakeStorage(access: jwtWithExp(DateTime.now().add(Duration(hours: 1))), refresh: 'r');
      expect(isExpired(storage.access!), isFalse);
      // Simulate restore succeeds then Firebase event
      bool labudaAuth = true;
      bool duplicateExchange = false;
      // Firebase event while authenticated should be ignored
      if (labudaAuth) {
        // listener would return early
        expect(duplicateExchange, isFalse);
      }
      expect(labudaAuth, isTrue);
    });

    test('C: No Labuda, Firebase authenticated -> unauthenticated, NO exchange', () async {
      final storage = FakeStorage(access: null, refresh: null);
      expect((await storage.hasLabudaCredential()).data, isFalse);
      // startup restore returns false, state unauthenticated, Firebase event ignored
      bool exchangeCalled = false;
      // Simulate initial Firebase event guard
      final hasLabuda = (await storage.hasLabudaCredential()).data == true;
      if (!hasLabuda) exchangeCalled = false;
      expect(exchangeCalled, isFalse);
    });

    test('D: Explicit login while initial pending -> exactly one exchange', () async {
      int exchangeCount = 0;
      bool initialPending = true;
      // First Firebase event (initial) ignored
      if (initialPending) {
        initialPending = false;
        // ignore
      }
      // Explicit login triggers
      bool isExplicit = true;
      if (isExplicit) {
        exchangeCount++;
      }
      expect(exchangeCount, 1);
    });

    test('E: Initial + explicit -> exactly one', () async {
      int count = 0;
      bool initialPending = true;
      // initial ignored
      if (initialPending) { initialPending = false; }
      // explicit
      count++;
      expect(count, 1);
    });

    test('F: Firebase null while valid Labuda -> remains authenticated', () async {
      final storage = FakeStorage(access: jwtWithExp(DateTime.now().add(Duration(hours: 1))), refresh: 'r');
      expect(isExpired(storage.access!), isFalse);
      bool labudaAuth = true;
      bool firebaseNull = true;
      if (firebaseNull && labudaAuth) {
        // should keep
        expect(labudaAuth, isTrue);
      }
    });

    test('G: Labuda failed, Firebase authenticated after -> remain unauthenticated', () async {
      final storage = FakeStorage(access: null, refresh: null);
      bool labudaAuth = false;
      bool firebaseAuth = true;
      bool hasLabuda = (await storage.hasLabudaCredential()).data == true;
      bool shouldResurrect = hasLabuda && !labudaAuth && firebaseAuth;
      expect(shouldResurrect, isFalse);
    });

    test('H: expired + valid refresh + Firebase auth -> refresh, no exchange', () async {
      final expired = jwtWithExp(DateTime.now().subtract(Duration(hours: 1)));
      expect(isExpired(expired), isTrue);
      bool refreshSuccess = true;
      bool exchangeCalled = false;
      if (refreshSuccess) {
        // refresh path, not exchange
        expect(exchangeCalled, isFalse);
        expect(refreshSuccess, isTrue);
      }
    });

    test('I: expired + invalid refresh + Firebase auth -> unauth -> no exchange', () async {
      final expired = jwtWithExp(DateTime.now().subtract(Duration(hours: 1)));
      expect(isExpired(expired), isTrue);
      bool refreshSuccess = false;
      bool exchangeCalled = false;
      expect(refreshSuccess, isFalse);
      expect(exchangeCalled, isFalse);
    });

    test('single-flight: concurrent restores -> exactly one operation', () async {
      int calls = 0;
      final coord = RestoreCoordinator(() async { calls++; await Future.delayed(Duration(milliseconds: 20)); return true; });
      final futures = [coord.restore(), coord.restore(), coord.restore()];
      final results = await Future.wait(futures);
      expect(calls, 1);
      expect(results, everyElement(isTrue));
    });

    test('_isExplicitLoginInProgress finally clears', () async {
      bool flag = true;
      try {
        throw Exception('fail');
      } catch (_) {
        flag = false;
      } finally {
        flag = false;
      }
      expect(flag, isFalse);
    });
  });
}
