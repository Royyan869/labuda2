import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:dio/dio.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart';
import 'package:labuda/domains/user/profile/data/services/avatar_upload_service.dart';
import 'package:labuda/domains/user/profile/data/services/cover_photo_upload_service.dart';
import 'package:labuda/domains/user/profile/domain/entities/profile_entity.dart';
import 'package:labuda/domains/user/profile/domain/repositories/i_profile_repository.dart';
import 'package:labuda/domains/user/profile/domain/use_cases/update_profile_use_case.dart';
import 'package:labuda/domains/user/profile/presentation/providers/profile_core_provider.dart'
    show
        ProfileActions,
        profileCoverPublicationRevisionProvider,
        profileActionsProvider,
        profilePublicationProvider,
        storeImagePublicationRevisionProvider;
import 'package:labuda/domains/user/profile/presentation/screens/edit_profile/edit_profile_save_handler.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/services/logger_service.dart';
import 'package:labuda/shared/widgets/stable_network_image.dart';
import 'package:labuda/domains/user/preference/seller/data/seller_providers.dart'
    show sellerRemoteDatasourceProvider, storePhotoUploadServiceProvider;
import 'package:labuda/domains/user/preference/seller/data/remote/seller_remote_datasource.dart';
import 'package:labuda/domains/user/preference/seller/data/services/store_photo_upload_service.dart';

class _RecordingAuthController extends AuthController {
  _RecordingAuthController(this._state) {
    _syncPrincipal(_state);
  }

  AuthState _state;
  String? _activePrincipalUid;
  int _principalEpoch = 0;
  int forceRefreshCalls = 0;
  int updateProfileCalls = 0;
  bool throwOnRefresh = false;
  bool refreshWasRequested = false;
  AuthState? forcedStateAfterUpdateProfile;

  @override
  String? get activePrincipalUid => _activePrincipalUid;

  @override
  int get principalEpoch => _principalEpoch;

  @override
  AuthState build() => _state;

  void setAuthState(AuthState next) {
    _state = next;
    state = next;
    _syncPrincipal(next);
  }

  void _syncPrincipal(AuthState next) {
    final nextUid = next is AuthStateAuthenticated ? next.user.id : null;
    if (_activePrincipalUid != nextUid) {
      _principalEpoch += 1;
    }
    _activePrincipalUid = nextUid;
  }

  @override
  Future<bool> updateProfile({
    String? photoUrl,
    String? username,
    String? bio,
    String? phoneNumber,
    String? location,
    DateTime? phoneVerifiedAt,
    DateTime? dateOfBirth,
    bool avatarCleared = false,
    bool refreshAuthState = true,
  }) async {
    updateProfileCalls += 1;
    refreshWasRequested = refreshAuthState;

    final current = state;
    if (forcedStateAfterUpdateProfile != null) {
      state = forcedStateAfterUpdateProfile!;
      _syncPrincipal(forcedStateAfterUpdateProfile!);
    } else if (current is AuthStateAuthenticated) {
      state = AuthStateAuthenticated(
        current.user.copyWith(
          avatarUrl: avatarCleared
              ? null
              : (photoUrl ?? current.user.avatarUrl),
          username: username ?? current.user.username,
          bio: bio ?? current.user.bio,
          phoneNumber: phoneNumber ?? current.user.phoneNumber,
          phoneVerifiedAt: phoneVerifiedAt ?? current.user.phoneVerifiedAt,
          dateOfBirth: dateOfBirth ?? current.user.dateOfBirth,
        ),
        emailVerified: current.emailVerified,
      );
    }

    if (refreshAuthState) {
      await forceRefreshAuthState();
    }

    return true;
  }

  @override
  Future<void> forceRefreshAuthState() async {
    forceRefreshCalls += 1;
    if (throwOnRefresh) {
      throw StateError('forced refresh failure');
    }
  }

  @override
  void publishStoreIdentity({
    String? storeName,
    String? storeImageUrl,
    bool clearStoreImage = false,
  }) {
    final current = state;
    if (current is! AuthStateAuthenticated) return;
    final updated = current.user.copyWith(
      storeName: storeName ?? current.user.storeName,
      storeImageUrl: clearStoreImage
          ? null
          : (storeImageUrl ?? current.user.storeImageUrl),
    );
    state = AuthStateAuthenticated(
      updated,
      emailVerified: current.emailVerified,
    );
  }
}

class _FakeProfileRepository implements IProfileRepository {
  _FakeProfileRepository(this.profile);

  final ProfileEntity profile;
  int getProfileCalls = 0;
  int watchProfileCalls = 0;

  @override
  Future<Result<ProfileEntity?>> getProfile(String userId) async {
    getProfileCalls += 1;
    return Result.success(profile);
  }

  @override
  Stream<ProfileEntity?> watchProfile(String userId) {
    watchProfileCalls += 1;
    return Stream.value(profile);
  }

  @override
  Future<Result<bool>> profileExists(String userId) async =>
      Result.success(true);

  @override
  Future<Result<ProfileStats>> getProfileStats(String userId) async =>
      Result.success(const ProfileStats(followersCount: 0, followingCount: 0));

  @override
  Future<Result<ProfileEntity>> createProfile(ProfileEntity profile) =>
      throw UnimplementedError();

  @override
  Future<Result<ProfileEntity>> updateProfile(ProfileEntity profile) =>
      throw UnimplementedError();

  @override
  Future<Result<List<ProfileEntity>>> getMultipleProfiles(
    List<String> userIds,
  ) async => Result.success(const <ProfileEntity>[]);

  @override
  Future<Result<List<ProfileEntity>>> getProfilesByType(
    UserRole userRole, {
    int limit = 20,
    String? lastDocumentId,
  }) async => Result.success(const <ProfileEntity>[]);

  @override
  Future<Result<List<ProfileEntity>>> getTrendingProfiles({
    int limit = 10,
  }) async => Result.success(const <ProfileEntity>[]);

  @override
  Future<Result<List<ProfileEntity>>> searchProfiles(
    String query, {
    int limit = 20,
    String? lastDocumentId,
  }) async => Result.success(const <ProfileEntity>[]);

  @override
  Future<Result<List<ProfileEntity>>> getVerifiedSellers({
    int limit = 20,
    String? lastDocumentId,
  }) => throw UnimplementedError();

  @override
  noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _FakeProfileActions extends ProfileActions {
  _FakeProfileActions(this.publishedProfile, {this.throwOnUpdate = false})
    : super(
        updateUseCase: _NoopUpdateProfileUseCase(),
        onProfileUpdated: () {},
      );

  final ProfileEntity publishedProfile;
  final bool throwOnUpdate;
  Map<String, dynamic>? lastFields;
  int updateFieldsCalls = 0;

  @override
  Future<ProfileEntity> updateFields(
    ProfileEntity currentProfile,
    Map<String, dynamic> fields,
  ) async {
    updateFieldsCalls += 1;
    lastFields = Map<String, dynamic>.from(fields);
    if (throwOnUpdate) {
      throw StateError('save failed');
    }
    return publishedProfile;
  }
}

class _NoopUpdateProfileUseCase implements UpdateProfileUseCase {
  @override
  Future<Result<ProfileEntity>> call(ProfileEntity profile) async =>
      Result.success(profile);
}

class _NoopAvatarUploadService implements AvatarUploadService {
  @override
  Future<Result<String>> uploadAvatar({
    required String userId,
    required String imagePath,
  }) async => Result.success('images/avatars/$userId.jpg');

