import 'package:labuda/core/common/result.dart';
import '../domain/entities/negotiation.dart';
import '../domain/repositories/negotiation_repository.dart';
import 'dto/negotiation_dto.dart';
import 'mappers/negotiation_mapper.dart';
import 'remote/negotiation_remote_datasource.dart';

/// Implementation of NegotiationRepository
///
/// **Data Layer** - implements domain interface
/// All operations delegate to chat-owned API endpoints.
class NegotiationRepositoryImpl implements NegotiationRepository {
  final NegotiationRemoteDatasource _remote;

  NegotiationRepositoryImpl({required NegotiationRemoteDatasource remote})
    : _remote = remote;

  @override
  Future<Result<Negotiation>> createNegotiation({
    required String chatRoomId,
    required String fixedPriceSaleId,
    required int price,
    String? note,
  }) async {
    try {
      final request = CreateNegotiationDto(
        forSaleId: fixedPriceSaleId,
        price: price,
        note: note,
      );
      final dto = await _remote.createNegotiation(
        chatRoomId: chatRoomId,
        request: request,
      );
      return Result.success(
        NegotiationMapper.toEntity(dto, chatRoomId: chatRoomId),
      );
    } catch (e) {
      return Result.error('Failed to create negotiation: $e');
    }
  }

  @override
  Future<Result<Negotiation>> counterOffer({
    required String chatRoomId,
    required String sessionId,
    required int price,
    String? note,
  }) async {
    try {
      final dto = await _remote.counterNegotiation(
        chatRoomId: chatRoomId,
        sessionId: sessionId,
        price: price,
        note: note,
      );
      return Result.success(
        NegotiationMapper.toEntity(dto, chatRoomId: chatRoomId),
      );
    } catch (e) {
      return Result.error('Failed to counter offer: $e');
    }
  }

  @override
  Future<Result<Negotiation>> acceptOffer({
    required String chatRoomId,
    required String sessionId,
  }) async {
    try {
      final dto = await _remote.acceptNegotiation(
        chatRoomId: chatRoomId,
        sessionId: sessionId,
      );
      return Result.success(
        NegotiationMapper.toEntity(dto, chatRoomId: chatRoomId),
      );
    } catch (e) {
      return Result.error('Failed to accept offer: $e');
    }
  }

  @override
  Future<Result<Negotiation>> cancelNegotiation({
    required String chatRoomId,
    required String sessionId,
  }) async {
    try {
      final dto = await _remote.cancelNegotiation(
        chatRoomId: chatRoomId,
        sessionId: sessionId,
      );
      return Result.success(
        NegotiationMapper.toEntity(dto, chatRoomId: chatRoomId),
      );
    } catch (e) {
      return Result.error('Failed to cancel negotiation: $e');
    }
  }

  @override
  Future<Result<Negotiation?>> getNegotiation({
    required String chatRoomId,
  }) async {
    try {
      final dto = await _remote.getNegotiation(chatRoomId: chatRoomId);
      if (dto == null) {
        return Result.success(null);
      }
      return Result.success(
        NegotiationMapper.toEntity(dto, chatRoomId: chatRoomId),
      );
    } catch (e) {
      return Result.error('Failed to get negotiation: $e');
    }
  }
}
