import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/domain/entities/resource_projection.dart';
import 'package:labuda/shared/widgets/empty_state.dart';
import 'package:labuda/shared/widgets/page_error_state.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/dto/bidding_item_dto.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/my_bids_provider.dart';

/// My Bids screen — canonical projection of GET /api/v1/bidding.
///
/// Business truth (Owner-final): lists auctions with an OPEN bidding
/// process for the user (active + waiting_settlement). Ended / cancelled /
/// lapsed never appear here. "Bid saya" is the user's latest bid by time.
///
/// This screen is presentation only: visibility, status and ordering come
/// from the backend. Countdowns read auction end_at and never mutate
/// business state.
class MyBidsScreen extends ConsumerStatefulWidget {
  const MyBidsScreen({super.key});

  @override
  ConsumerState<MyBidsScreen> createState() => _MyBidsScreenState();
}

class _MyBidsScreenState extends ConsumerState<MyBidsScreen>
    with RouteAware, WidgetsBindingObserver {
  // Saved during didChangeDependencies: ref must never be read in dispose.
  RouteObserver<PageRoute>? _routeObserver;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    _routeObserver ??= ref.read(screenViewRouteObserverProvider);
    final route = ModalRoute.of(context);
    if (route is PageRoute) {
      _routeObserver!.subscribe(this, route);
    }
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    _routeObserver?.unsubscribe(this);
    super.dispose();
  }

  @override
  void didPopNext() {
    ref.invalidate(myBidsProvider);
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    // Converge to canonical backend state when returning from background:
    // an auction that ended while away must disappear on resume.
    if (state == AppLifecycleState.resumed && mounted) {
      ref.invalidate(myBidsProvider);
    }
  }

  @override
  Widget build(BuildContext context) {
    final ref = this.ref;
    final asyncItems = ref.watch(myBidsProvider);
    return Scaffold(
      appBar: AppBar(title: const Text('My Bids')),
      body: asyncItems.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (_, _) =>
            PageErrorState(onRetry: () => ref.invalidate(myBidsProvider)),
        data: (items) => RefreshIndicator(
          onRefresh: () => ref.refresh(myBidsProvider.future),
          child: items.isEmpty
              ? ListView(
                  children: const [
                    SizedBox(height: 220),
                    EmptyState(
                      title: 'Belum ada lelang aktif yang kamu bid',
                      subtitle:
                          'Auction yang kamu bid dan masih berjalan akan muncul di sini. '
                          'Auction yang sudah selesai tidak ditampilkan.',
                    ),
                  ],
                )
              : ListView.separated(
                  padding: const EdgeInsets.all(AppMetrics.p16),
                  itemCount: items.length,
                  separatorBuilder: (_, _) => const SizedBox(height: 8),
                  itemBuilder: (context, index) {
                    final item = items[index];
                    return _MyBidCard(item: item);
                  },
                ),
        ),
      ),
    );
  }
}

class _MyBidCard extends StatelessWidget {
  final BiddingItemDto item;

