import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/core/src/providers/presence_provider.dart'
    as core_presence;
import 'package:labuda/domains/chat/chat/data/dto/chat_dto.dart';
import 'package:labuda/domains/chat/chat/data/remote/chat_api_datasource.dart';
import 'package:labuda/domains/chat/chat/data/repositories/chat_repository_impl.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:labuda/domains/chat/chat/domain/repositories/chat_repository.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_providers.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_state.dart'
    as chat_state;
import 'package:labuda/domains/chat/chat/presentation/screens/chat_detail_screen.dart';
import 'package:labuda/domains/commerce/negotiation/negotiation/presentation/providers/negotiation_notifier.dart';
import 'package:labuda/domains/commerce/negotiation/negotiation/presentation/providers/negotiation_providers.dart';
import 'package:labuda/domains/commerce/negotiation/negotiation/presentation/providers/negotiation_state.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/generated/app_localizations.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/providers/auth_status_providers.dart'
    show currentUserIdProvider;
import 'package:labuda/shared/providers/block_state_provider.dart';

const _chatId = '00000000-0000-0000-0000-00000000c123';
const _currentUserId = '00000000-0000-0000-0000-00000000a111';
const _peerUserId = '00000000-0000-0000-0000-00000000b222';

class _FakeAuthController extends AuthController {
  @override
  AuthState build() {
    final now = DateTime.utc(2026, 7, 30, 8);
    final user = AuthUser(
      id: _currentUserId,
      createdAt: now,
      updatedAt: now,
      email: 'me@example.com',
      username: 'me',
      isEmailVerified: true,
      accountStatus: AccountStatus.active,
      hasSellerProfile: false,
      hasMarketAuthority: false,
      sellerSubscriptionStatus: 'none',
      roles: const [UserRole.user],
      provider: ShonaAuthProvider.email,
      lifecycle: ContentLifecycle.active,
    );
    return AuthState.authenticated(user, emailVerified: true);
  }
}

class _FakePresenceManager extends core_presence.PresenceManager {
  @override
  core_presence.PresenceAuthorityState build() =>
      const core_presence.PresenceAuthorityState.empty();

  @override
  core_presence.PresenceSubscriptionHandle acquire(Set<String> userIds) {
    return core_presence.PresenceSubscriptionHandle(() async {});
  }

  @override
  Future<void> prepareForLogout() async {}

  @override
  core_presence.PresenceState? lookup(String userId) => null;

  @override
  Map<String, core_presence.PresenceState?> lookupMany(
    Iterable<String> userIds,
  ) {
    return {for (final id in userIds) id: null};
  }

  @override
  Future<void> publishSelfPresence({required bool isOnline}) async {}

  @override
  Future<void> setForeground(bool isForeground) async {}
}

class _FakeNegotiationNotifier extends NegotiationNotifier {
  @override
  NegotiationState build() => const NegotiationState();
}

