import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/rating/rating.dart';

enum ProfileRatingSummaryStatus { loading, empty, populated, unavailable }

/// Canonical presentation state for public profile rating summaries.
///
/// Loading, empty, populated, and unavailable states stay separate so the UI
/// never collapses transport failures into a fake zero.
class ProfileRatingSummaryState {
  final ProfileRatingSummaryStatus status;
  final double? averageRating;
  final int? totalReviews;
  final String? errorMessage;

  const ProfileRatingSummaryState._({
    required this.status,
    this.averageRating,
    this.totalReviews,
    this.errorMessage,
  });

  const ProfileRatingSummaryState.loading()
    : this._(status: ProfileRatingSummaryStatus.loading);

  const ProfileRatingSummaryState.empty({
    double averageRating = 0.0,
    int totalReviews = 0,
  }) : this._(
         status: ProfileRatingSummaryStatus.empty,
         averageRating: averageRating,
         totalReviews: totalReviews,
       );

  const ProfileRatingSummaryState.populated({
    required double averageRating,
    required int totalReviews,
  }) : this._(
         status: ProfileRatingSummaryStatus.populated,
         averageRating: averageRating,
         totalReviews: totalReviews,
       );

  const ProfileRatingSummaryState.unavailable([String? errorMessage])
    : this._(
        status: ProfileRatingSummaryStatus.unavailable,
        errorMessage: errorMessage,
      );

  factory ProfileRatingSummaryState.fromAsyncValue(
    AsyncValue<Result<RatingSummary>> asyncValue,
  ) {
    return asyncValue.when(
      loading: () => const ProfileRatingSummaryState.loading(),
      error: (error, _) =>
          ProfileRatingSummaryState.unavailable(error.toString()),
      data: (result) {
        if (!result.isSuccess || result.data == null) {
          return ProfileRatingSummaryState.unavailable(result.error);
        }

        final summary = result.data!;
        if (summary.totalRatings <= 0) {
          return ProfileRatingSummaryState.empty(
            averageRating: summary.averageRating,
            totalReviews: summary.totalRatings,
          );
        }

        return ProfileRatingSummaryState.populated(
          averageRating: summary.averageRating,
          totalReviews: summary.totalRatings,
        );
      },
    );
  }

  bool get isLoading => status == ProfileRatingSummaryStatus.loading;

  bool get isEmpty => status == ProfileRatingSummaryStatus.empty;

  bool get isPopulated => status == ProfileRatingSummaryStatus.populated;

  bool get isUnavailable => status == ProfileRatingSummaryStatus.unavailable;

  String get averageText {
    return switch (status) {
      ProfileRatingSummaryStatus.loading => '...',
      ProfileRatingSummaryStatus.unavailable => '\u2014',
      ProfileRatingSummaryStatus.empty ||
      ProfileRatingSummaryStatus.populated =>
        (averageRating ?? 0.0).toStringAsFixed(1),
    };
  }

  String get totalText {
    return switch (status) {
      ProfileRatingSummaryStatus.loading => '...',
      ProfileRatingSummaryStatus.unavailable => '\u2014',
      ProfileRatingSummaryStatus.empty ||
      ProfileRatingSummaryStatus.populated => (totalReviews ?? 0).toString(),
    };
  }

  String get averageLabel {
    return switch (status) {
      ProfileRatingSummaryStatus.loading => 'Memuat rating',
      ProfileRatingSummaryStatus.empty => 'Belum ada ulasan',
      ProfileRatingSummaryStatus.populated =>
        totalReviews == 1 ? 'Rating (1 ulasan)' : 'Rating ($totalReviews)',
      ProfileRatingSummaryStatus.unavailable => 'Rating unavailable',
    };
  }

  String get totalLabel {
    return switch (status) {
      ProfileRatingSummaryStatus.loading => 'Memuat ulasan',
      ProfileRatingSummaryStatus.empty => 'Total Reviews',
      ProfileRatingSummaryStatus.populated => 'Total Reviews',
      ProfileRatingSummaryStatus.unavailable => 'Rating unavailable',
    };
  }
}