  @override
  Future<Result<void>> deleteAvatar(String userId) async =>
      Result.success(null);
}

class _NoopCoverPhotoUploadService implements CoverPhotoUploadService {
  @override
  Future<Result<CoverUploadResult>> uploadCoverPhoto({
    required String userId,
    required String imagePath,
  }) async => Result.success(
    CoverUploadResult(
      storageKey: 'images/profile-covers/$userId.jpg',
      readUrl: 'https://cdn.example.com/media/images/profile-covers/$userId.jpg',
    ),
  );
}

class _FailingCoverPhotoUploadService implements CoverPhotoUploadService {
  @override
  Future<Result<CoverUploadResult>> uploadCoverPhoto({
    required String userId,
    required String imagePath,
  }) async => Result.error('Failed to upload cover photo: backend refused');
}

class _NoopStorePhotoUploadService implements StorePhotoUploadService {
  @override
  Future<Result<String>> uploadStorePhoto({
    required String userId,
    required String imagePath,
  }) async => Result.success('images/stores/$userId.jpg');

  @override
  Future<Result<void>> deleteStorePhoto(String userId) async =>
      Result.success(null);
}

class _NoopSellerRemoteDatasource extends SellerRemoteDatasource {
  _NoopSellerRemoteDatasource()
    : super(
        apiClient: const _NoopSellerApiClient(),
        logger: LoggerService.instance,
      );

  int updateStoreProfileCalls = 0;

  @override
  Future<Map<String, dynamic>> updateStoreProfile({
    String? storeName,
    String? storeImageUrl,
  }) async {
    updateStoreProfileCalls += 1;
    return <String, dynamic>{
      'store_name': storeName,
      'store_image_url': storeImageUrl,
    };
  }

  @override
  Future<Map<String, dynamic>> updateStoreImage({String? storeImageUrl}) async {
    updateStoreProfileCalls += 1;
    return <String, dynamic>{'store_image_url': storeImageUrl};
  }
}

class _NoopSellerApiClient implements ApiClient {
  const _NoopSellerApiClient();

  @override
  Dio get dio => throw UnimplementedError();

  @override
  Future<Response<T>> delete<T>(
    String path, {
    dynamic data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) => throw UnimplementedError();

  @override
  ApiException extractException(DioException e) =>
      UnknownApiException(message: e.message ?? 'unknown');

  @override
  Future<Response<T>> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) => throw UnimplementedError();

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
  }) => throw UnimplementedError();

  @override
  Future<Response<T>> post<T>(
    String path, {
    dynamic data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) => throw UnimplementedError();

  @override
  Future<Response<T>> put<T>(
    String path, {
    dynamic data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) => throw UnimplementedError();

  @override
  Future<Response<T>> uploadFile<T>(
    String path, {
    required String filePath,
    required String fieldName,
    Map<String, dynamic>? additionalFields,
    Options? options,
    CancelToken? cancelToken,
    void Function(int, int)? onSendProgress,
  }) => throw UnimplementedError();
}

class _TestEditProfileHost extends ConsumerStatefulWidget {
  const _TestEditProfileHost({
    super.key,
    required this.userId,
    required this.cachedProfile,
    required this.isSeller,
    required this.profileActions,
  });

  final String userId;
  final ProfileEntity cachedProfile;
  final bool isSeller;
  final ProfileActions profileActions;

  @override
  ConsumerState<_TestEditProfileHost> createState() =>
      _TestEditProfileHostState();
}

class _TestEditProfileHostState extends ConsumerState<_TestEditProfileHost>
    with EditProfileSaveHandler<_TestEditProfileHost> {
  final GlobalKey<FormState> _formKey = GlobalKey<FormState>();
  final TextEditingController _usernameController = TextEditingController();
  final TextEditingController _bioController = TextEditingController();
  final TextEditingController _storeNameController = TextEditingController();
  final TextEditingController _websiteController = TextEditingController();
  final TextEditingController _instagramController = TextEditingController();
  final TextEditingController _facebookController = TextEditingController();
  final TextEditingController _tiktokController = TextEditingController();
  final TextEditingController _twitterController = TextEditingController();

  bool loading = false;
  String? _avatarUrl;
  String? _profileCoverUrl;
  String? _profileCoverFile;
  String? _profileCoverKey;
  String? _storeImageUrl;
  String? _selectedAvatarPath;
  String? _selectedStorePhotoPath;
  bool _isAvatarMarkedForRemoval = false;
  bool _isProfileCoverMarkedForRemoval = false;
  bool _isStorePhotoMarkedForRemoval = false;
  bool throwOnPublicationRefresh = false;
  bool throwOnNavigation = false;
  DateTime? _establishedDate;
  final bool _isEmailPublic = true;
  final bool _isPhonePublic = true;
  final bool _isSocialMediaPublic = true;

  @override
  void initState() {
    super.initState();
    _usernameController.text = 'owner';
    _bioController.text = 'Bio';
  }

  @override
  GlobalKey<FormState> get formKey => _formKey;

  @override
  String get actualUserId => widget.userId;

  @override
  bool get isSeller => widget.isSeller;

  @override
  ProfileEntity? get cachedProfile => widget.cachedProfile;

  @override
  TextEditingController get usernameController => _usernameController;

  @override
  TextEditingController get bioController => _bioController;

  @override
  TextEditingController get storeNameController => _storeNameController;

  @override
  TextEditingController get websiteController => _websiteController;

  @override
  TextEditingController get instagramController => _instagramController;

  @override
  TextEditingController get facebookController => _facebookController;

  @override
  TextEditingController get tiktokController => _tiktokController;

  @override
  TextEditingController get twitterController => _twitterController;

  @override
  String? get avatarUrl => _avatarUrl;

  void setAvatarUrl(String? value) => _avatarUrl = value;

  @override
  String? get selectedAvatarPath => _selectedAvatarPath;

  void setSelectedAvatarPath(String? value) => _selectedAvatarPath = value;

  @override
  bool get isAvatarMarkedForRemoval => _isAvatarMarkedForRemoval;

  void setAvatarMarkedForRemoval(bool value) =>
      _isAvatarMarkedForRemoval = value;

  @override
  String? get profileCoverUrl => _profileCoverUrl;

  void setProfileCoverUrl(String? value) => _profileCoverUrl = value;

  @override
  String? get profileCoverFile => _profileCoverFile;

  @override
  String? get profileCoverKey => _profileCoverKey;

  void setProfileCoverKey(String? value) => _profileCoverKey = value;

  @override
  bool get isProfileCoverMarkedForRemoval => _isProfileCoverMarkedForRemoval;

  void setProfileCoverMarkedForRemoval(bool value) =>
      _isProfileCoverMarkedForRemoval = value;

  @override
  String? get storeImageUrl => _storeImageUrl;

  void setStoreImageUrl(String? value) => _storeImageUrl = value;

  @override
  String? get selectedStorePhotoPath => _selectedStorePhotoPath;

  void setStoreName(String value) => _storeNameController.text = value;

  void setBio(String value) => _bioController.text = value;

  void setUsername(String value) => _usernameController.text = value;

  void setInstagram(String value) => _instagramController.text = value;

  void setFacebook(String value) => _facebookController.text = value;

  void setTiktok(String value) => _tiktokController.text = value;

  void setTwitter(String value) => _twitterController.text = value;

  void setSelectedStorePhotoPath(String? value) =>
      _selectedStorePhotoPath = value;

  @override
  bool get isStorePhotoMarkedForRemoval => _isStorePhotoMarkedForRemoval;

  void setStorePhotoMarkedForRemoval(bool value) =>
      _isStorePhotoMarkedForRemoval = value;

  @override
  DateTime? get establishedDate => _establishedDate;

  void setEstablishedDate(DateTime? value) => _establishedDate = value;

  @override
  bool get isEmailPublic => _isEmailPublic;

  @override
  bool get isPhonePublic => _isPhonePublic;

  @override
  bool get isSocialMediaPublic => _isSocialMediaPublic;

  @override
  AvatarUploadService get avatarUploadService =>
      ref.read(avatarUploadServiceProvider);

  @override
  CoverPhotoUploadService get coverPhotoUploadService =>
      ref.read(coverPhotoUploadServiceProvider);

  @override
  StorePhotoUploadService get storePhotoUploadService =>
      ref.read(storePhotoUploadServiceProvider);

  @override
  SellerRemoteDatasource get sellerRemoteDatasource =>
      ref.read(sellerRemoteDatasourceProvider);

  @override
  void setLoading(bool loading) => this.loading = loading;

  @override
  bool get isLoading => loading;

  void setCoverPhotoPath(String? path) => _profileCoverFile = path;

  void setStorePhotoPath(String? path) => _selectedStorePhotoPath = path;

  Future<void> triggerSave() => save();

  @override
  Future<void> publishSavedProfileState(ProfileEntity? publishedProfile) async {
    if (throwOnPublicationRefresh) {
      throw StateError('forced publication failure');
    }
    if (publishedProfile == null) return;
    ref.read(profilePublicationProvider(widget.userId).notifier).state =
        publishedProfile;
  }

  @override
  Future<void> publishStoredIdentity({
    String? storeName,
    String? storeImageUrl,
    bool clearStoreImage = false,
  }) async {
    ref
        .read(authControllerProvider.notifier)
        .publishStoreIdentity(
          storeName: storeName,
          storeImageUrl: storeImageUrl,
          clearStoreImage: clearStoreImage,
        );
  }

  @override
  Future<void> completeSaveNavigation() async {
    Navigator.of(context).pop(true);
    if (throwOnNavigation) {
      throw StateError('forced navigation failure');
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: Form(key: _formKey, child: const SizedBox.shrink()),
    );
  }
}

class _AutoPushRouteHost extends StatefulWidget {
  const _AutoPushRouteHost({required this.child});

  final Widget child;

  @override
  State<_AutoPushRouteHost> createState() => _AutoPushRouteHostState();
}

class _AutoPushRouteHostState extends State<_AutoPushRouteHost> {
  bool _pushed = false;

  @override
  Widget build(BuildContext context) {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (_pushed || !mounted) return;
      _pushed = true;
      Navigator.of(
        context,
      ).push(MaterialPageRoute(builder: (_) => widget.child));
    });

    return const Scaffold(body: SizedBox.shrink());
  }
}

