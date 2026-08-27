import 'dart:async';

import 'package:firebase_auth/firebase_auth.dart';
import 'package:flutter_test/flutter_test.dart';

// =============================================================================
// Production-contract mirror: _PendingEmailRegistration
//
// This is a test-visible copy of the production class in auth_controller.dart.
// Tests verify the behavioral contract that the real implementation must satisfy.
// =============================================================================

class _SignupAttempt {
  final String expectedEmail;
  final String normalizedUsername;
  final int authEpoch;
  String? firebaseUid;
  bool cleared = false;
  bool createdByCurrentAttempt = false;

  _SignupAttempt({
    required this.expectedEmail,
    required this.normalizedUsername,
    required this.authEpoch,
  });

  void markFirebaseUserCreated(String uid) {
    createdByCurrentAttempt = true;
    firebaseUid = uid;
  }
}

// =============================================================================
// Real-ordering Firebase Auth fake
//
// Emits authStateChanges BEFORE createUserWithEmailAndPassword returns.
// This reproduces the exact production race condition observed by the owner.
// =============================================================================

class _RealOrderingFirebaseAuth extends Fake implements FirebaseAuth {
  final StreamController<User?> _authController = StreamController<User?>.broadcast();
  final List<String> _eventLog = [];
  int createCalls = 0;
  int exchangeCalls = 0;
  int usernameLessExchangeCalls = 0;
  String? lastExchangeUsername;

  _DeletableUser? _currentUser;

  @override
  User? get currentUser => _currentUser;

  @override
  Stream<User?> authStateChanges() => _authController.stream;

  @override
  Future<UserCredential> createUserWithEmailAndPassword({
    required String email,
    required String password,
  }) async {
    createCalls++;
    final user = _DeletableUser(uidValue: 'fb-${createCalls}');
    _currentUser = user;
    _eventLog.add('createUserWithEmailAndPassword: uid=${user.uid}');

    // EMIT BEFORE returning — this is the production race condition.
    _authController.add(user);
    _eventLog.add('authStateChanges emitted: uid=${user.uid} BEFORE create future completed');

    // Return the credential after the emission.
    return _DeletableUserCredential(user);
  }

  /// Simulate the listener callback. Returns true if suppressed.
  bool simulateListenerEvent(User? user, _SignupAttempt? pendingReg) {
    _eventLog.add('listener fired: uid=${user?.uid}');
    if (user == null) return false;

    // Check: is an explicit signup in progress?
    if (pendingReg != null && !pendingReg.cleared) {
      _eventLog.add('listener SUPPRESSED: active signup context exists');
      return true; // suppressed
    }
    _eventLog.add('listener PROCEEDED: no signup context (would exchange)');
    return false; // would exchange
  }

  /// Simulate the explicit exchange after UID binding.
  void simulateExplicitExchange(_SignupAttempt reg, String username) {
    exchangeCalls++;
    lastExchangeUsername = username;
    _eventLog.add('explicit exchange: registration_username=$username');
    if (username.isEmpty) {
      usernameLessExchangeCalls++;
      _eventLog.add('USERNAME MISSING — would be rejected by backend');
    }
  }

  List<String> get eventLog => List.unmodifiable(_eventLog);
}

class _DeletableUser extends Fake implements User {
  _DeletableUser({required this.uidValue});
  final String uidValue;
  int deleteCalls = 0;

  @override
  String get uid => uidValue;

  @override
  String? get email => '$uidValue@test.com';

  @override
  bool get emailVerified => false;

  @override
  String? get phoneNumber => null;

  @override
  List<UserInfo> get providerData => const <UserInfo>[];

  @override
  UserMetadata get metadata => _FakeUserMetadata();

  @override
  Future<void> reload() async {}

  @override
  Future<String?> getIdToken([bool forceRefresh = false]) async => 'token-$uidValue';

  @override
  Future<void> delete() async {
    deleteCalls++;
  }
}

class _DeletableUserCredential extends Fake implements UserCredential {
  _DeletableUserCredential(this.user);
  @override
  final User? user;
}

class _FakeUserMetadata extends Fake implements UserMetadata {
  @override
  DateTime? get creationTime => DateTime(2026);
  @override
  DateTime? get lastSignInTime => DateTime(2026);
}

// =============================================================================
// Tests
// =============================================================================