class _SilentLogger implements ILoggerService {
  @override
  Future<Result<void>> debug(String message, {Map<String, dynamic>? extra}) =>
      Future.value(Result.success(null));
  @override
  Future<Result<void>> info(String message, {Map<String, dynamic>? extra}) =>
      Future.value(Result.success(null));
  @override
  Future<Result<void>> warning(String message, {Map<String, dynamic>? extra}) =>
      Future.value(Result.success(null));
  @override
  Future<Result<void>> error(
    String message, {
    Map<String, dynamic>? extra,
    StackTrace? stackTrace,
  }) => Future.value(Result.success(null));
  @override
  Future<Result<void>> fatal(
    String message, {
    Map<String, dynamic>? extra,
    StackTrace? stackTrace,
  }) => Future.value(Result.success(null));
  @override
  Future<Result<void>> logUserAction(
    String action, {
    String? userId,
    Map<String, dynamic>? parameters,
  }) => Future.value(Result.success(null));
  @override
  Future<Result<void>> logPerformance(
    String operation, {
    required Duration duration,
    Map<String, dynamic>? metrics,
  }) => Future.value(Result.success(null));
  @override
  Future<Result<void>> logSecurityEvent(
    String event, {
    String? userId,
    String? severity,
    Map<String, dynamic>? details,
  }) => Future.value(Result.success(null));
  @override
  Future<Result<void>> logApiCall(
    String endpoint, {
    required String method,
    required int statusCode,
    required Duration duration,
    Map<String, dynamic>? requestData,
    Map<String, dynamic>? responseData,
  }) => Future.value(Result.success(null));
  @override
  Future<Result<void>> setLogLevel(LogLevel level) =>
      Future.value(Result.success(null));
  @override
  Future<Result<void>> clearLogs() => Future.value(Result.success(null));
  @override
  Future<Result<List<LogEntry>>> getLogs({
    LogLevel? minLevel,
    DateTime? startDate,
    DateTime? endDate,
    int? limit,
  }) => Future.value(Result.success(const <LogEntry>[]));
  @override
  Future<void> debugSync(String userId) async {}
  @override
  Future<void> debugSyncSuccess(String userId) async {}
  @override
  Future<void> debugSyncFailed(String userId, String? errorMessage) async {}
  @override
  Future<void> debugCallingGetCurrentUser() async {}
  @override
  Future<void> debugGetCurrentUserSuccess(
    String userId,
    bool isEmailVerified,
  ) async {}
  @override
  Future<void> debugGetCurrentUserFailed(
    String userId,
    String? errorMessage,
  ) async {}
  @override
  Future<void> debugSyncException(
    String userId,
    String errorMessage,
    String stackTrace,
  ) async {}
  @override
  Future<void> debugRouterCheck(
    String userId,
    bool isEmailVerified,
    String location,
    bool isVerificationRoute,
  ) async {}
  @override
  Future<void> log(String message, {LogLevel level = LogLevel.debug}) async {}
}

class _FakeChatApiDatasource extends ChatApiDatasource {
  final Future<Result<ChatDto>> Function(String chatId) onGetChatById;

  _FakeChatApiDatasource(this.onGetChatById)
    : super(_NoopApiClient(), logger: _SilentLogger());

  @override
  Future<Result<ChatDto>> getChatById(String chatId) => onGetChatById(chatId);
}

