import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/social/rating/rating.dart';

/// CANONICAL Rating List Screen
///
/// Displays user ratings (received or given) using canonical fields.
///
/// Business Truth (LOCKED):
/// - Rating is IMMUTABLE (no edit/delete)
/// - Rating direction is BUYER → SELLER ONLY
/// - Only order-based ratings (no context tabs needed)
class RatingListScreen extends ConsumerStatefulWidget {
  final String userId;
  final bool isReceived;

  const RatingListScreen({
    super.key,
    required this.userId,
    this.isReceived = true,
  });

  @override
  ConsumerState<RatingListScreen> createState() => _RatingListScreenState();
}

class _RatingListScreenState extends ConsumerState<RatingListScreen> {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      ref
          .read(ratingProvider.notifier)
          .loadUserRatings(
            userId: widget.userId,
            isReceived: widget.isReceived,
          );
    });
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(ratingProvider);

    return PopScope(
      canPop: false,
      onPopInvokedWithResult: (didPop, result) {
        if (didPop) return;
        if (Navigator.of(context).canPop()) {
          Navigator.of(context).pop();
        }
      },
      child: Scaffold(
        appBar: AppBar(
          title: Text(widget.isReceived ? 'Reviews Received' : 'Reviews Given'),
          surfaceTintColor: Colors.transparent,
          scrolledUnderElevation: 0,
          leading: IconButton(
            icon: const Icon(Icons.arrow_back),
            onPressed: () {
              if (Navigator.of(context).canPop()) {
                Navigator.of(context).pop();
              }
            },
          ),
        ),
        body: Column(
          children: [
            if (state.summary != null) _buildSummaryHeader(state.summary!),
            Expanded(
              child: state.isLoading
                  ? const Center(child: CircularProgressIndicator())
                  : state.error != null
                  ? const Center(child: Text('Data belum bisa dimuat.'))
                  : state.ratings.isEmpty
                  ? _buildEmptyState()
                  : _buildRatingsList(state.ratings),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildSummaryHeader(RatingSummary summary) {
    return Container(
      padding: const EdgeInsets.all(16),
      child: Row(
        children: [
          Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                '${summary.averageRating.toStringAsFixed(1)}/5.0',
                style: Theme.of(context).textTheme.headlineMedium?.copyWith(
                  fontWeight: FontWeight.bold,
                ),
              ),
              Text('${summary.totalRatings} reviews'),
            ],
          ),
          const Spacer(),
          _buildStarRating(summary.averageRating),
        ],
      ),
    );
  }

  Widget _buildStarRating(double rating) {
    return Row(
      children: List.generate(5, (index) {
        return Icon(
          index < rating ? Icons.star : Icons.star_border,
          color: Colors.amber,
          size: 20,
        );
      }),
    );
  }

  Widget _buildEmptyState() {
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(
            Icons.star_border,
            size: 64,
            color: Theme.of(context).colorScheme.outline,
          ),
          const SizedBox(height: 16),
          Text('No reviews yet', style: Theme.of(context).textTheme.titleLarge),
          const SizedBox(height: 8),
          Text(
            widget.isReceived
                ? 'Complete orders to receive reviews'
                : 'Leave reviews for completed orders',
            style: Theme.of(context).textTheme.bodyMedium?.copyWith(
              color: Theme.of(context).colorScheme.onSurfaceVariant,
            ),
            textAlign: TextAlign.center,
          ),
        ],
      ),
    );
  }

  Widget _buildRatingsList(List<Rating> ratings) {
    return ListView.builder(
      itemCount: ratings.length,
      itemBuilder: (context, index) {
        return RatingCard(rating: ratings[index]);
      },
    );
  }
}