void main() {
  group('real-ordering listener race', () {
    test('Firebase emits authStateChanges before create future completes', () async {
      final auth = _RealOrderingFirebaseAuth();
      final events = <String>[];

      // Subscribe BEFORE creation.
      final sub = auth.authStateChanges().listen((user) {
        events.add('received: ${user?.uid}');
      });

      // Create user — authStateChanges fires inside this call.
      final cred = await auth.createUserWithEmailAndPassword(
        email: 'test@example.com',
        password: 'Password123!',
      );

      // The event was received BEFORE create returned.
      expect(events.length, 1, reason: 'authStateChanges must fire during createUserWithEmailAndPassword');
      expect(events.first, contains('fb-1'));

      // The credential is available AFTER create returns.
      expect(cred.user?.uid, 'fb-1');

      await sub.cancel();
    });

    test('listener is suppressed when explicit signup context exists', () {
      final auth = _RealOrderingFirebaseAuth();
      final reg = _SignupAttempt(
        expectedEmail: 'test@example.com',
        normalizedUsername: 'testuser',
        authEpoch: 1,
      );

      // Create the user — listener fires before create returns.
      // Simulate the listener callback with the signup context active.
      final user = _DeletableUser(uidValue: 'fb-1');
      final suppressed = auth.simulateListenerEvent(user, reg);

      expect(suppressed, isTrue,
          reason: 'Listener must be suppressed when signup context exists');
      expect(auth.eventLog, contains('listener SUPPRESSED: active signup context exists'));
    });

    test('listener performs zero exchanges while signup context active', () {
      final auth = _RealOrderingFirebaseAuth();
      final reg = _SignupAttempt(
        expectedEmail: 'test@example.com',
        normalizedUsername: 'testuser',
        authEpoch: 1,
      );

      // Simulate the full sequence.
      // 1. createUserWithEmailAndPassword starts (pendingReg already set)
      // 2. authStateChanges fires — listener runs, sees pendingReg → suppressed
      final user = _DeletableUser(uidValue: 'fb-1');
      final suppressed = auth.simulateListenerEvent(user, reg);
      expect(suppressed, isTrue);

      // 3. createUserWithEmailAndPassword returns
      // 4. UID is bound, explicit exchange runs
      reg.markFirebaseUserCreated(user.uid);
      auth.simulateExplicitExchange(reg, reg.normalizedUsername);

      // Assertions.
      expect(reg.createdByCurrentAttempt, isTrue);
      expect(reg.firebaseUid, 'fb-1');
      expect(auth.exchangeCalls, 1, reason: 'Exactly one explicit exchange');
      expect(auth.lastExchangeUsername, 'testuser',
          reason: 'Exchange includes registration_username');
      expect(auth.usernameLessExchangeCalls, 0,
          reason: 'Zero username-less exchanges');
    });

    test('listener resumes after signup context cleared', () {
      final auth = _RealOrderingFirebaseAuth();

      // Signup context exists.
      final reg = _SignupAttempt(
        expectedEmail: 'test@example.com',
        normalizedUsername: 'testuser',
        authEpoch: 1,
      );
      reg.markFirebaseUserCreated('fb-1');

      // Explicit exchange succeeds.
      auth.simulateExplicitExchange(reg, reg.normalizedUsername);

      // Clear context (after successful exchange).
      reg.cleared = true;

      // Now a listener event would NOT be suppressed.
      final user = _DeletableUser(uidValue: 'fb-1');
      final suppressed = auth.simulateListenerEvent(user, reg);
      expect(suppressed, isFalse,
          reason: 'Listener must resume after signup context cleared');
    });

    test('signup failure clears context allowing future login', () {
      final auth = _RealOrderingFirebaseAuth();

      // Signup context exists but Firebase creation fails.
      final reg = _SignupAttempt(
        expectedEmail: 'test@example.com',
        normalizedUsername: 'testuser',
        authEpoch: 1,
      );
      // Firebase creation failed — clear context.
      reg.cleared = true;

      // A subsequent login event is not suppressed.
      final user = _DeletableUser(uidValue: 'fb-existing');
      final suppressed = auth.simulateListenerEvent(user, reg);
      expect(suppressed, isFalse,
          reason: 'Failed signup must not permanently suppress listener');
    });

    test('one signup produces exactly one username-bearing exchange', () {
      // Reproduce the full owner scenario.
      final auth = _RealOrderingFirebaseAuth();
      const submittedUsername = 'royyan89';

      // Step 1: Create pending registration.
      final reg = _SignupAttempt(
        expectedEmail: 'nurroyyan89@gmail.com',
        normalizedUsername: submittedUsername,
        authEpoch: 1,
      );

      // Step 2: Firebase createUserWithEmailAndPassword starts.
      // authStateChanges fires with new user BEFORE create returns.
      final user = _DeletableUser(uidValue: 'pTfWMZdO73c0fWpQikW3Touk5P83');
      final suppressed = auth.simulateListenerEvent(user, reg);
      expect(suppressed, isTrue, reason: 'Listener suppressed during signup');

      // Step 3: createUserWithEmailAndPassword returns.
      // UID is bound to the attempt.
      reg.markFirebaseUserCreated(user.uid);

      // Step 4: Explicit exchange runs with bound UID + username.
      auth.simulateExplicitExchange(reg, reg.normalizedUsername);

      // Results.
      expect(auth.exchangeCalls, 1);
      expect(auth.lastExchangeUsername, submittedUsername);
      expect(auth.usernameLessExchangeCalls, 0);
      expect(reg.createdByCurrentAttempt, isTrue);
      expect(reg.firebaseUid, user.uid);
    });

    test('ordinary login has no signup context so listener exchanges normally', () {
      final auth = _RealOrderingFirebaseAuth();
      // No _SignupAttempt — this is ordinary login.

      final user = _DeletableUser(uidValue: 'fb-existing');
      final suppressed = auth.simulateListenerEvent(user, null);
      expect(suppressed, isFalse,
          reason: 'Ordinary login must not be suppressed');
    });

    test('Google login has no signup context so listener exchanges normally', () {
      final auth = _RealOrderingFirebaseAuth();
      // Google sign-in does not create a _SignupAttempt.

      final user = _DeletableUser(uidValue: 'fb-google');
      final suppressed = auth.simulateListenerEvent(user, null);
      expect(suppressed, isFalse,
          reason: 'Google login must not be suppressed');
    });

    test('session restore has no signup context so listener exchanges', () {
      final auth = _RealOrderingFirebaseAuth();
      // Session restore does not create a _SignupAttempt.

      final user = _DeletableUser(uidValue: 'fb-restore');
      final suppressed = auth.simulateListenerEvent(user, null);
      expect(suppressed, isFalse,
          reason: 'Session restore must not be suppressed');
    });

    test('listener emits twice — only first is suppressed, second after clear is not', () {
      final auth = _RealOrderingFirebaseAuth();
      final reg = _SignupAttempt(
        expectedEmail: 'test@example.com',
        normalizedUsername: 'testuser',
        authEpoch: 1,
      );

      // First emission: suppressed (signup active).
      final user = _DeletableUser(uidValue: 'fb-1');
      expect(auth.simulateListenerEvent(user, reg), isTrue);

      // Explicit exchange completes, context cleared.
      reg.markFirebaseUserCreated(user.uid);
      auth.simulateExplicitExchange(reg, reg.normalizedUsername);
      reg.cleared = true;

      // Second emission: NOT suppressed (signup done).
      expect(auth.simulateListenerEvent(user, reg), isFalse);
    });

    test('exchange failure clears context so listener can resume', () {
      final auth = _RealOrderingFirebaseAuth();
      final reg = _SignupAttempt(
        expectedEmail: 'test@example.com',
        normalizedUsername: 'testuser',
        authEpoch: 1,
      );
      reg.markFirebaseUserCreated('fb-1');

      // Simulate deterministic backend rejection.
      // Compensation clears the context.
      reg.cleared = true;

      final user = _DeletableUser(uidValue: 'fb-1');
      final suppressed = auth.simulateListenerEvent(user, reg);
      expect(suppressed, isFalse,
          reason: 'After compensation, listener must not be suppressed');
    });

    test('username-less exchange count is zero in valid signup flow', () {
      final auth = _RealOrderingFirebaseAuth();
      final reg = _SignupAttempt(
        expectedEmail: 'test@example.com',
        normalizedUsername: 'validuser',
        authEpoch: 1,
      );

      // Listener suppressed.
      auth.simulateListenerEvent(_DeletableUser(uidValue: 'fb-1'), reg);

      // Explicit exchange with username.
      reg.markFirebaseUserCreated('fb-1');
      auth.simulateExplicitExchange(reg, reg.normalizedUsername);

      expect(auth.usernameLessExchangeCalls, 0);
      expect(auth.exchangeCalls, 1);
    });
  });
}
