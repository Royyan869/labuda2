import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/rating/domain/entities/rating_entity.dart';

// Import data layer providers (already migrated to Riverpod)
// This re-exports IRatingRepository and provides ratingRepositoryProvider
import 'package:labuda/domains/social/rating/data/rating_providers.dart';

part 'rating_provider.g.dart';

// =============================================================================
// DATA LAYER PROVIDERS - RE-EXPORT FROM DATA LAYER
// =============================================================================
// The following providers are now imported from data/rating_providers.dart:
// - ratingApiDatasourceProvider (uses apiClientProvider from core)
// - ratingRepositoryProvider (uses apiClientProvider from core)
//
// These are pure Riverpod providers - no ServiceLocator usage.

/// Get ratings for a specific order
@riverpod
Future<Result<List<Rating>>> getRatingsForOrder(Ref ref, String orderId) async {
  final repository = ref.watch(ratingRepositoryProvider);
  final result = await repository.getRatingForOrder(orderId: orderId);
  if (!result.isSuccess) {
    return Result.error(result.error ?? 'Failed to get ratings for order');
  }

  final rating = result.data;
  return Result.success(rating == null ? const <Rating>[] : [rating]);
}

/// Check if user has already rated a specific order
@riverpod
Future<bool> hasUserRatedOrder(
  Ref ref, {
  required String orderId,
  required String buyerId,
  required String sellerId,
}) async {
  final repository = ref.watch(ratingRepositoryProvider);
  final result = await repository.getRatingForOrder(orderId: orderId);

  if (result.isSuccess && result.data != null) {
    final rating = result.data!;
    // Verify the rating is from the buyer to the seller
    return rating.buyerId == buyerId && rating.sellerId == sellerId;
  }

  return false;
}

/// Get user's rating summary
@riverpod
Future<Result<RatingSummary>> getUserRatingSummary(
  Ref ref, {
  required String userId,
}) async {
  final repository = ref.watch(ratingRepositoryProvider);
  return repository.getRatingSummary(sellerId: userId);
}

/// Rating List State for displaying ratings
class RatingListState {
  final List<Rating> ratings;
  final bool isLoading;
  final String? error;
  final RatingSummary? summary;

  const RatingListState({
    this.ratings = const [],
    this.isLoading = false,
    this.error,
    this.summary,
  });

  RatingListState copyWith({
    List<Rating>? ratings,
    bool? isLoading,
    String? error,
    RatingSummary? summary,
  }) {
    return RatingListState(
      ratings: ratings ?? this.ratings,
      isLoading: isLoading ?? this.isLoading,
      error: error ?? this.error,
      summary: summary ?? this.summary,
    );
  }
}

/// CANONICAL Rating List Notifier for RatingListScreen
///
/// Handles loading user ratings (received or given).
///
/// Business Truth (LOCKED):
/// - Rating is IMMUTABLE (no edit/delete, no helpful voting)
/// - Rating direction is BUYER → SELLER ONLY
@riverpod
class RatingNotifier extends _$RatingNotifier {
  @override
  RatingListState build() {
    return const RatingListState();
  }

  /// Load user ratings (received or given)
  Future<void> loadUserRatings({
    required String userId,
    bool? isReceived,
  }) async {
    state = state.copyWith(isLoading: true, error: null);

    final repository = ref.read(ratingRepositoryProvider);

    // Load ratings using canonical methods
    final ratingsResult = await (isReceived == true
        ? repository.getRatingsReceived(sellerId: userId)
        : repository.getRatingsGiven());

    // Load summary for received ratings
    final summaryResult = isReceived == true
        ? await repository.getRatingSummary(sellerId: userId)
        : null;

    if (ratingsResult.isSuccess) {
      state = state.copyWith(
        ratings: ratingsResult.data!,
        summary: summaryResult != null && summaryResult.isSuccess
            ? summaryResult.data
            : null,
        isLoading: false,
      );
    } else {
      state = state.copyWith(isLoading: false, error: ratingsResult.error);
    }
  }

  /// Clear error
  void clearError() {
    state = state.copyWith(error: null);
  }
}

/// Rating Management State
class RatingManagementState {
  final bool isProcessing;
  final String? error;

  const RatingManagementState({this.isProcessing = false, this.error});

  RatingManagementState copyWith({bool? isProcessing, String? error}) {
    return RatingManagementState(
      isProcessing: isProcessing ?? this.isProcessing,
      error: error ?? this.error,
    );
  }
}

/// CANONICAL Rating Management Notifier
///
/// Handles creating ratings for orders.
///
/// Business Truth (LOCKED):
/// - Rating is IMMUTABLE (no edit/delete)
/// - Only order-based ratings
@riverpod
class RatingManagement extends _$RatingManagement {
  @override
  RatingManagementState build() {
    return const RatingManagementState();
  }

  /// Create new rating for an order
  Future<Rating?> createRatingForOrder({
    required String orderId,
    required int ratingValue,
    String? comment,
  }) async {
    // Domain validation
    if (ratingValue < 1 || ratingValue > 5) {
      state = state.copyWith(
        isProcessing: false,
        error: 'Rating harus antara 1-5',
      );
      return null;
    }

    state = state.copyWith(isProcessing: true, error: null);

    final repository = ref.read(ratingRepositoryProvider);
    final result = await repository.createRatingForOrder(
      orderId: orderId,
      ratingValue: ratingValue,
      comment: comment,
    );

    if (result.isSuccess) {
      state = state.copyWith(isProcessing: false);
      return result.data;
    } else {
      state = state.copyWith(isProcessing: false, error: result.error);
      return null;
    }
  }

  /// Clear error
  void clearError() {
    state = state.copyWith(error: null);
  }
}
