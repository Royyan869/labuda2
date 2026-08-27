import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/common/result.dart';
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

  /// Accept offer (seller only)
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

  /// Cancel negotiation (buyer only)
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

  /// Get latest negotiation for a chat room
  Future<Result<Negotiation?>> getNegotiation({
    required String chatRoomId,
  }) async {
    state = state.copyWith(isLoading: true, error: null);

    final result = await _repository.getNegotiation(chatRoomId: chatRoomId);

    if (result.isSuccess) {
      state = state.copyWith(isLoading: false, currentNegotiation: result.data);
    } else {
      state = state.copyWith(isLoading: false, error: result.error);
    }

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
