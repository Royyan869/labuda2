import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/core/src/interfaces/services/i_logger_service.dart';
import 'package:labuda/core/src/providers/presence_provider.dart'
    as core_presence;
import 'package:labuda/core/websocket/websocket_service.dart';
import 'package:labuda/domains/chat/chat/data/dto/attachment_dto.dart';
import 'package:labuda/domains/chat/chat/data/dto/chat_resource_occurrence_request.dart';
import 'package:labuda/domains/chat/chat/data/dto/message_dto.dart';
import 'package:labuda/domains/chat/chat/data/remote/chat_api_datasource.dart';
import 'package:labuda/domains/chat/chat/data/repositories/chat_repository_impl.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';

const _roomId = '11111111-1111-1111-1111-111111111111';
const _senderId = '22222222-2222-2222-2222-222222222222';
const _resourceIdA = '33333333-3333-3333-3333-333333333333';
const _resourceIdB = '44444444-4444-4444-4444-444444444444';
const _resourceIdC = '55555555-5555-5555-5555-555555555555';
const _idempotencyKey = '66666666-6666-6666-6666-666666666666';
const _secondIdempotencyKey = '77777777-7777-7777-7777-777777777777';

class _NoopApiClient implements ApiClient {
  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _SilentLogger implements ILoggerService {
  @override
  Future<Result<void>> debug(String message, {Map<String, dynamic>? extra}) =>
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
  Future<Result<void>> info(String message, {Map<String, dynamic>? extra}) =>
      Future.value(Result.success(null));

  @override
  Future<Result<void>> warning(String message, {Map<String, dynamic>? extra}) =>
      Future.value(Result.success(null));

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

class _FakePresenceManager extends core_presence.PresenceManager {
  @override
  core_presence.PresenceAuthorityState build() =>
      const core_presence.PresenceAuthorityState.empty();
}

class _TransportHarness {
  final _CapturingChatApiDatasource datasource;
  final ChatRepositoryImpl repo;

  _TransportHarness(this.datasource, this.repo);
}

class _CapturingChatApiDatasource extends ChatApiDatasource {
  final Future<Result<MessageDto>> Function(
    String chatRoomId,
    SendMessageDto request,
  )
  handler;
  final List<SendMessageDto> capturedRequests = [];

  _CapturingChatApiDatasource(this.handler) : super(_NoopApiClient());

  @override
  Future<Result<MessageDto>> sendMessage(
    String chatRoomId,
    SendMessageDto request,
  ) async {
    capturedRequests.add(request);
    return handler(chatRoomId, request);
  }
}

ChatResourceProjection _liveProfileProjection() {
  return ChatResourceProjection.fromJson({
    'state': 'LIVE',
    'resource_type': 'profile',
    'resource_id': _resourceIdA,
    'canonical_url': '/user/$_resourceIdA',
    'viewer_capabilities': {
      'can_view': true,
      'can_interact': false,
      'blocked_by_tombstone': false,
    },
    'profile': {
      'username': 'alice',
      'avatar_url': null,
      'store_name': 'Toko Alice',
      'is_seller': true,
      'lifecycle': 'active',
    },
  });
}

MessageDto _messageDto({ChatResourceProjection? resourceProjection}) {
  return MessageDto(
    id: 'msg-1',
    chatRoomId: _roomId,
    senderId: _senderId,
    senderName: 'Sender',
    senderUsername: 'sender',
    content: 'hello',
    type: 'text',
    status: 'sent',
    isRead: false,
    isEdited: false,
    createdAt: DateTime.parse('2026-08-09T00:00:00.000Z'),
    updatedAt: DateTime.parse('2026-08-09T00:00:00.000Z'),
    resourceProjection: resourceProjection,
  );
}

_TransportHarness _buildHarness({
  Future<Result<MessageDto>> Function(
    String chatRoomId,
    SendMessageDto request,
  )?
  handler,
}) {
  final datasource = _CapturingChatApiDatasource(
    handler ?? (chatRoomId, request) async => Result.success(_messageDto()),
  );
  final repo = ChatRepositoryImpl(
    apiDatasource: datasource,
    webSocketService: WebSocketService(baseUrl: 'ws://example.invalid'),
    logger: _SilentLogger(),
    presenceManager: _FakePresenceManager(),
  );
  return _TransportHarness(datasource, repo);
}

void main() {
  group('Unified share chat mobile canonical write transport', () {
    test('W1 normal text send -> no resource_occurrence', () async {
      final harness = _buildHarness();
      addTearDown(harness.repo.dispose);

      await harness.repo.sendMessage(
        chatId: _roomId,
        senderId: _senderId,
        senderName: 'Sender',
        content: 'hello',
        type: MessageType.text,
        idempotencyKey: _idempotencyKey,
      );

      final payload = harness.datasource.capturedRequests.single.toJson();
      expect(payload['message_type'], 'text');
      expect(payload['body'], 'hello');
      expect(payload.containsKey('resource_occurrence'), isFalse);
      expect(payload.containsKey('attachment_json'), isFalse);
    });

    test('W2 normal media send -> no resource_occurrence', () async {
      final harness = _buildHarness();
      addTearDown(harness.repo.dispose);

      await harness.repo.sendMessage(
        chatId: _roomId,
        senderId: _senderId,
        senderName: 'Sender',
        content: 'media',
        type: MessageType.image,
        mediaAssetIds: const ['asset-1'],
        idempotencyKey: _idempotencyKey,
      );

      final payload = harness.datasource.capturedRequests.single.toJson();
      expect(payload['message_type'], 'text');
      expect(payload['media_asset_ids'], ['asset-1']);
      expect(payload.containsKey('resource_occurrence'), isFalse);
    });

    test('W3 location/negotiation/shipping send unchanged', () {
      final cases = <AttachmentDto>[
        LocationAttachmentDto(
          latitude: -6.2,
          longitude: 106.8,
          placeName: 'Jakarta',
          address: 'Jakarta',
        ),
        NegotiationProposalAttachmentDto(
          sessionId: 'session-1',
          proposalSequence: 1,
          price: 150000,
          resourceType: 'for_sale',
          resourceId: _resourceIdB,
          note: 'counter',
        ),
        ShippingQuoteAttachmentDto(
          offerId: 'offer-1',
          linkedItemId: _resourceIdB,
          linkedItemType: 'listing',
          shippingType: 'manual',
          shippingTypeName: 'Manual',
          shippingTypeEmoji: '🚚',
          rate: 25000.0,
          status: 'ACTIVE',
          sellerId: _senderId,
        ),
      ];

      for (final attachment in cases) {
        final payload = SendMessageDto(
          body: 'hello',
          messageType: 'text',
          idempotencyKey: _idempotencyKey,
          attachment: attachment,
        ).toJson();
        expect(payload.containsKey('attachment_json'), isTrue);
        expect(payload.containsKey('resource_occurrence'), isFalse);
      }
    });

    test('W4 profile share request serializes share_to_chat', () {
      final request = ChatResourceOccurrenceRequest.shareToChat(
        resourceType: ChatResourceOccurrenceResourceType.profile,
        resourceId: _resourceIdA,
      );
      expect(request.toJson(), {
        'operation': 'share_to_chat',
        'resource_type': 'profile',
        'resource_id': _resourceIdA,
      });
    });

    test('W5 content share request serializes share_to_chat', () {
      final request = ChatResourceOccurrenceRequest.shareToChat(
        resourceType: ChatResourceOccurrenceResourceType.content,
        resourceId: _resourceIdB,
      );
      expect(request.toJson(), {
        'operation': 'share_to_chat',
        'resource_type': 'content',
        'resource_id': _resourceIdB,
      });
    });

    test('W6 FPS share request serializes share_to_chat', () {
      final request = ChatResourceOccurrenceRequest.shareToChat(
        resourceType: ChatResourceOccurrenceResourceType.forSale,
        resourceId: _resourceIdC,
      );
      expect(request.toJson(), {
        'operation': 'share_to_chat',
        'resource_type': 'for_sale',
        'resource_id': _resourceIdC,
      });
    });

    test('W7 Auction share request serializes share_to_chat', () {
      final request = ChatResourceOccurrenceRequest.shareToChat(
        resourceType: ChatResourceOccurrenceResourceType.auction,
        resourceId: _resourceIdA,
      );
      expect(request.toJson(), {
        'operation': 'share_to_chat',
        'resource_type': 'auction',
        'resource_id': _resourceIdA,
      });
    });

    test('W8 FPS direct insert serializes direct_commerce_insert_chat', () {
      final request = ChatResourceOccurrenceRequest.directCommerceInsertChat(
        resourceType: ChatResourceOccurrenceResourceType.forSale,
        resourceId: _resourceIdB,
      );
      expect(request.toJson(), {
        'operation': 'direct_commerce_insert_chat',
        'resource_type': 'for_sale',
        'resource_id': _resourceIdB,
      });
    });

    test('W9 Auction direct insert serializes direct_commerce_insert_chat', () {
      final request = ChatResourceOccurrenceRequest.directCommerceInsertChat(
        resourceType: ChatResourceOccurrenceResourceType.auction,
        resourceId: _resourceIdC,
      );
      expect(request.toJson(), {
        'operation': 'direct_commerce_insert_chat',
        'resource_type': 'auction',
        'resource_id': _resourceIdC,
      });
    });

    test('W10 direct Profile is rejected structurally', () {
      expect(
        () => ChatResourceOccurrenceRequest.directCommerceInsertChat(
          resourceType: ChatResourceOccurrenceResourceType.profile,
          resourceId: _resourceIdA,
        ),
        throwsFormatException,
      );
    });

    test('W11 direct Content is rejected structurally', () {
      expect(
        () => ChatResourceOccurrenceRequest.directCommerceInsertChat(
          resourceType: ChatResourceOccurrenceResourceType.content,
          resourceId: _resourceIdB,
        ),
        throwsFormatException,
      );
    });

    test('W12 resource-only send allowed', () async {
      final harness = _buildHarness();
      addTearDown(harness.repo.dispose);
      final occurrence = ChatResourceOccurrenceRequest.shareToChat(
        resourceType: ChatResourceOccurrenceResourceType.forSale,
        resourceId: _resourceIdC,
      );

      await harness.repo.sendMessage(
        chatId: _roomId,
        senderId: _senderId,
        senderName: 'Sender',
        content: '',
        type: MessageType.system,
        idempotencyKey: _idempotencyKey,
        resourceOccurrence: occurrence,
      );

      final payload = harness.datasource.capturedRequests.single.toJson();
      expect(payload['message_type'], 'system');
      expect(payload['body'], '');
      expect(payload['resource_occurrence'], occurrence.toJson());
    });

    test('W13 resource + optional text allowed', () async {
      final harness = _buildHarness();
      addTearDown(harness.repo.dispose);
      final occurrence = ChatResourceOccurrenceRequest.shareToChat(
        resourceType: ChatResourceOccurrenceResourceType.auction,
        resourceId: _resourceIdA,
      );

      await harness.repo.sendMessage(
        chatId: _roomId,
        senderId: _senderId,
        senderName: 'Sender',
        content: 'caption text',
        type: MessageType.text,
        idempotencyKey: _idempotencyKey,
        resourceOccurrence: occurrence,
      );

      final payload = harness.datasource.capturedRequests.single.toJson();
      expect(payload['message_type'], 'text');
      expect(payload['body'], 'caption text');
      expect(payload['resource_occurrence'], occurrence.toJson());
    });

    test('W14 request contains identity only', () {
      final request = ChatResourceOccurrenceRequest.shareToChat(
        resourceType: ChatResourceOccurrenceResourceType.profile,
        resourceId: _resourceIdA,
      );
      expect(request.toJson().keys.toSet(), {
        'operation',
        'resource_type',
        'resource_id',
      });
    });

    test('W15 no preview/title/image/price/seller fields', () {
      final request = ChatResourceOccurrenceRequest.directCommerceInsertChat(
        resourceType: ChatResourceOccurrenceResourceType.forSale,
        resourceId: _resourceIdB,
      );
      final json = request.toJson();
      for (final forbidden in <String>[
        'preview',
        'title',
        'image',
        'price',
        'seller',
        'username',
      ]) {
        expect(json.containsKey(forbidden), isFalse);
      }
    });

    test('W16 canonical resource send omits attachment_json', () {
      final occurrence = ChatResourceOccurrenceRequest.shareToChat(
        resourceType: ChatResourceOccurrenceResourceType.forSale,
        resourceId: _resourceIdC,
      );
      final payload = SendMessageDto(
        body: '',
        messageType: 'system',
        idempotencyKey: _idempotencyKey,
        resourceOccurrence: occurrence,
      ).toJson();
      expect(payload.containsKey('attachment_json'), isFalse);
      expect(payload['resource_occurrence'], occurrence.toJson());
    });

    test('W17 room context not mutated', () {
      final occurrence = ChatResourceOccurrenceRequest.shareToChat(
        resourceType: ChatResourceOccurrenceResourceType.auction,
        resourceId: _resourceIdA,
      );
      final payload = SendMessageDto(
        body: 'caption',
        messageType: 'text',
        idempotencyKey: _idempotencyKey,
        resourceOccurrence: occurrence,
      ).toJson();
      expect(payload.containsKey('room_context'), isFalse);
      expect(payload.containsKey('room_commerce_context'), isFalse);
      expect(payload.containsKey('source_type'), isFalse);
    });

    test('W18 same FPS can produce distinct share/direct operations', () {
      final share = ChatResourceOccurrenceRequest.shareToChat(
        resourceType: ChatResourceOccurrenceResourceType.forSale,
        resourceId: _resourceIdB,
      );
      final direct = ChatResourceOccurrenceRequest.directCommerceInsertChat(
        resourceType: ChatResourceOccurrenceResourceType.forSale,
        resourceId: _resourceIdB,
      );
      expect(share, isNot(equals(direct)));
      expect(share.toJson()['operation'], 'share_to_chat');
      expect(direct.toJson()['operation'], 'direct_commerce_insert_chat');
    });

    test('W19 same Auction can produce distinct share/direct operations', () {
      final share = ChatResourceOccurrenceRequest.shareToChat(
        resourceType: ChatResourceOccurrenceResourceType.auction,
        resourceId: _resourceIdC,
      );
      final direct = ChatResourceOccurrenceRequest.directCommerceInsertChat(
        resourceType: ChatResourceOccurrenceResourceType.auction,
        resourceId: _resourceIdC,
      );
      expect(share, isNot(equals(direct)));
      expect(share.toJson()['operation'], 'share_to_chat');
      expect(direct.toJson()['operation'], 'direct_commerce_insert_chat');
    });

    test(
      'W20 server SendMessage response parses canonical resource_projection',
      () async {
        final projection = _liveProfileProjection();
        final harness = _buildHarness(
          handler: (chatRoomId, request) async =>
              Result.success(_messageDto(resourceProjection: projection)),
        );
        addTearDown(harness.repo.dispose);

        final result = await harness.repo.sendMessage(
          chatId: _roomId,
          senderId: _senderId,
          senderName: 'Sender',
          content: '',
          type: MessageType.system,
          idempotencyKey: _idempotencyKey,
          resourceOccurrence: ChatResourceOccurrenceRequest.shareToChat(
            resourceType: ChatResourceOccurrenceResourceType.profile,
            resourceId: _resourceIdA,
          ),
        );

        expect(result.isSuccess, isTrue);
        expect(result.data?.resourceProjection, isNotNull);
        expect(
          result.data?.resourceProjection?.resourceType.wireValue,
          'profile',
        );
      },
    );

    test(
      'W21 server response replaces optimistic/local resource representation',
      () async {
        final projection = _liveProfileProjection();
        final harness = _buildHarness(
          handler: (chatRoomId, request) async =>
              Result.success(_messageDto(resourceProjection: projection)),
        );
        addTearDown(harness.repo.dispose);

        final result = await harness.repo.sendMessage(
          chatId: _roomId,
          senderId: _senderId,
          senderName: 'Sender',
          content: 'caption',
          type: MessageType.text,
          idempotencyKey: _idempotencyKey,
          resourceOccurrence: ChatResourceOccurrenceRequest.shareToChat(
            resourceType: ChatResourceOccurrenceResourceType.profile,
            resourceId: _resourceIdA,
          ),
        );

        expect(result.data?.resourceProjection, isNotNull);
      },
    );

    test(
      'W22 retry/idempotency preserves canonical command semantics',
      () async {
        final harness = _buildHarness();
        addTearDown(harness.repo.dispose);
        final occurrence = ChatResourceOccurrenceRequest.shareToChat(
          resourceType: ChatResourceOccurrenceResourceType.forSale,
          resourceId: _resourceIdC,
        );

        await harness.repo.sendMessage(
          chatId: _roomId,
          senderId: _senderId,
          senderName: 'Sender',
          content: 'caption',
          type: MessageType.text,
          idempotencyKey: _idempotencyKey,
          resourceOccurrence: occurrence,
        );
        await harness.repo.sendMessage(
          chatId: _roomId,
          senderId: _senderId,
          senderName: 'Sender',
          content: 'caption',
          type: MessageType.text,
          idempotencyKey: _idempotencyKey,
          resourceOccurrence: occurrence,
        );

        expect(harness.datasource.capturedRequests, hasLength(2));
        expect(
          harness.datasource.capturedRequests[0].toJson()['idempotency_key'],
          _idempotencyKey,
        );
        expect(
          harness.datasource.capturedRequests[1].toJson()['idempotency_key'],
          _idempotencyKey,
        );
        expect(
          harness.datasource.capturedRequests[0]
              .toJson()['resource_occurrence'],
          occurrence.toJson(),
        );
        expect(
          harness.datasource.capturedRequests[1]
              .toJson()['resource_occurrence'],
          occurrence.toJson(),
        );
      },
    );

    test(
      'W23 backend rejection propagates as send failure without legacy fallback',
      () async {
        final harness = _buildHarness(
          handler: (chatRoomId, request) async =>
              Result.error('backend rejected canonical occurrence'),
        );
        addTearDown(harness.repo.dispose);
        final occurrence =
            ChatResourceOccurrenceRequest.directCommerceInsertChat(
              resourceType: ChatResourceOccurrenceResourceType.auction,
              resourceId: _resourceIdA,
            );

        final result = await harness.repo.sendMessage(
          chatId: _roomId,
          senderId: _senderId,
          senderName: 'Sender',
          content: '',
          type: MessageType.system,
          idempotencyKey: _secondIdempotencyKey,
          resourceOccurrence: occurrence,
        );

        expect(result.isFailure, isTrue);
        expect(result.error, 'backend rejected canonical occurrence');
        expect(
          harness.datasource.capturedRequests.single
              .toJson()['resource_occurrence'],
          occurrence.toJson(),
        );
        expect(
          harness.datasource.capturedRequests.single.toJson().containsKey(
            'attachment_json',
          ),
          isFalse,
        );
      },
    );

    test('W24 unrelated legacy/non-resource attachments remain unchanged', () {
      final attachment = ShippingQuoteAttachmentDto(
        offerId: 'offer-9',
        linkedItemId: _resourceIdB,
        linkedItemType: 'listing',
        shippingType: 'manual',
        shippingTypeName: 'Manual',
        shippingTypeEmoji: '🚚',
        rate: 25000.0,
        status: 'ACTIVE',
        sellerId: _senderId,
      );
      final payload = SendMessageDto(
        body: 'hello',
        messageType: 'text',
        idempotencyKey: _idempotencyKey,
        attachment: attachment,
      ).toJson();

      expect(payload['attachment_json'], isNotNull);
      expect(payload.containsKey('resource_occurrence'), isFalse);
    });

    test('rejects unknown operation at parse boundary', () {
      expect(
        () => ChatResourceOccurrenceOperation.fromWire('unknown'),
        throwsFormatException,
      );
    });

    test('rejects unknown resource type at parse boundary', () {
      expect(
        () => ChatResourceOccurrenceResourceType.fromWire('unknown'),
        throwsFormatException,
      );
    });

    test('rejects nil resource ID structurally', () {
      expect(
        () => ChatResourceOccurrenceRequest.shareToChat(
          resourceType: ChatResourceOccurrenceResourceType.profile,
          resourceId: '00000000-0000-0000-0000-000000000000',
        ),
        throwsFormatException,
      );
    });

    test('rejects empty resource ID structurally', () {
      expect(
        () => ChatResourceOccurrenceRequest.shareToChat(
          resourceType: ChatResourceOccurrenceResourceType.profile,
          resourceId: '   ',
        ),
        throwsFormatException,
      );
    });

    test('rejects multiple resource occurrences by construction', () {
      final request = ChatResourceOccurrenceRequest.shareToChat(
        resourceType: ChatResourceOccurrenceResourceType.profile,
        resourceId: _resourceIdA,
      );
      final payload = SendMessageDto(
        body: 'caption',
        messageType: 'text',
        idempotencyKey: _idempotencyKey,
        resourceOccurrence: request,
      ).toJson();

      expect(payload.containsKey('resource_occurrence'), isTrue);
      expect(payload.containsKey('resource_occurrences'), isFalse);
    });

    test('rejects canonical occurrence plus legacy attachment dual-write', () {
      final request = ChatResourceOccurrenceRequest.shareToChat(
        resourceType: ChatResourceOccurrenceResourceType.forSale,
        resourceId: _resourceIdB,
      );
      final attachment = LocationAttachmentDto(
        latitude: -6.2,
        longitude: 106.8,
        placeName: 'Jakarta',
        address: 'Jakarta',
      );

      expect(
        () => SendMessageDto(
          body: 'caption',
          messageType: 'text',
          idempotencyKey: _idempotencyKey,
          attachment: attachment,
          resourceOccurrence: request,
        ).toJson(),
        throwsStateError,
      );
    });
  });
}
