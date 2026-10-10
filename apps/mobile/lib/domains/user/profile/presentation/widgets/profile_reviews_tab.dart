import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/shared/shared.dart';
import 'package:hishumi/domains/social/rating/rating.dart';
import 'package:hishumi/domains/user/profile/profile.dart' show userDataProvider;
import 'profile_reviews_tab/rating_overview_section.dart';
import 'profile_reviews_tab/reviews_empty_state.dart';

/// CANONICAL Reviews Tab for User Profile
///
/// Features:
/// - Overall rating display with stars
/// - Rating breakdown per stars (5-1)
/// - Individual review cards with user info
/// - Professional layout design
/// - Empty state for no reviews
/// - Professional card styling
/// - Support for seller (ratings received) and buyer (ratings given)
/// - Sub-tabs for seller: Received vs Given
///
/// Business Truth (LOCKED):
/// - Rating is IMMUTABLE (no edit/delete, no helpful voting)
/// - Rating direction is BUYER → SELLER ONLY
/// - Only order-based ratings (verified purchase)
class ProfileReviewsTab extends ConsumerStatefulWidget {
  final String userId;
  final bool isSeller;

  const ProfileReviewsTab({
    super.key,
    required this.userId,
    this.isSeller = true, // Default: seller profile
  });

  @override
  ConsumerState<ProfileReviewsTab> createState() => _ProfileReviewsTabState();
}

