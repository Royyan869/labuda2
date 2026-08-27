import '../../domain/entities/negotiation.dart';

/// State untuk Negotiation operations
///
/// **Presentation Layer** - state management
class NegotiationState {
  final bool isLoading;
  final String? error;
  final Negotiation? currentNegotiation;
  final List<Negotiation> negotiations;

  const NegotiationState({
    this.isLoading = false,
    this.error,
    this.currentNegotiation,
    this.negotiations = const [],
  });

  NegotiationState copyWith({
    bool? isLoading,
    String? error,
    Negotiation? currentNegotiation,
    List<Negotiation>? negotiations,
  }) {
    return NegotiationState(
      isLoading: isLoading ?? this.isLoading,
      error: error ?? this.error,
      currentNegotiation: currentNegotiation ?? this.currentNegotiation,
      negotiations: negotiations ?? this.negotiations,
    );
  }
}