class _ResultCapturingRouteHost extends StatefulWidget {
  const _ResultCapturingRouteHost({
    required this.child,
    required this.onResult,
  });

  final Widget child;
  final void Function(bool? saved) onResult;

  @override
  State<_ResultCapturingRouteHost> createState() =>
      _ResultCapturingRouteHostState();
}

class _ResultCapturingRouteHostState extends State<_ResultCapturingRouteHost> {
  bool _pushed = false;

  @override
  Widget build(BuildContext context) {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (_pushed || !mounted) return;
      _pushed = true;
      Navigator.of(
        context,
      ).push(MaterialPageRoute(builder: (_) => widget.child)).then((saved) {
        widget.onResult(saved);
      });
    });

    return const Scaffold(body: SizedBox.shrink());
  }
}

Future<void> _runEditProfileSaveScenario(
  WidgetTester tester, {
  required AuthUser user,
  required ProfileEntity cachedProfile,
  required ProfileEntity publishedProfile,
  required void Function(_TestEditProfileHostState state) configureState,
  Future<void> Function(_TestEditProfileHostState state)? beforeSave,
  required void Function(
    ProviderContainer container,
    _RecordingAuthController authController,
    _FakeProfileActions fakeActions,
    bool? savedResult,
    _TestEditProfileHostState state,
  )
  verify,
}) async {
  final fakeActions = _FakeProfileActions(publishedProfile);
  final authController = _RecordingAuthController(
    AuthState.authenticated(user, emailVerified: true),
  );
  final editorKey = GlobalKey<_TestEditProfileHostState>();
  final container = ProviderContainer(
    overrides: [
      authControllerProvider.overrideWith(() => authController),
      profileActionsProvider.overrideWithValue(fakeActions),
      avatarUploadServiceProvider.overrideWithValue(_NoopAvatarUploadService()),
      coverPhotoUploadServiceProvider.overrideWithValue(
        _NoopCoverPhotoUploadService(),
      ),
      storePhotoUploadServiceProvider.overrideWithValue(
        _NoopStorePhotoUploadService(),
      ),
      sellerRemoteDatasourceProvider.overrideWithValue(
        _NoopSellerRemoteDatasource(),
      ),
      profileRepositoryProvider.overrideWithValue(
        _FakeProfileRepository(cachedProfile),
      ),
    ],
  );
  addTearDown(container.dispose);
  bool? savedResult;

  await tester.pumpWidget(
    UncontrolledProviderScope(
      container: container,
      child: MaterialApp(
        home: _ResultCapturingRouteHost(
          onResult: (saved) => savedResult = saved,
          child: _TestEditProfileHost(
            key: editorKey,
            userId: user.id,
            cachedProfile: cachedProfile,
            isSeller: true,
            profileActions: fakeActions,
          ),
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();

  final state = editorKey.currentState!;
  configureState(state);

  if (beforeSave != null) {
    await beforeSave(state);
  } else {
    await state.triggerSave();
  }
  await tester.pumpAndSettle();

  verify(container, authController, fakeActions, savedResult, state);
}

class _DelayedImageHttpClient implements HttpClient {
  _DelayedImageHttpClient(this._responders);

  final Map<String, _DelayedResponse> _responders;

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
    final responder = _responders[url.toString()];
    if (responder == null) {
      throw StateError('No responder for $url');
    }
    return _DelayedImageHttpClientRequest(responder);
  }

  @override
  noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _DelayedImageHttpClientRequest implements HttpClientRequest {
  _DelayedImageHttpClientRequest(this._responder);

  final _DelayedResponse _responder;

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

class _DelayedResponse {
  _DelayedResponse._(this.response, this.complete);

  final HttpClientResponse response;
  final void Function(List<int> bytes) complete;

  factory _DelayedResponse.pending() {
    final controller = StreamController<List<int>>();
    final response = _DelayedImageHttpClientResponse(controller.stream);
    return _DelayedResponse._(response, (bytes) {
      controller.add(bytes);
      controller.close();
    });
  }
}

class _DelayedImageHttpClientResponse extends StreamView<List<int>>
    implements HttpClientResponse {
  _DelayedImageHttpClientResponse(super.stream);

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

ProfileEntity _sellerProfile({
  required String userId,
  required String? coverPhotoUrl,
  required String? storeImageUrl,
  DateTime? updatedAt,
}) {
  return ProfileEntity(
    id: 'profile-$userId',
    userId: userId,
    location: 'Bandung',
    coverPhotoUrl: coverPhotoUrl,
    updatedAt: updatedAt ?? DateTime.utc(2026, 6, 2),
    joinedAt: DateTime.utc(2026, 1, 1),
    lastActiveAt: DateTime.utc(2026, 6, 1),
    stats: const ProfileStats(followersCount: 2, followingCount: 1),
    verification: const UserVerificationInfo(
      isPhoneVerified: true,
      isEmailVerified: true,
      isIdVerified: false,
      isFarmVerified: true,
      badges: <ProfileBadge>[],
    ),
    contactInfo: const ContactInfo(
      maskedEmail: 'seller***@example.com',
      maskedPhone: '0812***6789',
      isEmailPublic: true,
      isPhonePublic: true,
      instagramHandle: 'seller_farm',
      facebookHandle: 'seller_farm',
      tiktokHandle: 'seller_farm',
      twitterHandle: 'seller_farm',
      isSocialMediaPublic: true,
    ),
  );
}

AuthUser _sellerUser({
  required String id,
  required String username,
  required String avatarUrl,
  required String storeName,
  required String storeImageUrl,
}) {
  return AuthUser(
    id: id,
    createdAt: DateTime.utc(2026, 6, 1),
    updatedAt: DateTime.utc(2026, 6, 1),
    email: '$username@example.com',
    username: username,
    avatarUrl: avatarUrl,
    bio: 'Bio for $username',
    isEmailVerified: true,
    accountStatus: AccountStatus.active,
    roles: const [UserRole.user],
    provider: ShonaAuthProvider.email,
    lifecycle: ContentLifecycle.active,
    hasSellerProfile: true,
    sellerSubscriptionStatus: 'active',
    hasMarketAuthority: true,
    storeName: storeName,
    storeImageUrl: storeImageUrl,
  );
}

void main() {
  testWidgets(
    'save path publishes store and cover identity immediately without forcing auth refresh',
    (tester) async {
      final user = _sellerUser(
        id: 'seller-1',
        username: 'seller',
        avatarUrl: 'https://example.com/avatar-old.jpg',
        storeName: 'Seller Farm',
        storeImageUrl: 'https://example.com/store-old.jpg',
      );
      final cachedProfile = _sellerProfile(
        userId: user.id,
        coverPhotoUrl: 'https://example.com/cover-old.jpg',
        storeImageUrl: 'https://example.com/store-old.jpg',
        updatedAt: DateTime.utc(2026, 6, 2, 10, 0),
      );
      final publishedProfile = _sellerProfile(
        userId: user.id,
        coverPhotoUrl: 'https://example.com/cover-new.jpg',
        storeImageUrl: 'https://example.com/store-new.jpg',
        updatedAt: DateTime.utc(2026, 6, 2, 10, 5),
      );
      final fakeActions = _FakeProfileActions(publishedProfile);
      final authController = _RecordingAuthController(
        AuthState.authenticated(user, emailVerified: true),
      );
      authController.throwOnRefresh = true;
      final editorKey = GlobalKey<_TestEditProfileHostState>();
      final container = ProviderContainer(
        overrides: [
          authControllerProvider.overrideWith(() => authController),
          profileActionsProvider.overrideWithValue(fakeActions),
          avatarUploadServiceProvider.overrideWithValue(
            _NoopAvatarUploadService(),
          ),
          coverPhotoUploadServiceProvider.overrideWithValue(
            _NoopCoverPhotoUploadService(),
          ),
          storePhotoUploadServiceProvider.overrideWithValue(
            _NoopStorePhotoUploadService(),
          ),
          sellerRemoteDatasourceProvider.overrideWithValue(
            _NoopSellerRemoteDatasource(),
          ),
          profileRepositoryProvider.overrideWithValue(
            _FakeProfileRepository(cachedProfile),
          ),
        ],
      );
      addTearDown(container.dispose);
      bool? savedResult;

      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: MaterialApp(
            home: _ResultCapturingRouteHost(
              onResult: (saved) => savedResult = saved,
              child: _TestEditProfileHost(
                key: editorKey,
                userId: user.id,
                cachedProfile: cachedProfile,
                isSeller: true,
                profileActions: fakeActions,
              ),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();

      final state = editorKey.currentState!;
      state.setCoverPhotoPath('/tmp/cover.jpg');
      state.setStorePhotoPath('/tmp/store.jpg');
      await state.triggerSave();
      await tester.pumpAndSettle();

      expect(authController.forceRefreshCalls, 0);
      expect(authController.refreshWasRequested, isFalse);
      expect(fakeActions.updateFieldsCalls, 1);
      expect(
        fakeActions.lastFields,
        isNotNull,
        reason: 'cover and contact updates must be published from one save',
      );
      expect(authController.state, isA<AuthStateAuthenticated>());
      final updated = authController.state as AuthStateAuthenticated;
      expect(updated.user.storeImageUrl, 'images/stores/seller-1.jpg');
      expect(
        container.read(profilePublicationProvider(user.id)),
        publishedProfile,
      );
      expect(
        container.read(profileCoverPublicationRevisionProvider(user.id)),
        1,
      );
      expect(container.read(storeImagePublicationRevisionProvider(user.id)), 1);
      expect(savedResult, isTrue);
    },
  );

  testWidgets(
    'save path skips profile and store mutations when nothing is dirty',
    (tester) async {
      final user = AuthUser(
        id: 'seller-1',
        createdAt: DateTime.utc(2026, 6, 1),
        updatedAt: DateTime.utc(2026, 6, 1),
        email: 'owner@example.com',
        username: 'owner',
        avatarUrl: null,
        bio: 'Bio',
        isEmailVerified: true,
        accountStatus: AccountStatus.active,
        roles: const [UserRole.user],
        provider: ShonaAuthProvider.email,
        lifecycle: ContentLifecycle.active,
        hasSellerProfile: true,
        sellerSubscriptionStatus: 'active',
        hasMarketAuthority: true,
        storeName: '',
        storeImageUrl: null,
      );
      final cachedProfile =
          _sellerProfile(
            userId: user.id,
            coverPhotoUrl: 'https://example.com/cover-old.jpg',
            storeImageUrl: null,
          ).copyWith(
            contactInfo: const ContactInfo(
              maskedEmail: 'owner***@example.com',
              maskedPhone: '0812***6789',
              isEmailPublic: true,
              isPhonePublic: true,
              instagramHandle: null,
              facebookHandle: null,
              tiktokHandle: null,
              twitterHandle: null,
              isSocialMediaPublic: true,
            ),
          );
      final fakeActions = _FakeProfileActions(cachedProfile);
      final authController = _RecordingAuthController(
        AuthState.authenticated(user, emailVerified: true),
      );
      final sellerDatasource = _NoopSellerRemoteDatasource();
      final container = ProviderContainer(
        overrides: [
          authControllerProvider.overrideWith(() => authController),
          profileActionsProvider.overrideWithValue(fakeActions),
          avatarUploadServiceProvider.overrideWithValue(
            _NoopAvatarUploadService(),
          ),
          coverPhotoUploadServiceProvider.overrideWithValue(
            _NoopCoverPhotoUploadService(),
          ),
          storePhotoUploadServiceProvider.overrideWithValue(
            _NoopStorePhotoUploadService(),
          ),
          sellerRemoteDatasourceProvider.overrideWithValue(sellerDatasource),
          profileRepositoryProvider.overrideWithValue(
            _FakeProfileRepository(cachedProfile),
          ),
        ],
      );
      addTearDown(container.dispose);
      bool? savedResult;

      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: MaterialApp(
            home: _ResultCapturingRouteHost(
              onResult: (saved) => savedResult = saved,
              child: _TestEditProfileHost(
                userId: user.id,
                cachedProfile: cachedProfile,
                isSeller: true,
                profileActions: fakeActions,
              ),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();

      final state = tester.state<_TestEditProfileHostState>(
        find.byType(_TestEditProfileHost),
      );
      await state.triggerSave();
      await tester.pumpAndSettle();

      expect(authController.forceRefreshCalls, 0);
      expect(authController.refreshWasRequested, isFalse);
      expect(fakeActions.updateFieldsCalls, 0);
      expect(sellerDatasource.updateStoreProfileCalls, 0);
      expect(savedResult, isTrue);
      expect(find.byType(_TestEditProfileHost), findsNothing);
    },
  );

  testWidgets('save path separates mutation success from navigation failure', (
    tester,
  ) async {
    final user = _sellerUser(
      id: 'seller-2',
      username: 'seller2',
      avatarUrl: 'https://example.com/avatar-old.jpg',
      storeName: 'Seller Farm',
      storeImageUrl: 'https://example.com/store-old.jpg',
    );
    final cachedProfile = _sellerProfile(
      userId: user.id,
      coverPhotoUrl: 'https://example.com/cover-old.jpg',
      storeImageUrl: 'https://example.com/store-old.jpg',
    );
    final publishedProfile = _sellerProfile(
      userId: user.id,
      coverPhotoUrl: 'https://example.com/cover-new.jpg',
      storeImageUrl: 'https://example.com/store-old.jpg',
    );
    final fakeActions = _FakeProfileActions(publishedProfile);
    final authController = _RecordingAuthController(
      AuthState.authenticated(user, emailVerified: true),
    );
    final editorKey = GlobalKey<_TestEditProfileHostState>();
    final container = ProviderContainer(
      overrides: [
        authControllerProvider.overrideWith(() => authController),
        profileActionsProvider.overrideWithValue(fakeActions),
        avatarUploadServiceProvider.overrideWithValue(
          _NoopAvatarUploadService(),
        ),
        coverPhotoUploadServiceProvider.overrideWithValue(
          _NoopCoverPhotoUploadService(),
        ),
        storePhotoUploadServiceProvider.overrideWithValue(
          _NoopStorePhotoUploadService(),
        ),
        sellerRemoteDatasourceProvider.overrideWithValue(
          _NoopSellerRemoteDatasource(),
        ),
        profileRepositoryProvider.overrideWithValue(
          _FakeProfileRepository(cachedProfile),
        ),
      ],
    );
    addTearDown(container.dispose);
    Object? navigationError;
    bool? savedResult;

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          home: _ResultCapturingRouteHost(
            onResult: (saved) {
              savedResult = saved;
              if (saved != true) return;
              try {
                throw StateError('navigation failed');
              } catch (error) {
                navigationError = error;
              }
            },
            child: _TestEditProfileHost(
              key: editorKey,
              userId: user.id,
              cachedProfile: cachedProfile,
              isSeller: true,
              profileActions: fakeActions,
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    final state = editorKey.currentState!;
    state.setProfileCoverUrl('https://example.com/cover-old.jpg');
    state.setCoverPhotoPath('/tmp/cover-fail.jpg');
    await state.triggerSave();
    await tester.pumpAndSettle();

    expect(fakeActions.updateFieldsCalls, 1);
    expect(savedResult, isTrue);
    expect(navigationError, isA<StateError>());
    expect(
      container.read(profilePublicationProvider(user.id)),
      publishedProfile,
    );
    expect(
      find.text('Profile saved, but navigation back failed.'),
      findsNothing,
      reason: 'navigation errors should not be surfaced as mutation failures',
    );
  });

  testWidgets(
    'mutation failure keeps the editor open and shows the failure snackbar',
    (tester) async {
      final user = _sellerUser(
        id: 'seller-3',
        username: 'seller3',
        avatarUrl: 'https://example.com/avatar-old.jpg',
        storeName: 'Seller Farm',
        storeImageUrl: 'https://example.com/store-old.jpg',
      );
      final cachedProfile = _sellerProfile(
        userId: user.id,
        coverPhotoUrl: 'https://example.com/cover-old.jpg',
        storeImageUrl: 'https://example.com/store-old.jpg',
      );
      final publishedProfile = _sellerProfile(
        userId: user.id,
        coverPhotoUrl: 'https://example.com/cover-new.jpg',
        storeImageUrl: 'https://example.com/store-old.jpg',
      );
      final fakeActions = _FakeProfileActions(
        publishedProfile,
        throwOnUpdate: true,
      );
      final authController = _RecordingAuthController(
        AuthState.authenticated(user, emailVerified: true),
      );
      final editorKey = GlobalKey<_TestEditProfileHostState>();
      final container = ProviderContainer(
        overrides: [
          authControllerProvider.overrideWith(() => authController),
          profileActionsProvider.overrideWithValue(fakeActions),
          avatarUploadServiceProvider.overrideWithValue(
            _NoopAvatarUploadService(),
          ),
          coverPhotoUploadServiceProvider.overrideWithValue(
            _NoopCoverPhotoUploadService(),
          ),
          storePhotoUploadServiceProvider.overrideWithValue(
            _NoopStorePhotoUploadService(),
          ),
          sellerRemoteDatasourceProvider.overrideWithValue(
            _NoopSellerRemoteDatasource(),
          ),
          profileRepositoryProvider.overrideWithValue(
            _FakeProfileRepository(cachedProfile),
          ),
        ],
      );
      addTearDown(container.dispose);

      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: MaterialApp(
            home: _ResultCapturingRouteHost(
              onResult: (_) {},
              child: _TestEditProfileHost(
                key: editorKey,
                userId: user.id,
                cachedProfile: cachedProfile,
                isSeller: true,
                profileActions: fakeActions,
              ),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();

      final state = editorKey.currentState!;
      await state.triggerSave();
      await tester.pumpAndSettle();

      expect(fakeActions.updateFieldsCalls, 1);
      expect(find.byKey(editorKey), findsOneWidget);
      expect(container.read(profilePublicationProvider(user.id)), isNull);
      expect(
        find.text('Perubahan belum bisa disimpan. Coba lagi.'),
        findsOneWidget,
      );
      expect(
        find.text('Profile saved, but navigation back failed.'),
        findsNothing,
      );
    },
  );

  testWidgets('failed cover preview upload does not publish saved state', (
    tester,
  ) async {
    final user = _sellerUser(
      id: 'seller-upload-fail',
      username: 'seller-upload-fail',
      avatarUrl: 'https://example.com/avatar-old.jpg',
      storeName: 'Seller Farm',
      storeImageUrl: 'https://example.com/store-old.jpg',
    );
    final cachedProfile = _sellerProfile(
      userId: user.id,
      coverPhotoUrl: 'https://example.com/cover-old.jpg',
      storeImageUrl: 'https://example.com/store-old.jpg',
    );
    final fakeActions = _FakeProfileActions(cachedProfile);
    final authController = _RecordingAuthController(
      AuthState.authenticated(user, emailVerified: true),
    );
    final container = ProviderContainer(
      overrides: [
        authControllerProvider.overrideWith(() => authController),
        profileActionsProvider.overrideWithValue(fakeActions),
        avatarUploadServiceProvider.overrideWithValue(
          _NoopAvatarUploadService(),
        ),
        coverPhotoUploadServiceProvider.overrideWithValue(
          _FailingCoverPhotoUploadService(),
        ),
        storePhotoUploadServiceProvider.overrideWithValue(
          _NoopStorePhotoUploadService(),
        ),
        sellerRemoteDatasourceProvider.overrideWithValue(
          _NoopSellerRemoteDatasource(),
        ),
        profileRepositoryProvider.overrideWithValue(
          _FakeProfileRepository(cachedProfile),
        ),
      ],
    );
    addTearDown(container.dispose);
    bool? savedResult;
    final editorKey = GlobalKey<_TestEditProfileHostState>();

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          home: _ResultCapturingRouteHost(
            onResult: (saved) => savedResult = saved,
            child: _TestEditProfileHost(
              key: editorKey,
              userId: user.id,
              cachedProfile: cachedProfile,
              isSeller: true,
              profileActions: fakeActions,
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    final state = editorKey.currentState!;
    state.setProfileCoverUrl('https://example.com/cover-old.jpg');
    state.setCoverPhotoPath('/tmp/cover-fail.jpg');
    await state.triggerSave();
    await tester.pumpAndSettle();

    expect(savedResult, isNull);
    expect(fakeActions.updateFieldsCalls, 0);
    expect(container.read(profilePublicationProvider(user.id)), isNull);
    expect(
      find.text('Failed to upload cover photo: backend refused'),
      findsOneWidget,
    );
    expect(find.byKey(editorKey), findsOneWidget);
  });

  testWidgets(
    'rapid duplicate save is blocked while the first save is active',
    (tester) async {
      final user = _sellerUser(
        id: 'seller-rapid',
        username: 'sellerRapid',
        avatarUrl: 'https://example.com/avatar-old.jpg',
        storeName: 'Seller Farm',
        storeImageUrl: 'https://example.com/store-old.jpg',
      );
      final cachedProfile = _sellerProfile(
        userId: user.id,
        coverPhotoUrl: 'https://example.com/cover-old.jpg',
        storeImageUrl: 'https://example.com/store-old.jpg',
      );

      await _runEditProfileSaveScenario(
        tester,
        user: user,
        cachedProfile: cachedProfile,
        publishedProfile: cachedProfile,
        configureState: (state) {
          state.setUsername('sellerRapid-renamed');
          state.setBio('Updated bio');
          state.setStoreName('Seller Farm');
          state.setStoreImageUrl('https://example.com/store-old.jpg');
          state.setProfileCoverUrl('https://example.com/cover-old.jpg');
          state.setCoverPhotoPath('/tmp/cover-rapid.jpg');
        },
        verify: (container, authController, fakeActions, savedResult, state) {
          expect(savedResult, isTrue);
          expect(fakeActions.updateFieldsCalls, 1);
          expect(authController.updateProfileCalls, 1);
          expect(
            container.read(profilePublicationProvider(user.id)),
            cachedProfile,
          );
          expect(find.byKey(state.widget.key!), findsNothing);
        },
        beforeSave: (state) async {
          final first = state.triggerSave();
          final second = state.triggerSave();
          await Future.wait([first, second]);
        },
      );
    },
  );

  testWidgets('store name-only update stays canonical and immediate', (
    tester,
  ) async {
    final user = _sellerUser(
      id: 'seller-4',
      username: 'seller4',
      avatarUrl: 'https://example.com/avatar-old.jpg',
      storeName: 'Seller Farm',
      storeImageUrl: 'https://example.com/store-old.jpg',
    );
    final cachedProfile = _sellerProfile(
      userId: user.id,
      coverPhotoUrl: 'https://example.com/cover-old.jpg',
      storeImageUrl: 'https://example.com/store-old.jpg',
      updatedAt: DateTime.utc(2026, 6, 2, 10, 0),
    );

    await _runEditProfileSaveScenario(
      tester,
      user: user,
      cachedProfile: cachedProfile,
      publishedProfile: cachedProfile,
      configureState: (state) {
        state.setUsername(user.username);
        state.setBio(user.bio ?? '');
        state.setStoreName('Seller Farm Prime');
        state.setStoreImageUrl('https://example.com/store-old.jpg');
      },
      verify: (container, authController, fakeActions, savedResult, state) {
        expect(savedResult, isTrue);
        expect(fakeActions.updateFieldsCalls, 1);
        expect(authController.forceRefreshCalls, 0);
        expect(authController.state, isA<AuthStateAuthenticated>());
        final updated = authController.state as AuthStateAuthenticated;
        expect(updated.user.storeName, 'Seller Farm Prime');
        expect(updated.user.storeImageUrl, 'https://example.com/store-old.jpg');
        expect(
          container.read(profilePublicationProvider(user.id)),
          cachedProfile,
        );
        expect(
          container.read(profileCoverPublicationRevisionProvider(user.id)),
          0,
        );
        expect(
          container.read(storeImagePublicationRevisionProvider(user.id)),
          0,
        );
        expect(
          find.text('Perubahan belum bisa disimpan. Coba lagi.'),
          findsNothing,
        );
        expect(find.byKey(state.widget.key!), findsNothing);
      },
    );
  });

  testWidgets('store image-only update preserves store name', (tester) async {
    final user = _sellerUser(
      id: 'seller-5',
      username: 'seller5',
      avatarUrl: 'https://example.com/avatar-old.jpg',
      storeName: 'Seller Farm',
      storeImageUrl: 'https://example.com/store-old.jpg',
    );
    final cachedProfile = _sellerProfile(
      userId: user.id,
      coverPhotoUrl: 'https://example.com/cover-old.jpg',
      storeImageUrl: 'https://example.com/store-old.jpg',
      updatedAt: DateTime.utc(2026, 6, 2, 10, 0),
    );

    await _runEditProfileSaveScenario(
      tester,
      user: user,
      cachedProfile: cachedProfile,
      publishedProfile: cachedProfile,
      configureState: (state) {
        state.setUsername(user.username);
        state.setBio(user.bio ?? '');
        state.setStoreName('Seller Farm');
        state.setStoreImageUrl('https://example.com/store-old.jpg');
        state.setSelectedStorePhotoPath('/tmp/new-store.jpg');
      },
      verify: (container, authController, fakeActions, savedResult, state) {
        expect(savedResult, isTrue);
        expect(fakeActions.updateFieldsCalls, 1);
        final updated = authController.state as AuthStateAuthenticated;
        expect(updated.user.storeName, 'Seller Farm');
        expect(updated.user.storeImageUrl, 'images/stores/seller-5.jpg');
        expect(
          container.read(profilePublicationProvider(user.id)),
          cachedProfile,
        );
        expect(
          container.read(profileCoverPublicationRevisionProvider(user.id)),
          0,
        );
        expect(
          container.read(storeImagePublicationRevisionProvider(user.id)),
          1,
        );
        expect(
          find.text('Perubahan belum bisa disimpan. Coba lagi.'),
          findsNothing,
        );
        expect(find.byKey(state.widget.key!), findsNothing);
      },
    );
  });

  testWidgets('store removal clears the canonical store image', (tester) async {
    final user = _sellerUser(
      id: 'seller-6',
      username: 'seller6',
      avatarUrl: 'https://example.com/avatar-old.jpg',
      storeName: 'Seller Farm',
      storeImageUrl: 'https://example.com/store-old.jpg',
    );
    final cachedProfile = _sellerProfile(
      userId: user.id,
      coverPhotoUrl: 'https://example.com/cover-old.jpg',
      storeImageUrl: 'https://example.com/store-old.jpg',
      updatedAt: DateTime.utc(2026, 6, 2, 10, 0),
    );

    await _runEditProfileSaveScenario(
      tester,
      user: user,
      cachedProfile: cachedProfile,
      publishedProfile: cachedProfile,
      configureState: (state) {
        state.setUsername(user.username);
        state.setBio(user.bio ?? '');
        state.setStoreName('Seller Farm');
        state.setStoreImageUrl('https://example.com/store-old.jpg');
        state.setStorePhotoMarkedForRemoval(true);
      },
      verify: (container, authController, fakeActions, savedResult, state) {
        expect(savedResult, isTrue);
        expect(fakeActions.updateFieldsCalls, 1);
        final updated = authController.state as AuthStateAuthenticated;
        expect(updated.user.storeName, 'Seller Farm');
        expect(updated.user.storeImageUrl, isNull);
        expect(
          container.read(profilePublicationProvider(user.id)),
          cachedProfile,
        );
        expect(
          container.read(profileCoverPublicationRevisionProvider(user.id)),
          0,
        );
        expect(
          container.read(storeImagePublicationRevisionProvider(user.id)),
          1,
        );
        expect(
          find.text('Perubahan belum bisa disimpan. Coba lagi.'),
          findsNothing,
        );
        expect(find.byKey(state.widget.key!), findsNothing);
      },
    );
  });

  testWidgets('cover update publishes immediately without failure', (
    tester,
  ) async {
    final user = _sellerUser(
      id: 'seller-7',
      username: 'seller7',
      avatarUrl: 'https://example.com/avatar-old.jpg',
      storeName: 'Seller Farm',
      storeImageUrl: 'https://example.com/store-old.jpg',
    );
    final cachedProfile = _sellerProfile(
      userId: user.id,
      coverPhotoUrl: 'https://example.com/cover-old.jpg',
      storeImageUrl: 'https://example.com/store-old.jpg',
      updatedAt: DateTime.utc(2026, 6, 2, 10, 0),
    );
    final publishedProfile = _sellerProfile(
      userId: user.id,
      coverPhotoUrl: 'https://example.com/cover-new.jpg',
      storeImageUrl: 'https://example.com/store-old.jpg',
      updatedAt: DateTime.utc(2026, 6, 2, 10, 5),
    );

    await _runEditProfileSaveScenario(
      tester,
      user: user,
      cachedProfile: cachedProfile,
      publishedProfile: publishedProfile,
      configureState: (state) {
        state.setUsername(user.username);
        state.setBio(user.bio ?? '');
        state.setStoreName('Seller Farm');
        state.setStoreImageUrl('https://example.com/store-old.jpg');
        state.setProfileCoverUrl('https://example.com/cover-old.jpg');
        state.setCoverPhotoPath('/tmp/cover.jpg');
      },
      verify: (container, authController, fakeActions, savedResult, state) {
        expect(savedResult, isTrue);
        expect(fakeActions.updateFieldsCalls, 1);
        expect(
          container.read(profilePublicationProvider(user.id)),
          publishedProfile,
        );
        expect(
          container.read(profileCoverPublicationRevisionProvider(user.id)),
          1,
        );
        expect(
          find.text('Perubahan belum bisa disimpan. Coba lagi.'),
          findsNothing,
        );
        expect(find.byKey(state.widget.key!), findsNothing);
      },
    );
  });

  testWidgets('cover removal keeps the last good frame and saves', (
    tester,
  ) async {
    final user = _sellerUser(
      id: 'seller-8',
      username: 'seller8',
      avatarUrl: 'https://example.com/avatar-old.jpg',
      storeName: 'Seller Farm',
      storeImageUrl: 'https://example.com/store-old.jpg',
    );
    final cachedProfile = _sellerProfile(
      userId: user.id,
      coverPhotoUrl: 'https://example.com/cover-old.jpg',
      storeImageUrl: 'https://example.com/store-old.jpg',
      updatedAt: DateTime.utc(2026, 6, 2, 10, 0),
    );
    final publishedProfile = _sellerProfile(
      userId: user.id,
      coverPhotoUrl: null,
      storeImageUrl: 'https://example.com/store-old.jpg',
      updatedAt: DateTime.utc(2026, 6, 2, 10, 5),
    );

    await _runEditProfileSaveScenario(
      tester,
      user: user,
      cachedProfile: cachedProfile,
      publishedProfile: publishedProfile,
      configureState: (state) {
        state.setUsername(user.username);
        state.setBio(user.bio ?? '');
        state.setStoreName('Seller Farm');
        state.setStoreImageUrl('https://example.com/store-old.jpg');
        state.setProfileCoverUrl('https://example.com/cover-old.jpg');
        state.setProfileCoverMarkedForRemoval(true);
      },
      verify: (container, authController, fakeActions, savedResult, state) {
        expect(savedResult, isTrue);
        expect(fakeActions.updateFieldsCalls, 1);
        expect(
          container.read(profilePublicationProvider(user.id)),
          publishedProfile,
        );
        expect(
          find.text('Perubahan belum bisa disimpan. Coba lagi.'),
          findsNothing,
        );
        expect(find.byKey(state.widget.key!), findsNothing);
      },
    );
  });

  testWidgets('personal avatar updates only the personal identity', (
    tester,
  ) async {
    final user = _sellerUser(
      id: 'seller-9',
      username: 'seller9',
      avatarUrl: 'https://example.com/avatar-old.jpg',
      storeName: 'Seller Farm',
      storeImageUrl: 'https://example.com/store-old.jpg',
    );
    final cachedProfile = _sellerProfile(
      userId: user.id,
      coverPhotoUrl: 'https://example.com/cover-old.jpg',
      storeImageUrl: 'https://example.com/store-old.jpg',
    );

    await _runEditProfileSaveScenario(
      tester,
      user: user,
      cachedProfile: cachedProfile,
      publishedProfile: cachedProfile,
      configureState: (state) {
        state.setUsername(user.username);
        state.setBio(user.bio ?? '');
        state.setStoreName('Seller Farm');
        state.setStoreImageUrl('https://example.com/store-old.jpg');
        state.setSelectedAvatarPath('/tmp/avatar.jpg');
        state.setAvatarUrl('https://example.com/avatar-old.jpg');
      },
      verify: (container, authController, fakeActions, savedResult, state) {
        expect(savedResult, isTrue);
        expect(fakeActions.updateFieldsCalls, 1);
        final updated = authController.state as AuthStateAuthenticated;
        expect(updated.user.avatarUrl, 'images/avatars/seller-9.jpg');
        expect(updated.user.storeName, 'Seller Farm');
        expect(updated.user.storeImageUrl, 'https://example.com/store-old.jpg');
        expect(
          container.read(profilePublicationProvider(user.id)),
          cachedProfile,
        );
        expect(
          find.text('Perubahan belum bisa disimpan. Coba lagi.'),
          findsNothing,
        );
        expect(find.byKey(state.widget.key!), findsNothing);
      },
    );
  });

  testWidgets('personal text updates save immediately without store drift', (
    tester,
  ) async {
    final user = _sellerUser(
      id: 'seller-10',
      username: 'seller10',
      avatarUrl: 'https://example.com/avatar-old.jpg',
      storeName: 'Seller Farm',
      storeImageUrl: 'https://example.com/store-old.jpg',
    );
    final cachedProfile = _sellerProfile(
      userId: user.id,
      coverPhotoUrl: 'https://example.com/cover-old.jpg',
      storeImageUrl: 'https://example.com/store-old.jpg',
    );

    await _runEditProfileSaveScenario(
      tester,
      user: user,
      cachedProfile: cachedProfile,
      publishedProfile: cachedProfile,
      configureState: (state) {
        state.setUsername('seller10-renamed');
        state.setBio('Updated seller bio');
        state.setStoreName('Seller Farm');
        state.setStoreImageUrl('https://example.com/store-old.jpg');
      },
      verify: (container, authController, fakeActions, savedResult, state) {
        expect(savedResult, isTrue);
        expect(fakeActions.updateFieldsCalls, 1);
        final updated = authController.state as AuthStateAuthenticated;
        expect(updated.user.username, 'seller10-renamed');
        expect(updated.user.bio, 'Updated seller bio');
        expect(updated.user.storeName, 'Seller Farm');
        expect(
          container.read(profilePublicationProvider(user.id)),
          cachedProfile,
        );
        expect(
          find.text('Perubahan belum bisa disimpan. Coba lagi.'),
          findsNothing,
        );
        expect(find.byKey(state.widget.key!), findsNothing);
      },
    );
  });

  testWidgets('publication refresh failures do not flip a successful save', (
    tester,
  ) async {
    final user = _sellerUser(
      id: 'seller-11',
      username: 'seller11',
      avatarUrl: 'https://example.com/avatar-old.jpg',
      storeName: 'Seller Farm',
      storeImageUrl: 'https://example.com/store-old.jpg',
    );
    final cachedProfile = _sellerProfile(
      userId: user.id,
      coverPhotoUrl: 'https://example.com/cover-old.jpg',
      storeImageUrl: 'https://example.com/store-old.jpg',
    );

    await _runEditProfileSaveScenario(
      tester,
      user: user,
      cachedProfile: cachedProfile,
      publishedProfile: cachedProfile,
      configureState: (state) {
        state.throwOnPublicationRefresh = true;
        state.setUsername(user.username);
        state.setBio(user.bio ?? '');
        state.setStoreName('Seller Farm Audit');
        state.setStoreImageUrl('https://example.com/store-old.jpg');
        state.setProfileCoverUrl('https://example.com/cover-old.jpg');
        state.setCoverPhotoPath('/tmp/cover-refresh.jpg');
      },
      verify: (container, authController, fakeActions, savedResult, state) {
        expect(savedResult, isTrue);
        expect(fakeActions.updateFieldsCalls, 1);
        expect(authController.state, isA<AuthStateAuthenticated>());
        final updated = authController.state as AuthStateAuthenticated;
        expect(updated.user.storeName, 'Seller Farm Audit');
        expect(
          find.text('Perubahan belum bisa disimpan. Coba lagi.'),
          findsNothing,
        );
        expect(find.byKey(state.widget.key!), findsNothing);
      },
    );
  });

  testWidgets('navigation failures do not turn persistence into failure', (
    tester,
  ) async {
    final user = _sellerUser(
      id: 'seller-12',
      username: 'seller12',
      avatarUrl: 'https://example.com/avatar-old.jpg',
      storeName: 'Seller Farm',
      storeImageUrl: 'https://example.com/store-old.jpg',
    );
    final cachedProfile = _sellerProfile(
      userId: user.id,
      coverPhotoUrl: 'https://example.com/cover-old.jpg',
      storeImageUrl: 'https://example.com/store-old.jpg',
    );
    final publishedProfile = _sellerProfile(
      userId: user.id,
      coverPhotoUrl: 'https://example.com/cover-new.jpg',
      storeImageUrl: 'https://example.com/store-old.jpg',
    );

    await _runEditProfileSaveScenario(
      tester,
      user: user,
      cachedProfile: cachedProfile,
      publishedProfile: publishedProfile,
      configureState: (state) {
        state.throwOnNavigation = true;
        state.setUsername(user.username);
        state.setBio(user.bio ?? '');
        state.setStoreName('Seller Farm');
        state.setStoreImageUrl('https://example.com/store-old.jpg');
        state.setProfileCoverUrl('https://example.com/cover-old.jpg');
        state.setCoverPhotoPath('/tmp/cover-nav.jpg');
      },
      verify: (container, authController, fakeActions, savedResult, state) {
        expect(savedResult, isTrue);
        expect(fakeActions.updateFieldsCalls, 1);
        expect(
          container.read(profilePublicationProvider(user.id)),
          publishedProfile,
        );
        expect(
          find.text('Perubahan belum bisa disimpan. Coba lagi.'),
          findsNothing,
        );
        expect(find.byKey(state.widget.key!), findsNothing);
      },
    );
  });

  testWidgets('stale principal switch suppresses publication and navigation', (
    tester,
  ) async {
    final user = _sellerUser(
      id: 'seller-stale',
      username: 'sellerStale',
      avatarUrl: 'https://example.com/avatar-old.jpg',
      storeName: 'Seller Farm',
      storeImageUrl: 'https://example.com/store-old.jpg',
    );
    final cachedProfile = _sellerProfile(
      userId: user.id,
      coverPhotoUrl: 'https://example.com/cover-old.jpg',
      storeImageUrl: 'https://example.com/store-old.jpg',
    );
    final switchedUser = _sellerUser(
      id: 'seller-switch',
      username: 'sellerSwitch',
      avatarUrl: 'https://example.com/avatar-switch.jpg',
      storeName: 'Other Store',
      storeImageUrl: 'https://example.com/store-switch.jpg',
    );

    await _runEditProfileSaveScenario(
      tester,
      user: user,
      cachedProfile: cachedProfile,
      publishedProfile: cachedProfile,
      configureState: (state) {
        state.setUsername('sellerStale-renamed');
        state.setBio('Updated stale bio');
        state.setStoreName('Seller Farm');
        state.setStoreImageUrl('https://example.com/store-old.jpg');
        state.setProfileCoverUrl('https://example.com/cover-old.jpg');
        state.setCoverPhotoPath('/tmp/cover-stale.jpg');
      },
      beforeSave: (state) {
        final authController =
            state.ref.read(authControllerProvider.notifier)
                as _RecordingAuthController;
        authController.forcedStateAfterUpdateProfile = AuthState.authenticated(
          switchedUser,
          emailVerified: true,
        );
        return state.triggerSave();
      },
      verify: (container, authController, fakeActions, savedResult, state) {
        expect(savedResult, isNull);
        expect(fakeActions.updateFieldsCalls, 0);
        expect(authController.updateProfileCalls, 1);
        expect(authController.state, isA<AuthStateAuthenticated>());
        final current = authController.state as AuthStateAuthenticated;
        expect(current.user.id, switchedUser.id);
        expect(container.read(profilePublicationProvider(user.id)), isNull);
        expect(find.text('Profile updated successfully'), findsNothing);
        expect(find.byKey(state.widget.key!), findsOneWidget);
      },
    );
  });

  testWidgets(
    'StableNetworkImage keeps the previous image visible while a new URL is loading',
    (tester) async {
      const firstUrl = 'https://cdn.example.com/first.jpg';
      const secondUrl = 'https://cdn.example.com/second.jpg';
      final first = _DelayedResponse.pending();
      final second = _DelayedResponse.pending();
      final responders = <String, _DelayedResponse>{
        firstUrl: first,
        secondUrl: second,
      };

      await HttpOverrides.runZoned(() async {
        await tester.pumpWidget(
          MaterialApp(
            home: StatefulBuilder(
              builder: (context, setState) {
                return Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    SizedBox(
                      width: 24,
                      height: 24,
                      child: StableNetworkImage(
                        imageUrl: _StableImageHarness.url,
                        fallback: SizedBox(width: 24, height: 24),
                      ),
                    ),
                    TextButton(
                      onPressed: () => setState(() {
                        _StableImageHarness.url = secondUrl;
                      }),
                      child: const Text('Swap'),
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

        await tester.tap(find.text('Swap'));
        await tester.pump();
        expect(find.byType(Stack), findsOneWidget);
        expect(find.byType(Image), findsNWidgets(2));

        second.complete(_onePxPngBytes);
        await tester.pumpAndSettle();
        expect(find.byType(Image), findsOneWidget);
      }, createHttpClient: (_) => _DelayedImageHttpClient(responders));
    },
  );
}

class _StableImageHarness {
  static String url = 'https://cdn.example.com/first.jpg';
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
