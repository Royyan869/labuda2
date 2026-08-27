import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/domains/social/rating/rating.dart';
import 'package:labuda/domains/user/profile/profile.dart' show userDataProvider;
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
    final isDark = Theme.of(context).brightness == Brightness.dark;

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
                  color: isDark
                      ? AppColors.darkGray800
                      : AppColors.neutralWhite,
                  child: TabBar(
                    controller: _subTabController,
                    labelColor: AppColors.primaryRed,
                    unselectedLabelColor: isDark
                        ? AppColors.neutralGray400
                        : AppColors.neutralGray500,
                    indicatorColor: AppColors.primaryRed,
                    indicatorWeight: 2,
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
              SliverToBoxAdapter(child: _buildFilterSection(isDark)),

            // Loading indicator
            if (isLoadingRatings)
              const SliverFillRemaining(
                child: Center(child: CircularProgressIndicator()),
              )
            // Empty state
            else if ((!widget.isSeller || _currentSubTab == 0)
                ? summary.totalRatings == 0
                : ratings.isEmpty)
              const SliverFillRemaining(child: ReviewsEmptyState())
            // Empty filtered results
            else if (filteredRatings.isEmpty)
              SliverFillRemaining(
                child: Center(
                  child: Padding(
                    padding: const EdgeInsets.all(24),
                    child: Column(
                      mainAxisAlignment: MainAxisAlignment.center,
                      children: [
                        Icon(
                          Icons.filter_list_off,
                          size: 64,
                          color: Colors.grey[400],
                        ),
                        const SizedBox(height: 16),
                        Text(
                          'No ratings with filter "$_selectedFilter"',
                          textAlign: TextAlign.center,
                          style: TextStyle(
                            fontSize: 16,
                            color: Colors.grey[600],
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
                padding: const EdgeInsets.all(16),
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

  Widget _buildFilterSection(bool isDark) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
      color: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
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
            backgroundColor: isDark
                ? AppColors.darkGray700
                : AppColors.neutralGray100,
            selectedColor: AppColors.primaryRed.withValues(alpha: 0.2),
            checkmarkColor: AppColors.primaryRed,
            labelStyle: TextStyle(
              color: isSelected ? AppColors.primaryRed : Colors.grey,
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
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Card(
      margin: const EdgeInsets.only(bottom: 16),
      elevation: isDark ? 4 : 2,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
      color: isDark ? AppColors.darkGray700 : AppColors.neutralWhite,
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // Reviewer info
            Row(
              children: [
                CircleAvatar(
                  radius: 20,
                  backgroundColor: AppColors.primaryRed.withValues(alpha: 0.1),
                  backgroundImage: author?.avatarUrl != null
                      ? NetworkImage(author.avatarUrl)
                      : null,
                  child: author?.avatarUrl == null
                      ? Text(
                          author?.username?.isNotEmpty == true
                              ? author.username.substring(0, 1).toUpperCase()
                              : 'U',
                          style: const TextStyle(
                            color: AppColors.primaryRed,
                            fontWeight: FontWeight.bold,
                          ),
                        )
                      : null,
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        '@${author?.username ?? 'User'}',
                        style: TextStyle(
                          fontSize: 14,
                          fontWeight: FontWeight.w600,
                          color: isDark
                              ? AppColors.neutralWhite
                              : AppColors.neutralGray900,
                        ),
                      ),
                      if (!isReceived) ...[
                        Text(
                          'Rated this seller',
                          style: TextStyle(
                            fontSize: 12,
                            color: isDark
                                ? AppColors.neutralGray400
                                : AppColors.neutralGray500,
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
                    _buildStarRating(rating.ratingValue, 14),
                    const SizedBox(height: 2),
                    TimeAgoWidget.compact(
                      dateTime: rating.createdAt,
                      color: isDark
                          ? AppColors.neutralGray400
                          : AppColors.neutralGray500,
                      fontSize: 11,
                    ),
                  ],
                ),
              ],
            ),
            if (rating.comment != null && rating.comment!.isNotEmpty) ...[
              const SizedBox(height: 12),
              Text(
                rating.comment!,
                style: TextStyle(
                  fontSize: 14,
                  height: 1.4,
                  color: isDark
                      ? AppColors.neutralGray200
                      : AppColors.neutralGray700,
                ),
              ),
            ],
            const SizedBox(height: 8),
            Text(
              'Verified Purchase',
              style: TextStyle(
                fontSize: 11,
                color: AppColors.success,
                fontWeight: FontWeight.w500,
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildStarRating(int rating, double size) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: List.generate(5, (index) {
        return Icon(
          index < rating ? Icons.star : Icons.star_border,
          size: size,
          color: Colors.amber,
        );
      }),
    );
  }
}
