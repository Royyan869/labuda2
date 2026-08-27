import 'package:dio/dio.dart';
import 'package:firebase_auth/firebase_auth.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mockito/mockito.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/identity/authentication/data/datasources/auth_api_datasource.dart';
import 'package:labuda/domains/user/identity/authentication/data/repositories/auth_profile_repository.dart';
import 'package:labuda/domains/user/identity/authentication/data/services/firebase_auth_core_service.dart';
import 'package:labuda/domains/user/identity/authentication/data/username_service.dart';
import 'package:labuda/domains/user/profile/data/datasources/user_api_datasource.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/user_profile_patch.dart';
import 'package:labuda/domains/user/profile/presentation/screens/edit_profile/edit_profile_personal_section.dart';

class _MockFirebaseAuth extends Mock implements FirebaseAuth {
  _MockFirebaseAuth({this.createUserCredential, this.currentUserValue});

  final UserCredential? createUserCredential;
  final User? currentUserValue;
  int createUserCalls = 0;

  @override
  Stream<User?> authStateChanges() => const Stream<User?>.empty();

  @override
  User? get currentUser => currentUserValue;

  @override
  Future<UserCredential> createUserWithEmailAndPassword({
    required String email,
    required String password,
  }) async {
    createUserCalls++;
    return createUserCredential!;
  }
}

class _MockUser extends Mock implements User {
  _MockUser({
    required this.uidValue,
    required this.emailValue,
    required this.photoUrlValue,
    required this.phoneNumberValue,
    required this.emailVerifiedValue,
    required this.metadataValue,
    required this.providerDataValue,
  });

  final String uidValue;
  final String? emailValue;
  String? photoUrlValue;
  final String? phoneNumberValue;
  final bool emailVerifiedValue;
  final UserMetadata metadataValue;
  final List<UserInfo> providerDataValue;

  int updateDisplayNameCalls = 0;
  int updatePhotoUrlCalls = 0;
  int reloadCalls = 0;
  String? lastDisplayNameArg;
  String? lastPhotoUrlArg;

  @override
  String get uid => uidValue;

  @override
  String? get email => emailValue;

  @override
  String? get photoURL => photoUrlValue;

  @override
  String? get phoneNumber => phoneNumberValue;

  @override
  bool get emailVerified => emailVerifiedValue;

  @override
  UserMetadata get metadata => metadataValue;

  @override
  List<UserInfo> get providerData => providerDataValue;

  @override
  Future<void> reload() async {
    reloadCalls++;
  }

  @override
  Future<void> updateDisplayName(String? displayName) async {
    updateDisplayNameCalls++;
    lastDisplayNameArg = displayName;
  }

  @override
  Future<void> updatePhotoURL(String? photoURL) async {
    updatePhotoUrlCalls++;
    lastPhotoUrlArg = photoURL;
    photoUrlValue = photoURL;
  }
}

class _MockUserCredential extends Mock implements UserCredential {
  _MockUserCredential(this.userValue);

  final User? userValue;

  @override
  User? get user => userValue;
}

class _MockUserMetadata extends Mock implements UserMetadata {
  _MockUserMetadata({
    required this.creationTimeValue,
    required this.lastSignInTimeValue,
  });

  final DateTime? creationTimeValue;
  final DateTime? lastSignInTimeValue;

  @override
  DateTime? get creationTime => creationTimeValue;

  @override
  DateTime? get lastSignInTime => lastSignInTimeValue;
}

class _MockApiClient implements ApiClient {
  const _MockApiClient();

  @override
  Dio get dio => throw UnimplementedError();

