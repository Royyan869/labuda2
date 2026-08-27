import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/features/home/home.dart';
import 'package:labuda/features/explore/explore.dart';

/// Explore Screen - Central hub untuk Product dan Auction
///
/// PRODUCT CONTRACT:
/// - Explore is a COMMERCE-first browse surface
/// - Displays: Listings, Auctions
/// - NO social content (universal content and reposts) - those belong in Home Feed
/// - Promoted/sponsored items appear as injected cards within Listing/Auction tabs
///   (server-side injection via FeedPromotionInjector / SearchPromotionInjector).
///   There is NO standalone Promo tab — promotion is always interleaved, not siloed.
///
/// Struktur:
/// - Tab 1: For Sale (Product Catalog)
/// - Tab 2: Auction (Auction List)
///
/// This screen is designed to be used in bottom navigation,
/// without hamburger menu or back button.
///
/// Refactored to use clean architecture pattern.
class ExploreScreen extends ConsumerStatefulWidget {
  /// Initial tab index to show (0=Listing/For Sale, 1=Auction)
  final int initialTab;

  const ExploreScreen({super.key, this.initialTab = 0});

  @override
  ConsumerState<ExploreScreen> createState() => _ExploreScreenState();
}

class _ExploreScreenState extends ConsumerState<ExploreScreen>
    with SingleTickerProviderStateMixin {
  late TabController _tabController;

  @override
  void initState() {
    super.initState();
    _tabController = TabController(
      length: 2,
      vsync: this,
      initialIndex: widget.initialTab > 1
          ? 0
          : widget.initialTab, // Clamp to valid range
    );

    // Check pending switch setelah widget ready
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _handlePendingSwitch();
    });
  }

  void _handlePendingSwitch() {
    final pending = ref.read(pendingTabSwitchProvider);
    if (pending.hasSwitch && pending.target == 'explore' && mounted) {
      final subTab = pending.exploreSubTab ?? 0;
      _tabController.animateTo(subTab);
      // Clear pending switch
      ref.read(pendingTabSwitchProvider.notifier).clear();
    }
  }

  @override
  void dispose() {
    _tabController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Container(
      color: isDark ? AppColors.darkGray900 : AppColors.neutralGray50,
      child: Column(
        children: [
          // Clean Tab Bar Header (no redundant buttons)
          Container(
            decoration: BoxDecoration(
              color: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
              border: Border(
                bottom: BorderSide(
                  color: isDark
                      ? AppColors.neutralGray700
                      : AppColors.neutralGray200,
                  width: 1,
                ),
              ),
            ),
            child: TabBar(
              controller: _tabController,
              indicatorColor: AppColors.primaryRed,
              labelColor: AppColors.primaryRed,
              unselectedLabelColor: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray600,
              labelStyle: const TextStyle(
                fontSize: 14,
                fontWeight: FontWeight.w600,
              ),
              unselectedLabelStyle: const TextStyle(
                fontSize: 14,
                fontWeight: FontWeight.w400,
              ),
              tabs: const [
                Tab(text: 'For Sale'),
                Tab(text: 'Auction'),
              ],
            ),
          ),
          // Tab Content
          Expanded(
            child: TabBarView(
              controller: _tabController,
              children: const [
                // Tab 1: For Sale Catalog Content
                ExploreForSaleTab(),

                // Tab 2: Auction List Content
                ExploreAuctionTab(),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
