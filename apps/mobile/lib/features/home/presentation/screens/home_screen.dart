import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/features/home/home.dart';
import 'package:labuda/features/home/presentation/widgets/commerce_preview_section.dart';

/// Home Screen - Feed display dengan Clean Architecture
///
/// FEED / DISCOVERY QUALITY PASS V1:
///
/// PRODUCT CONTRACT:
/// - Home Feed is a SOCIAL-first timeline
/// - Displays: Universal content, reposts, and commerce previews
/// - NO commerce objects (listings, auctions, contests) - those belong in Explore
/// - Reposts are clearly distinguished with canonical RepostAttributionBar
/// - No fake engagement counts (hidden instead of showing "0")
class HomeScreen extends ConsumerStatefulWidget {
  const HomeScreen({super.key});

  @override
  ConsumerState<HomeScreen> createState() => _HomeScreenState();
}

class _HomeScreenState extends ConsumerState<HomeScreen> {
  // Distance from the bottom of the scroll extent (in logical pixels) at
  // which the next feed page should be fetched. Keep this large enough to
  // hide the network round-trip, small enough that we don't prefetch when
  // the user is barely scrolling.
  static const double _loadMoreThreshold = 400.0;

  @override
  void initState() {
    super.initState();
    // Set global callback untuk refresh feed dari mana saja
    setGlobalFeedRefreshCallback(() {
      if (mounted) {
        ref.invalidate(feedProvider);
      }
    });
  }

  /// Decide whether the current scroll position should trigger a
  /// pagination fetch, and dispatch it if so.
  ///
  /// All guards are checked against the most recent [FeedState]:
  ///   - skip while the initial load is in flight (isLoading)
  ///   - skip while a previous loadMore is in flight (isLoadingMore)
  ///   - skip after backend has reported has_more=false (hasReachedMax)
  ///   - skip while the feed is in an error state — user retries via the
  ///     "Try Again" button, not by scrolling
  ///   - skip if we haven't actually reached the threshold yet
  ///
  /// The notifier additionally holds a synchronous private lock, so even
  /// if the scroll listener fires faster than state can propagate, only
  /// one network request can be in flight at a time.
  void _maybeLoadMore(ScrollMetrics metrics, FeedState state) {
    if (state.isLoading) return;
    if (state.isLoadingMore) return;
    if (state.hasReachedMax) return;
    if (state.errorMessage != null) return;
    if (metrics.pixels < metrics.maxScrollExtent - _loadMoreThreshold) return;
    ref.read(feedProvider.notifier).loadMore();
  }

  @override
  Widget build(BuildContext context) {
    // Feed provider dari home module
    final feedState = ref.watch(feedProvider);

    return Column(
      children: [
        // HOME HEADER MESSAGE - Entry clarity
        _buildHomeHeader(context),
        // Feed content
        Expanded(child: _buildFeedContent(feedState)),
      ],
    );
  }

