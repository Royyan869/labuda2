import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:hishumi/core/common/result.dart';
import '../../domain/entities/negotiation.dart';
import '../../domain/repositories/negotiation_repository.dart';
import 'negotiation_state.dart';
import 'negotiation_providers.dart' show negotiationRepositoryProvider;

/// Notifier untuk Negotiation operations
///
/// **Presentation Layer** - UseCase logic moved here
///
/// **CHAT-OWNED CONTRACT:**
/// All operations require chatRoomId because negotiation is scoped
/// under chat rooms (not a standalone resource).
class NegotiationNotifier extends Notifier<NegotiationState> {
  late final NegotiationRepository _repository;

  // Synchronous double-submit guards for financial operations
  bool _isCreatingNegotiation = false;
  bool _isCounteringOffer = false;
  bool _isAcceptingOffer = false;

  @override
  NegotiationState build() {
    _repository = ref.read(negotiationRepositoryProvider);
    return const NegotiationState();
  }

  /// DEAL PRICE BINDING (commerce authority): the accepted, settleable
  /// session held in state for [forSaleId]. Every Beli path — for-sale
  /// detail, chat projection card, proposal card, shipping quote — forwards
  /// HERE, so checkout binds the DEAL price, never the list price (owner
  /// truth: the deal price is valid 24h from accept). Commerce decides the
  /// rule; other domains only carry the returned id.
  String? acceptedNegotiationIdFor(String? forSaleId) {
    if (forSaleId == null || forSaleId.isEmpty) return null;
    final cur = state.currentNegotiation;
    if (cur != null &&
        cur.status == NegotiationStatus.accepted &&
        cur.fixedPriceSaleId == forSaleId) {
      return cur.id;
    }
    return null;
  }

  /// Start new negotiation in a chat room
  Future<Result<Negotiation>> createNegotiation({
    required String chatRoomId,
    required String fixedPriceSaleId,
    required int price,
    String? note,
  }) async {
    if (_isCreatingNegotiation) {
      return Result.error('Already creating a negotiation');
    }
    _isCreatingNegotiation = true;

    try {
      state = state.copyWith(isLoading: true, error: null);

      final result = await _repository.createNegotiation(
        chatRoomId: chatRoomId,
        fixedPriceSaleId: fixedPriceSaleId,
        price: price,
        note: note,
      );

      if (result.isSuccess && result.data != null) {
        state = state.copyWith(
          isLoading: false,
          currentNegotiation: result.data,
        );
      } else {
        state = state.copyWith(isLoading: false, error: result.error);
      }

      return result;
    } finally {
      _isCreatingNegotiation = false;
    }
  }

  /// Counter offer
  Future<Result<Negotiation>> counterOffer({
    required String chatRoomId,
    required String sessionId,
    required int price,
    String? note,
  }) async {
    if (_isCounteringOffer) {
      return Result.error('Already countering an offer');
    }
    _isCounteringOffer = true;

    try {
      state = state.copyWith(isLoading: true, error: null);

      final result = await _repository.counterOffer(
        chatRoomId: chatRoomId,
        sessionId: sessionId,
        price: price,
        note: note,
      );

      if (result.isSuccess && result.data != null) {
        state = state.copyWith(
          isLoading: false,
          currentNegotiation: result.data,
        );
      } else {
        state = state.copyWith(isLoading: false, error: result.error);
      }

      return result;
    } finally {
      _isCounteringOffer = false;
    }
  }

  /// Accept the current price (either participant — owner truth: Terima
  /// exists on both sides; backend authorizes participation).
  Future<Result<Negotiation>> acceptOffer({
    required String chatRoomId,
    required String sessionId,
  }) async {
    if (_isAcceptingOffer) {
      return Result.error('Already accepting an offer');
    }
    _isAcceptingOffer = true;

    try {
      state = state.copyWith(isLoading: true, error: null);

      final result = await _repository.acceptOffer(
        chatRoomId: chatRoomId,
        sessionId: sessionId,
      );

      if (result.isSuccess && result.data != null) {
        state = state.copyWith(
          isLoading: false,
          currentNegotiation: result.data,
        );
      } else {
        state = state.copyWith(isLoading: false, error: result.error);
      }

      return result;
    } finally {
      _isAcceptingOffer = false;
    }
  }

  /// Reject (Tolak) an active negotiation — either participant.
  Future<Result<Negotiation>> cancelNegotiation({
    required String chatRoomId,
    required String sessionId,
  }) async {
    state = state.copyWith(isLoading: true, error: null);

    final result = await _repository.cancelNegotiation(
      chatRoomId: chatRoomId,
      sessionId: sessionId,
    );

    if (result.isSuccess && result.data != null) {
      state = state.copyWith(isLoading: false, currentNegotiation: result.data);
    } else {
      state = state.copyWith(isLoading: false, error: result.error);
    }

    return result;
  }

  /// Load the negotiation session of ONE chat room.
  ///
  /// EXACT-SET CONTRACT: success with null (room has no session) and transport
  /// failure both REPLACE currentNegotiation. copyWith keeps the old value on
  /// null, which leaked a previous room's session into the next room's banner.
  Future<Result<Negotiation?>> getNegotiation({
    required String chatRoomId,
  }) async {
    state = state.copyWith(isLoading: true, error: null);

    final result = await _repository.getNegotiation(chatRoomId: chatRoomId);

    state = NegotiationState(
      isLoading: false,
      error: result.isSuccess ? null : result.error,
      currentNegotiation: result.isSuccess ? result.data : null,
      negotiations: state.negotiations,
    );

    return result;
  }

  /// Clear error
  void clearError() {
    state = state.copyWith(error: null);
  }

  /// Reset state
  void reset() {
    state = const NegotiationState();
  }
}