  @override
  Future<Response<T>> delete<T>(
    String path, {
    dynamic data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async => throw UnimplementedError();

  @override
  ApiException extractException(DioException e) =>
      UnknownApiException(message: e.message ?? 'unknown');

  @override
  Future<Response<T>> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async => throw UnimplementedError();

  @override
  bool isNetworkError(DioException e) => false;

  @override
  bool isNotFound(DioException e) => false;

  @override
  bool isUnauthorized(DioException e) => false;

  @override
  bool isValidationError(DioException e) => false;

  @override
  Future<Response<T>> patch<T>(
    String path, {
    dynamic data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async => throw UnimplementedError();

  @override
  Future<Response<T>> post<T>(
    String path, {
    dynamic data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async => throw UnimplementedError();

  @override
  Future<Response<T>> put<T>(
    String path, {
    dynamic data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async => throw UnimplementedError();

  @override
  Future<Response<T>> uploadFile<T>(
    String path, {
    required String filePath,
    required String fieldName,
    Map<String, dynamic>? additionalFields,
    Options? options,
    CancelToken? cancelToken,
    void Function(int, int)? onSendProgress,
  }) async => throw UnimplementedError();
}

class _RecordingAuthApiDatasource extends AuthApiDatasource {
  _RecordingAuthApiDatasource() : super(const _MockApiClient());

  String? recordedUsername;
  String? recordedBio;
  String? recordedPhotoUrl;
  String? recordedPhoneNumber;
  String? recordedLocation;

  @override
  Future<Result<Map<String, dynamic>>> updateProfile({
    String? username,
    String? bio,
    String? photoUrl,
    String? phoneNumber,
    String? location,
    DateTime? phoneVerifiedAt,
    DateTime? dateOfBirth,
  }) async {
    recordedUsername = username;
    recordedBio = bio;
    recordedPhotoUrl = photoUrl;
    recordedPhoneNumber = phoneNumber;
    recordedLocation = location;

    return Result.success({
      'id': 'user-1',
      'email': 'yayan@example.com',
      'is_email_verified': true,
      'created_at': '2026-06-01T00:00:00.000Z',
      'updated_at': '2026-06-02T00:00:00.000Z',
      'profile': {
        'id': 'user-1',
        'username': username ?? 'yayan',
        'bio': bio,
        'avatar_url': photoUrl,
      },
    });
  }
}

class _FakeUsernameService extends UsernameService {
  _FakeUsernameService() : super(_MockUsernameDatasource());

  @override
  void checkUsernameAvailability({
    required String username,
    required void Function(UsernameCheckResult) onResult,
    Duration delay = const Duration(milliseconds: 500),
  }) {
    onResult(UsernameCheckResult.available());
  }
}

class _MockUsernameDatasource extends Mock implements UserApiDatasource {}

class _FakeAuthController extends AuthController {
  _FakeAuthController(this._state);

  final AuthState _state;
  String? lastCompletedUsername;

  @override
  AuthState build() => _state;

  @override
  Future<bool> completeProfile({required String username}) async {
    lastCompletedUsername = username;
    return true;
  }
}

void main() {
  testWidgets('Edit profile personal section only exposes username and bio', (
    tester,
  ) async {
    final usernameController = TextEditingController(text: 'yayan');
    final bioController = TextEditingController(text: 'bio');

    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: EditProfilePersonalSection(
            usernameController: usernameController,
            bioController: bioController,
          ),
        ),
      ),
    );

    expect(find.text('Username'), findsOneWidget);
    expect(find.text('Bio'), findsOneWidget);
    expect(find.text('Display Name'), findsNothing);
    expect(find.text('Full Name'), findsNothing);
    expect(find.text('Nama'), findsNothing);
    expect(find.text('Nama Lengkap'), findsNothing);

    usernameController.dispose();
    bioController.dispose();
  });

  test(
    'FirebaseAuthCoreService signUpWithEmail never writes Firebase displayName',
    () async {
      final metadata = _MockUserMetadata(
        creationTimeValue: DateTime.parse('2026-06-01T00:00:00.000Z'),
        lastSignInTimeValue: DateTime.parse('2026-06-02T00:00:00.000Z'),
      );
      final user = _MockUser(
        uidValue: 'firebase-user-1',
        emailValue: 'yayan@example.com',
        photoUrlValue: 'https://example.com/avatar.png',
        phoneNumberValue: null,
        emailVerifiedValue: true,
        metadataValue: metadata,
        providerDataValue: const <UserInfo>[],
      );
      final credential = _MockUserCredential(user);
      final firebaseAuth = _MockFirebaseAuth(createUserCredential: credential);

      final service = FirebaseAuthCoreService(firebaseAuth: firebaseAuth);
      final result = await service.signUpWithEmail(
        'yayan@example.com',
        'password123',
      );

      expect(result.isSuccess, isTrue);
      expect(firebaseAuth.createUserCalls, 1);
      expect(user.updateDisplayNameCalls, 0);
      expect(user.reloadCalls, 0);
    },
  );

  test(
    'AuthProfileRepository updateProfile preserves avatar updates and never writes Firebase displayName',
    () async {
      final metadata = _MockUserMetadata(
        creationTimeValue: DateTime.parse('2026-06-01T00:00:00.000Z'),
        lastSignInTimeValue: DateTime.parse('2026-06-02T00:00:00.000Z'),
      );
      final user = _MockUser(
        uidValue: 'firebase-user-1',
        emailValue: 'yayan@example.com',
        photoUrlValue: 'https://example.com/avatar.png',
        phoneNumberValue: null,
        emailVerifiedValue: true,
        metadataValue: metadata,
        providerDataValue: const <UserInfo>[],
      );
      final datasource = _RecordingAuthApiDatasource();

      final firebaseAuth = _MockFirebaseAuth(currentUserValue: user);

      final repository = AuthProfileRepository(
        firebaseAuth: firebaseAuth,
        apiDatasource: datasource,
      );

      final result = await repository.updateProfile(
        photoUrl: 'https://example.com/avatar-updated.png',
        username: 'yayan',
        bio: 'bio',
      );

      expect(result.isSuccess, isTrue);
      expect(result.data, isA<UserProfilePatch>());
      expect(result.data?.username, equals('yayan'));
      expect(
        result.data?.photoUrl,
        equals('https://example.com/avatar-updated.png'),
      );
      expect(datasource.recordedUsername, equals('yayan'));
      expect(
        datasource.recordedPhotoUrl,
        equals('https://example.com/avatar-updated.png'),
      );
      expect(datasource.recordedBio, equals('bio'));
      expect(user.updatePhotoUrlCalls, 0);
      expect(user.lastPhotoUrlArg, isNull);
      expect(user.updateDisplayNameCalls, 0);
      expect(user.reloadCalls, 1);
    },
  );

  testWidgets('Complete profile submits username only', (tester) async {
    final authController = _FakeAuthController(
      const AuthState.requiresProfileCompletion(
        userId: 'firebase-user-1',
        email: 'yayan@example.com',
      ),
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          authControllerProvider.overrideWith(() => authController),
          usernameServiceProvider.overrideWithValue(_FakeUsernameService()),
        ],
        child: const MaterialApp(home: CompleteProfileScreen()),
      ),
    );
    await tester.pump();

    expect(find.byType(TextField), findsOneWidget);
    expect(find.text('seeded_username'), findsNothing);

    await tester.enterText(find.byType(TextField), 'seeded_username');
    await tester.pump(const Duration(milliseconds: 600));
    await tester.pump();

    final button = tester.widget<ElevatedButton>(find.byType(ElevatedButton));
    expect(button.onPressed, isNotNull);

    await tester.tap(find.text('Complete Profile'));
    await tester.pumpAndSettle();

    expect(authController.lastCompletedUsername, equals('seeded_username'));
  });
}