class _ProfileReviewsTabState extends ConsumerState<ProfileReviewsTab>
    with SingleTickerProviderStateMixin {
  String _selectedFilter = 'All';
  final List<String> _filterOptions = [
    'All',
    '5 Stars',
    '4 Stars',
    '3 Stars',
    '2 Stars',
    '1 Star',
  ];

  late TabController _subTabController;
  int _currentSubTab = 0;

  @override
  void initState() {
    super.initState();

    // Initialize sub-tab controller for seller (2 tabs: Received, Given)
    if (widget.isSeller) {
      _subTabController = TabController(length: 2, vsync: this);
      _subTabController.addListener(() {
        if (!_subTabController.indexIsChanging) {
          setState(() {
            _currentSubTab = _subTabController.index;
          });
          _loadRatingsForCurrentTab();
        }
      });
    }

    // Load ratings on mount - only once
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) {
        _loadRatingsForCurrentTab();
      }
    });
  }

  @override
  void didUpdateWidget(ProfileReviewsTab oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.userId != widget.userId) {
      setState(() {
        _selectedFilter = 'All';
        _currentSubTab = 0;
      });
      _loadRatingsForCurrentTab();
    }
  }

  @override
  void dispose() {
    if (widget.isSeller) {
      _subTabController.dispose();
    }
    super.dispose();
  }

  void _loadRatingsForCurrentTab() {
    if (!mounted) return;

    if (widget.isSeller) {
      // Seller: Tab 0 = Received, Tab 1 = Given
      final isReceived = _currentSubTab == 0;
      ref
          .read(ratingProvider.notifier)
          .loadUserRatings(userId: widget.userId, isReceived: isReceived);
    } else {
      // Buyer: Only show given ratings
      ref
          .read(ratingProvider.notifier)
          .loadUserRatings(userId: widget.userId, isReceived: false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;

    final ratingsAsync = ref.watch(
      getUserRatingSummaryProvider(userId: widget.userId),
    );

    final userRatingsAsync = ref.watch(ratingProvider);

    return ratingsAsync.when(
      data: (summaryResult) {
        if (summaryResult.isError) {
          return Center(child: const Text('Data belum bisa dimuat.'));
        }

        final summary = summaryResult.data!;
        final ratings = userRatingsAsync.ratings;

        // Filter ratings based on selected filter
        final filteredRatings = _filterRatings(ratings);

        // Determine loading state
        final bool isLoadingRatings;
        if (!widget.isSeller || _currentSubTab == 0) {
          isLoadingRatings =
              userRatingsAsync.isLoading ||
              (summary.totalRatings > 0 && ratings.isEmpty);
        } else {
          isLoadingRatings = userRatingsAsync.isLoading;
        }

        return CustomScrollView(
          slivers: [
            // Sub-tabs for seller (Received vs Given)
            if (widget.isSeller)
              SliverToBoxAdapter(
                child: Container(
                  color: scheme.surface,
                  child: TabBar(
                    controller: _subTabController,
                    tabs: const [
                      Tab(text: 'Diterima'),
                      Tab(text: 'Diberikan'),
                    ],
                  ),
                ),
              ),

            // Rating overview - ONLY for "Received" tab (seller only)
            if (widget.isSeller && _currentSubTab == 0)
              SliverToBoxAdapter(
                child: RatingOverviewSection(
                  averageRating: summary.averageRating,
                  totalReviews: summary.totalRatings,
                  ratingBreakdown: summary.distribution,
                ),
              ),

            // Filter section (only show if there are ratings)
            if ((!widget.isSeller || _currentSubTab == 1)
                ? ratings.isNotEmpty
                : summary.totalRatings > 0)
              SliverToBoxAdapter(child: _buildFilterSection(context)),

            // Loading indicator. Non-scrollable state → the canonical
            // bounded-tab-cell rule: hasScrollBody:false lets the sliver grow
            // to max(remaining, intrinsic) so a short viewport scrolls rather
            // than clamping (and overflowing) the state.
            if (isLoadingRatings)
              const SliverFillRemaining(
                hasScrollBody: false,
                child: Center(child: CircularProgressIndicator()),
              )
            // Empty state (non-scrollable) → same canonical rule.
            else if ((!widget.isSeller || _currentSubTab == 0)
                ? summary.totalRatings == 0
                : ratings.isEmpty)
              const SliverFillRemaining(
                hasScrollBody: false,
                child: ReviewsEmptyState(),
              )
            // Empty filtered results (non-scrollable) → same canonical rule.
            else if (filteredRatings.isEmpty)
              SliverFillRemaining(
                hasScrollBody: false,
                child: Center(
                  child: Padding(
                    padding: const EdgeInsets.all(AppMetrics.p24),
                    child: Column(
                      mainAxisAlignment: MainAxisAlignment.center,
                      children: [
                        Icon(
                          Icons.filter_list_off,
                          size: AppIconSize.display,
                          color: scheme.outline,
                        ),
                        const SizedBox(height: 16),
                        Text(
                          'No ratings with filter "$_selectedFilter"',
                          textAlign: TextAlign.center,
                          style: context.typeRoles.titleCompact.copyWith(
                            color: scheme.onSurfaceVariant,
                          ),
                        ),
                        const SizedBox(height: 8),
                        TextButton(
                          onPressed: () {
                            setState(() {
                              _selectedFilter = 'All';
                            });
                          },
                          child: const Text('Reset Filter'),
                        ),
                      ],
                    ),
                  ),
                ),
              )
            // Show ratings list
            else
              SliverPadding(
                padding: const EdgeInsets.all(AppMetrics.p16),
                sliver: SliverList(
                  delegate: SliverChildBuilderDelegate(
                    (context, index) =>
                        _buildReviewCard(filteredRatings[index]),
                    childCount: filteredRatings.length,
                  ),
                ),
              ),
          ],
        );
      },
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (error, stack) =>
          const Center(child: Text('Data belum bisa dimuat.')),
    );
  }

  Widget _buildFilterSection(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.symmetric(
        horizontal: AppMetrics.p16,
        vertical: AppMetrics.p8,
      ),
      color: scheme.surface,
      child: Wrap(
        spacing: 8,
        runSpacing: 8,
        children: _filterOptions.map((filter) {
          final isSelected = _selectedFilter == filter;
          return FilterChip(
            label: Text(filter),
            selected: isSelected,
            onSelected: (selected) {
              setState(() {
                _selectedFilter = filter;
              });
            },
            backgroundColor: scheme.surfaceContainerHigh,
            selectedColor: scheme.primary.withValues(alpha: 0.2),
            checkmarkColor: scheme.primary,
            labelStyle: TextStyle(
              color: isSelected ? scheme.primary : scheme.onSurfaceVariant,
              fontWeight: isSelected ? FontWeight.w600 : FontWeight.normal,
            ),
          );
        }).toList(),
      ),
    );
  }

  /// Filter ratings based on selected filter
  List<Rating> _filterRatings(List<Rating> ratings) {
    if (_selectedFilter == 'All') {
      return ratings;
    }

    // Extract star count from filter (e.g., "5 Stars" -> 5)
    final starCount = int.tryParse(_selectedFilter.split(' ').first);
    if (starCount == null) return ratings;

    return ratings.where((r) => r.ratingValue == starCount).toList();
  }

  /// Build review card widget with user data
  Widget _buildReviewCard(Rating rating) {
    // For received ratings: show buyer info (who gave the rating)
    // For given ratings: show seller info (who received the rating)
    final isReceived = _currentSubTab == 0 && widget.isSeller;
    final authorUserId = isReceived ? rating.buyerId : rating.sellerId;

    final authorDataAsync = ref.watch(userDataProvider(authorUserId));

    return authorDataAsync.when(
      data: (author) {
        return _buildSimpleReviewCard(
          rating: rating,
          author: author,
          isReceived: isReceived,
        );
      },
      loading: () => const SizedBox(
        height: 100,
        child: Center(child: CircularProgressIndicator()),
      ),
      error: (error, stack) => _buildSimpleReviewCard(
        rating: rating,
        author: null,
        isReceived: isReceived,
      ),
    );
  }

  Widget _buildSimpleReviewCard({
    required Rating rating,
    dynamic author,
    required bool isReceived,
  }) {
    final scheme = Theme.of(context).colorScheme;

    return Card(
      margin: const EdgeInsets.only(bottom: AppMetrics.p16),
      // Canonical card direction is flat — no depth for a review row.
      elevation: AppElevation.none,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(AppShape.r12),
      ),
      color: scheme.surface,
      child: Padding(
        padding: const EdgeInsets.all(AppMetrics.p16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // Reviewer info
            Row(
              children: [
                ProfileAvatar(
                  userId: (isReceived ? rating.buyerId : rating.sellerId),
                  size: 40,
                  imageUrl: author?.avatarUrl,
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        '@${author?.username ?? 'User'}',
                        style: context.typeRoles.bodyDense.copyWith(
                          fontWeight: FontWeight.w600,
                          color: scheme.onSurface,
                        ),
                      ),
                      if (!isReceived) ...[
                        Text(
                          'Rated this seller',
                          style: context.typeRoles.labelMicro.copyWith(
                            color: scheme.onSurfaceVariant,
                          ),
                        ),
                      ],
                    ],
                  ),
                ),
                // Rating and date
                Column(
                  crossAxisAlignment: CrossAxisAlignment.end,
                  children: [
                    _buildStarRating(context, rating.ratingValue, 14),
                    const SizedBox(height: 2),
                    TimeAgoWidget.compact(
                      dateTime: rating.createdAt,
                      color: scheme.onSurfaceVariant,
                      fontSize: context.typeRoles.labelMicro.fontSize,
                    ),
                  ],
                ),
              ],
            ),
            if (rating.comment != null && rating.comment!.isNotEmpty) ...[
              const SizedBox(height: 12),
              Text(
                rating.comment!,
                style: context.typeRoles.bodyDense.copyWith(
                  height: 1.4,
                  color: scheme.onSurface,
                ),
              ),
            ],
            const SizedBox(height: 8),
            Text(
              'Verified Purchase',
              style: context.typeRoles.labelMicro.copyWith(
                color: context.statusColors.success,
                fontWeight: FontWeight.w500,
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildStarRating(BuildContext context, int rating, double size) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: List.generate(5, (index) {
        return Icon(
          index < rating ? Icons.star : Icons.star_border,
          size: size,
          color: AppColors.koiGold,
        );
      }),
    );
  }
}
