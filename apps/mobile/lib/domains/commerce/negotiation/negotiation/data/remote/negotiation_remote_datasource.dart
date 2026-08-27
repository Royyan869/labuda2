import 'package:labuda/core/api/api_client.dart';
import '../dto/negotiation_dto.dart';

/// Remote Datasource for Negotiation API calls
///
/// **Data Layer** - API calls isolated here
///
/// **BACKEND API CONTRACT (Chat-Owned):**
/// All negotiation endpoints are scoped under chat rooms:
/// - POST /api/v1/chat/rooms/:room_id/negotiate  → Start negotiation
/// - POST /api/v1/chat/rooms/:room_id/counter    → Counter offer
/// - POST /api/v1/chat/rooms/:room_id/respond    → Accept or cancel
/// - GET  /api/v1/chat/rooms/:room_id/negotiation → Get latest session
///
/// **NON-CANONICAL (must NOT exist):**
/// - /negotiations/* standalone routes do NOT exist
class NegotiationRemoteDatasource {
  final ApiClient _apiClient;

  NegotiationRemoteDatasource({required ApiClient apiClient})
    : _apiClient = apiClient;

  /// Start negotiation in a chat room
  Future<NegotiationResponseDto> createNegotiation({
    required String chatRoomId,
    required CreateNegotiationDto request,
  }) async {
    final response = await _apiClient.post(
      '/chat/rooms/$chatRoomId/negotiate',
      data: request.toJson(),
    );
    return NegotiationResponseDto.fromJson(response.data['data']);
  }

  /// Send counter offer
  Future<NegotiationResponseDto> counterNegotiation({
    required String chatRoomId,
    required String sessionId,
    required int price,
    String? note,
  }) async {
    final response = await _apiClient.post(
      '/chat/rooms/$chatRoomId/counter',
      data: CounterOfferDto(
        sessionId: sessionId,
        price: price,
        note: note,
      ).toJson(),
    );
    return NegotiationResponseDto.fromJson(response.data['data']);
  }

  /// Accept negotiation
  Future<NegotiationResponseDto> acceptNegotiation({
    required String chatRoomId,
    required String sessionId,
  }) async {
    final response = await _apiClient.post(
      '/chat/rooms/$chatRoomId/respond',
      data: RespondNegotiationDto(
        sessionId: sessionId,
        action: 'accept',
      ).toJson(),
    );
    return NegotiationResponseDto.fromJson(response.data['data']);
  }

  /// Cancel negotiation
  Future<NegotiationResponseDto> cancelNegotiation({
    required String chatRoomId,
    required String sessionId,
  }) async {
    final response = await _apiClient.post(
      '/chat/rooms/$chatRoomId/respond',
      data: RespondNegotiationDto(
        sessionId: sessionId,
        action: 'cancel',
      ).toJson(),
    );
    return NegotiationResponseDto.fromJson(response.data['data']);
  }

  /// Get latest negotiation session for a chat room
  ///
  /// Returns null if no negotiation exists (backend returns 404).
  Future<NegotiationResponseDto?> getNegotiation({
    required String chatRoomId,
  }) async {
    try {
      final response = await _apiClient.get(
        '/chat/rooms/$chatRoomId/negotiation',
      );
      return NegotiationResponseDto.fromJson(response.data['data']);
    } catch (_) {
      // 404 = no negotiation session exists for this room
      return null;
    }
  }
}
