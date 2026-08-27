/// Block Action State
///
/// State class untuk BlockActionsNotifier.
library;

/// Block actions state
class BlockActionState {
  final bool isLoading;
  final String? error;
  final String? successMessage;

  const BlockActionState({
    this.isLoading = false,
    this.error,
    this.successMessage,
  });

  BlockActionState copyWith({
    bool? isLoading,
    String? error,
    String? successMessage,
  }) {
    return BlockActionState(
      isLoading: isLoading ?? this.isLoading,
      error: error,
      successMessage: successMessage,
    );
  }
}
