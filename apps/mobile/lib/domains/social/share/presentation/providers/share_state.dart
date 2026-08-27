import '../../domain/domain.dart';

/// Base state for share operations
/// Presentation layer - uses freezed for immutability
sealed class ShareState {
  const ShareState();
}

/// Initial state - no operation has been started
class ShareInitial extends ShareState {
  const ShareInitial();
}

/// Loading state - operation in progress
class ShareLoading extends ShareState {
  const ShareLoading();
}

/// Success state - operation completed successfully
class ShareSuccess extends ShareState {
  final ShareResult result;
  final String? postId; // For shareAsPost operation

  const ShareSuccess({required this.result, this.postId});

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is ShareSuccess &&
          runtimeType == other.runtimeType &&
          result == other.result &&
          postId == other.postId;

  @override
  int get hashCode => result.hashCode ^ postId.hashCode;
}

/// Error state - operation failed
class ShareError extends ShareState {
  final ShareFailure failure;

  const ShareError(this.failure);

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is ShareError &&
          runtimeType == other.runtimeType &&
          failure == other.failure;

  @override
  int get hashCode => failure.hashCode;
}