class _NoopApiClient implements ApiClient {
  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _FakeChatRepository extends Fake implements ChatRepository {
  _FakeChatRepository({
    required this.getChatByIdResult,
    required this.getMessagesResult,
  });

  final Result<Chat> getChatByIdResult;
  final Result<List<Message>> getMessagesResult;

  @override
  Future<Result<Chat>> getChatById(String chatId) async => getChatByIdResult;

  @override
  Future<Result<List<Message>>> getMessages({
    required String chatId,
    required String userId,
    int page = 1,
    int limit = 50,
    DateTime? cursorCreatedAt,
    String? cursorId,
  }) async => getMessagesResult;
}

ProviderContainer _buildChatContainer({required ChatRepository repository}) {
  return ProviderContainer(
    overrides: [chatRepositoryProvider.overrideWithValue(repository)],
  );
}

ProviderScope _buildChatDetailScope({required ChatRepository repository}) {
  return ProviderScope(
    overrides: [
      apiClientProvider.overrideWithValue(_NoopApiClient()),
      authControllerProvider.overrideWith(_FakeAuthController.new),
      currentUserIdProvider.overrideWithValue(_currentUserId),
      typingIndicatorEnabledProvider.overrideWithValue(false),
      isUserBlockedProvider(_peerUserId).overrideWith((ref) => false),
      negotiationNotifierProvider.overrideWith(_FakeNegotiationNotifier.new),
      chatRepositoryProvider.overrideWithValue(repository),
      presenceProvider.overrideWithValue(const chat_state.PresenceState()),
    ],
    child: MaterialApp(
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      locale: const Locale('en', 'US'),
      home: const ChatDetailScreen(chatId: _chatId),
    ),
  );
}

void main() {
  group('ChatRepositoryImpl.getChatById', () {
    test(
      'maps canonical direct-room response and stops returning the stubbed error',
      () async {
        final dto = ChatDto.fromJson({
          'id': _chatId,
          'room_type': 'direct',
          'other_user_id': _peerUserId,
          'other_user': {
            'id': _peerUserId,
            'username': 'bob',
            'avatar_url': null,
          },
          'created_at': '2026-07-30T00:00:00Z',
          'updated_at': '2026-07-30T00:00:00Z',
          'unread_count': 0,
        });

        final repo = ChatRepositoryImpl(
          apiDatasource: _FakeChatApiDatasource(
            (_) async => Result.success(dto),
          ),
          webSocketService: WebSocketService(baseUrl: 'ws://example.invalid'),
          logger: _SilentLogger(),
          presenceManager: _FakePresenceManager(),
        );

        final result = await repo.getChatById(_chatId);

        expect(result.isSuccess, isTrue);
        expect(result.error, isNull);
        expect(result.data?.id, _chatId);
        expect(result.data?.participantNames[_peerUserId], 'bob');
      },
    );
  });

  group('ChatDetailNotifier.getChatById flow', () {
    test('success populates state.chat and clears loading', () async {
      final repository = _FakeChatRepository(
        getChatByIdResult: Result.success(
          Chat(
            id: _chatId,
            type: ChatType.private,
            participantIds: [_peerUserId],
            participantNames: const {_peerUserId: 'bob'},
            participantAvatars: const {},
            participantLifecycles: const {_peerUserId: ContentLifecycle.active},
            createdAt: DateTime.utc(2026, 7, 30),
            status: ChatStatus.active,
          ),
        ),
        getMessagesResult: Result.success(const <Message>[]),
      );

      final container = _buildChatContainer(repository: repository);
      addTearDown(container.dispose);

      final notifier = container.read(chatDetailProvider(_chatId).notifier);
      await notifier.loadChat(_currentUserId);

      final state = container.read(chatDetailProvider(_chatId));
      expect(state.isLoading, isFalse);
      expect(state.error, isNull);
      expect(state.chat, isNotNull);
      expect(
        state.chat?.participantIds,
        containsAll([_currentUserId, _peerUserId]),
      );
      expect(state.chat?.participantNames[_peerUserId], 'bob');
    });

    test('failure clears loading and stores the error', () async {
      final repository = _FakeChatRepository(
        getChatByIdResult: Result.error('Room not found'),
        getMessagesResult: Result.success(const <Message>[]),
      );

      final container = _buildChatContainer(repository: repository);
      addTearDown(container.dispose);

      final notifier = container.read(chatDetailProvider(_chatId).notifier);
      await notifier.loadChat(_currentUserId);

      final state = container.read(chatDetailProvider(_chatId));
      expect(state.isLoading, isFalse);
      expect(state.chat, isNull);
      expect(state.error, 'Room not found');
    });
  });

  group('ChatDetailScreen bootstrap', () {
    testWidgets(
      'constructs, waits until after the first frame, and loads chat',
      (tester) async {
        final repository = _FakeChatRepository(
          getChatByIdResult: Result.success(
            Chat(
              id: _chatId,
              type: ChatType.private,
              participantIds: [_peerUserId],
              participantNames: const {_peerUserId: 'bob'},
              participantAvatars: const {},
              participantLifecycles: const {
                _peerUserId: ContentLifecycle.active,
              },
              createdAt: DateTime.utc(2026, 7, 30),
              status: ChatStatus.active,
            ),
          ),
          getMessagesResult: Result.success(const <Message>[]),
        );

        await tester.pumpWidget(_buildChatDetailScope(repository: repository));

        expect(tester.takeException(), isNull);
        expect(find.text('Loading...'), findsOneWidget);

        await tester.pump();
        await tester.pump(const Duration(milliseconds: 300));

        expect(tester.takeException(), isNull);
        expect(find.text('Loading...'), findsNothing);
        expect(find.text('@bob'), findsOneWidget);
        expect(find.text('Failed to load messages'), findsNothing);
      },
    );
  });
}
