import 'package:labuda/core/common/result.dart';
import '../entities/negotiation.dart';

/// Negotiation Repository Interface
///
/// **Domain Layer** - interface bebas dari implementation details
///
/// **CHAT-OWNED CONTRACT:**
/// All negotiation operations require a chatRoomId because negotiation
/// is scoped under chat rooms (not a standalone resource).
/// Backend handles expiration automatically via NegotiationExpireWorker.
abstract class NegotiationRepository {
  /// Start new negotiation in a chat room (buyer initiates)
  Future<Result<Negotiation>> createNegotiation({
    required String chatRoomId,
    required String fixedPriceSaleId,
    required int price,
    String? note,
  });

  /// Counter offer (either party)
  Future<Result<Negotiation>> counterOffer({
    required String chatRoomId,
    required String sessionId,
    required int price,
    String? note,
  });

  /// Accept current offer (seller only)
  Future<Result<Negotiation>> acceptOffer({
    required String chatRoomId,
    required String sessionId,
  });

  /// Cancel negotiation (buyer only)
  Future<Result<Negotiation>> cancelNegotiation({
    required String chatRoomId,
    required String sessionId,
  });

  /// Get latest negotiation session for a chat room
  Future<Result<Negotiation?>> getNegotiation({required String chatRoomId});
}