  /// HOME HEADER MESSAGE
  /// User langsung paham app ini apa
  Widget _buildHomeHeader(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
        border: Border(
          bottom: BorderSide(
            color: isDark ? AppColors.darkGray700 : AppColors.neutralGray200,
            width: 1,
          ),
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Title with fish emoji
          Row(
            children: [
              Text('🐟', style: TextStyle(fontSize: 20)),
              const SizedBox(width: 8),
              Text(
                'Komunitas & Marketplace Koi',
                style: TextStyle(
                  fontSize: 16,
                  fontWeight: FontWeight.w700,
                  color: isDark
                      ? AppColors.neutralWhite
                      : AppColors.neutralGray900,
                ),
              ),
            ],
          ),
          const SizedBox(height: 4),
          // Subtext
          Text(
            'Jual, beli, lelang, atau cari koi langsung dari sesama penghobi',
            style: TextStyle(
              fontSize: 13,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray600,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildFeedContent(FeedState feedState) {
    // Handle loading, error, and data states
    if (feedState.errorMessage != null) {
      return _buildError(feedState.errorMessage!);
    }

    if (feedState.isLoading && feedState.items.isEmpty) {
      return const Center(child: CircularProgressIndicator());
    }

    final feedItems = feedState.items;

    if (feedItems.isEmpty) {
      return _buildEmptyState();
    }

    return RefreshIndicator(
      onRefresh: () async {
        ref.invalidate(feedProvider);
        await Future.delayed(const Duration(milliseconds: 100));
      },
      child: NotificationListener<ScrollNotification>(
        onNotification: (notification) {
          // Drive pagination from the scroll position. Returning false
          // lets the notification continue propagating to RefreshIndicator
          // and any ancestor scroll observers.
          if (notification is ScrollUpdateNotification ||
              notification is ScrollEndNotification) {
            _maybeLoadMore(notification.metrics, feedState);
          }
          return false;
        },
        child: CustomScrollView(
          slivers: [
            // Upload progress indicator
            const SliverToBoxAdapter(child: UploadProgressWidget()),

            // COMMERCE PREVIEW: Sedang Laku Hari Ini
            // Shows active listings/auctions to demonstrate marketplace activity
            const SliverToBoxAdapter(child: CommercePreviewSection()),

            // Feed items
            SliverList(
              delegate: SliverChildBuilderDelegate((context, index) {
                final item = feedItems[index];
                return FeedCardFactory.buildCardForFeedItem(item, ref);
              }, childCount: feedItems.length),
            ),

            // Bottom pagination spinner. Rendered inside the scroll view
            // so it appears beneath the last loaded card without shifting
            // layout. Stays hidden when no fetch is in flight.
            if (feedState.isLoadingMore)
              const SliverToBoxAdapter(
                child: Padding(
                  padding: EdgeInsets.symmetric(vertical: 16),
                  child: Center(child: CircularProgressIndicator()),
                ),
              ),

            // Add spacing at bottom
            const SliverToBoxAdapter(child: SizedBox(height: 100)),
          ],
        ),
      ),
    );
  }

  Widget _buildEmptyState() {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Center(
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 24),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            // Icon with friendly emoji
            Container(
              width: 100,
              height: 100,
              decoration: BoxDecoration(
                color: AppColors.primaryRed.withValues(alpha: 0.1),
                shape: BoxShape.circle,
              ),
              child: const Icon(
                Icons.emoji_emotions_outlined,
                size: 48,
                color: AppColors.primaryRed,
              ),
            ),
            const SizedBox(height: 24),

            // Decision-based title
            Text(
              '🎯 Kamu ingin apa hari ini?',
              style: TextStyle(
                fontSize: 20,
                fontWeight: FontWeight.w700,
                color: isDark
                    ? AppColors.neutralWhite
                    : AppColors.neutralGray900,
              ),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 32),

            // PRIMARY ACTION: Cari & Beli Koi
            _buildPrimaryActionButton(
              icon: Icons.shopping_bag_outlined,
              label: 'Cari & Beli Koi',
              onTap: () => _navigateToExplore(context),
            ),
            const SizedBox(height: 12),

            // SECONDARY ACTION: Universal content composer
            _buildSecondaryActionButton(
              icon: Icons.post_add_outlined,
              label: 'Buat Konten',
              onTap: () => _navigateToCreateContent(context),
            ),
          ],
        ),
      ),
    );
  }

  /// Primary action button - filled style for main action
  Widget _buildPrimaryActionButton({
    required IconData icon,
    required String label,
    required VoidCallback onTap,
  }) {
    return SizedBox(
      width: 280,
      child: FilledButton.icon(
        icon: Icon(icon, size: 22),
        label: Text(
          label,
          style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w600),
        ),
        onPressed: onTap,
        style: FilledButton.styleFrom(
          backgroundColor: AppColors.primaryRed,
          foregroundColor: Colors.white,
          padding: const EdgeInsets.symmetric(vertical: 16, horizontal: 24),
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(14),
          ),
        ),
      ),
    );
  }

  /// Secondary action button - outlined style
  Widget _buildSecondaryActionButton({
    required IconData icon,
    required String label,
    required VoidCallback onTap,
  }) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return SizedBox(
      width: 280,
      child: OutlinedButton.icon(
        icon: Icon(icon, size: 20),
        label: Text(
          label,
          style: TextStyle(
            fontSize: 15,
            fontWeight: FontWeight.w500,
            color: isDark ? AppColors.neutralWhite : AppColors.neutralGray900,
          ),
        ),
        onPressed: onTap,
        style: OutlinedButton.styleFrom(
          foregroundColor: isDark
              ? AppColors.neutralWhite
              : AppColors.neutralGray900,
          side: BorderSide(
            color: isDark ? AppColors.darkGray600 : AppColors.neutralGray300,
          ),
          padding: const EdgeInsets.symmetric(vertical: 14, horizontal: 24),
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(14),
          ),
        ),
      ),
    );
  }

  void _navigateToCreateContent(BuildContext context) {
    // Trigger create content bottom sheet via navigation
    context.push(RoutePaths.createContent);
  }

  void _navigateToExplore(BuildContext context) {
    // Navigate to listings (marketplace browse)
    // push() preserves back-stack so Android Back returns to Home
    context.push(RoutePaths.forSales);
  }

  Widget _buildError(String error) {
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          const Icon(
            Icons.error_outline,
            size: 64,
            color: AppColors.statusError,
          ),
          const SizedBox(height: 16),
          const Text(
            'Feed belum bisa dimuat',
            style: TextStyle(fontSize: 18, fontWeight: FontWeight.w500),
          ),
          const SizedBox(height: 8),
          Text(
            error,
            style: const TextStyle(
              fontSize: 14,
              color: AppColors.neutralGray500,
            ),
            textAlign: TextAlign.center,
          ),
          const SizedBox(height: 16),
          ElevatedButton(
            onPressed: () => ref.refresh(feedProvider),
            child: const Text('Coba Lagi'),
          ),
        ],
      ),
    );
  }
}
