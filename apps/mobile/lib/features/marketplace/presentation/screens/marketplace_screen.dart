import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/features/home/home.dart';
import 'package:labuda/features/marketplace/marketplace.dart';

/// Marketplace Screen - Central hub untuk Product dan Auction
///
/// PRODUCT CONTRACT:
/// - Marketplace is a COMMERCE-first browse surface
/// - Displays: ForSales, Auctions
/// - NO social content (universal content and reposts) - those belong in Home Feed
/// - Promoted/sponsored items appear as injected cards within ForSale/Auction tabs
///   (server-side injection via FeedPromotionInjector / SearchPromotionInjector).
///   There is NO standalone Promo tab — promotion is always interleaved, not siloed.
///
/// Struktur:
/// - Tab 1: For Sale (Product Catalog)
/// - Tab 2: Auction (Auction List)
class MarketplaceScreen extends ConsumerStatefulWidget {
  /// Initial tab index to show (0=ForSale/For Sale, 1=Auction)
  final int initialTab;

  const MarketplaceScreen({super.key, this.initialTab = 0});

  @override
  ConsumerState<MarketplaceScreen> createState() => _MarketplaceScreenState();
}

class _MarketplaceScreenState extends ConsumerState<MarketplaceScreen>
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

    // Cover a switch that was set before the first build.
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _handlePendingSwitch();
    });
  }

  /// Consumes a pending switch to this screen and its sub-tab.
  ///
  /// This screen lives in an `IndexedStack`, so `initState` runs exactly ONCE —
  /// reading the pending switch there meant the second and later requests (the
  /// Create FAB, deep links) were silently dropped. The listener below is the
  /// authority; this post-frame read only covers a switch set before mount.
  void _handlePendingSwitch() {
    final pending = ref.read(pendingTabSwitchProvider);
    if (pending.hasSwitch && pending.target == 'marketplace' && mounted) {
      _tabController.animateTo(pending.marketplaceSubTab ?? 0);
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
    final scheme = Theme.of(context).colorScheme;

    // Every change, not just the first: main screen only moves the OUTER tab,
    // and this is the one side that owns the sub-tab (and the clearing).
    ref.listen(pendingTabSwitchProvider, (previous, next) {
      if (!next.hasSwitch || next.target != 'marketplace' || !mounted) return;
      _tabController.animateTo(next.marketplaceSubTab ?? 0);
      ref.read(pendingTabSwitchProvider.notifier).clear();
    });

    return Container(
      color: scheme.surfaceContainerLowest,
      child: Column(
        children: [
          // Clean Tab Bar Header (no redundant buttons)
          Container(
            decoration: BoxDecoration(
              color: scheme.surface,
              border: Border(
                bottom: BorderSide(color: scheme.outlineVariant, width: 1),
              ),
            ),
            child: TabBar(
              controller: _tabController,
              // Role, not size: the tab label is `labelLarge` (14 — the
              // same metrics the TabBar default titleSmall carries), and the
              // selected/unselected weights stay the call site's own decision.
              labelStyle: Theme.of(context).textTheme.labelLarge?.copyWith(
                fontWeight: FontWeight.w600,
              ),
              unselectedLabelStyle:
                  Theme.of(context).textTheme.labelLarge?.copyWith(
                    fontWeight: FontWeight.w400,
                  ),
              tabs: const [
                Tab(text: 'For Sale'),
                Tab(text: 'Auction'),
              ],
            ),
          ),
          // NO create entry here — by owner decision the marketplace surface
          // carries zero create buttons. The single create entry is the bottom
          // bar (main screen), which pushes the form and, on success, lands the
          // user back on the matching marketplace tab via pendingTabSwitch.
          // Tab Content
          Expanded(
            child: TabBarView(
              controller: _tabController,
              children: const [
                // Tab 1: For Sale Catalog Content
                MarketplaceForSaleTab(),

                // Tab 2: Auction List Content
                MarketplaceAuctionTab(),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
