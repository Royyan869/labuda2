import '../../domain/entities/like.dart';

/// Base state for Like feature
abstract class LikeState {
  const LikeState();
}

/// Initial state
class LikeInitial extends LikeState {
  const LikeInitial();
}

/// Loading state
class LikeLoading extends LikeState {
  const LikeLoading();
}

/// Data loaded state - stats cache
class LikeLoaded extends LikeState {
  final Map<String, LikeStats> statsCache;

  const LikeLoaded({this.statsCache = const {}});
}

/// Error state
class LikeError extends LikeState {
  final String message;

  const LikeError(this.message);
}

/// Result state for single operations
class LikeOperationResult extends LikeState {
  final bool success;
  final String? error;

  const LikeOperationResult({required this.success, this.error});
}