  const _MyBidCard({required this.item});

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final status = _statusMeta(context, item.status);
    return InkWell(
      onTap: () => context.push(RoutePaths.auctionDetail(item.auctionId)),
      borderRadius: BorderRadius.circular(AppShape.r8),
      child: Container(
        padding: const EdgeInsets.all(AppMetrics.p12),
        decoration: BoxDecoration(
          color: scheme.surfaceContainer,
          borderRadius: BorderRadius.circular(AppShape.r8),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Expanded(
                  child: Text(
                    item.title,
                    style: context.typeRoles.bodyDense.copyWith(
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                ),
                const Icon(Icons.chevron_right),
              ],
            ),
            const SizedBox(height: 8),
            Row(
              children: [
                Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: AppMetrics.p8,
                    vertical: AppMetrics.p4,
                  ),
                  decoration: BoxDecoration(
                    color: status.color.withValues(alpha: 0.12),
                    borderRadius: BorderRadius.circular(AppShape.r4),
                  ),
                  child: Text(
                    status.label,
                    style: context.typeRoles.labelMicro.copyWith(
                      color: status.color,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                ),
                const Spacer(),
                // Auction countdown runs for active auctions only.
                // waiting_claim shows settlement state, not a bid countdown.
                if (item.status == 'leading' || item.status == 'outbid')
                  MyBidCountdown(endAt: item.endAt),
              ],
            ),
            const SizedBox(height: 8),
            Row(
              children: [
                Expanded(
                  child: Text(
                    'Bid saya Rp ${formatGroupedAmount(item.yourLastBid)}',
                    style: context.typeRoles.bodyDense.copyWith(
                      fontWeight: FontWeight.bold,
                    ),
                  ),
                ),
                Text(
                  'Terkini Rp ${formatGroupedAmount(item.currentBid)}',
                  style: context.typeRoles.labelMicro.copyWith(
                    color: scheme.onSurfaceVariant,
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

/// Status presentation uses existing Labuda vocabulary:
/// leading → 'Anda Memimpin', outbid → 'Ter-Lelang',
/// waiting_claim → 'Menunggu Penyelesaian'. Unknown statuses are never
/// expected from the canonical API; they render neutrally, never as a
/// fabricated business state.
({String label, Color color}) _statusMeta(
  BuildContext context,
  String status,
) {
  final scheme = Theme.of(context).colorScheme;
  switch (status) {
    case 'leading':
      return (
        label: 'Anda Memimpin',
        color: context.statusColors.success,
      );
    case 'outbid':
      return (label: 'Ter-Lelang', color: scheme.error);
    case 'waiting_claim':
      return (
        label: 'Menunggu Penyelesaian',
        color: context.statusColors.warning,
      );
    default:
      return (label: status, color: scheme.onSurfaceVariant);
  }
}

/// Realtime presentation countdown from canonical auction end_at.
///
/// Presentation only: ticks locally, clamps at 00:00:00, and never changes
/// auction status or eligibility. The item disappears via canonical data
/// refresh once the backend moves the auction out of My Bids visibility.
///
/// The [now] clock defaults to wall-clock time; it is injectable so tests
/// can prove realtime ticking deterministically.
class MyBidCountdown extends StatefulWidget {
  final DateTime endAt;
  final DateTime Function() now;

  // ignore: prefer_const_constructors_in_immutables
  MyBidCountdown({super.key, required this.endAt, DateTime Function()? now})
    : now = now ?? DateTime.now;

  @override
  State<MyBidCountdown> createState() => _MyBidCountdownState();
}

class _MyBidCountdownState extends State<MyBidCountdown> {
  Timer? _timer;

  @override
  void initState() {
    super.initState();
    _timer = Timer.periodic(const Duration(seconds: 1), (_) {
      if (mounted) setState(() {});
    });
  }

  @override
  void dispose() {
    _timer?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final remaining = widget.endAt.difference(widget.now());
    final Color color;
    if (remaining.inHours < 1) {
      color = scheme.error;
    } else if (remaining.inHours < 6) {
      color = context.statusColors.warning;
    } else {
      color = scheme.onSurface;
    }
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(Icons.access_time, size: AppIconSize.inlineGlyph, color: color),
        const SizedBox(width: 4),
        Text(
          _formatDuration(remaining),
          style: context.typeRoles.titleCompact.copyWith(
            fontWeight: FontWeight.bold,
            color: color,
          ),
        ),
      ],
    );
  }
}

String _formatDuration(Duration duration) {
  if (duration.isNegative) return '00:00:00';
  final hours = duration.inHours;
  final minutes = duration.inMinutes.remainder(60);
  final seconds = duration.inSeconds.remainder(60);
  return '${hours.toString().padLeft(2, '0')}:'
      '${minutes.toString().padLeft(2, '0')}:'
      '${seconds.toString().padLeft(2, '0')}';
}
