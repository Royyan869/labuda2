/// My Reports Screen
///
/// User-facing screen to view submitted reports and their status.
/// Displays canonical derived state from Case + Decision.
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/system/shared/domain/services/time_format_service.dart';
import 'package:labuda/domains/system/report/domain/entities/entities.dart';
import 'package:labuda/domains/system/report/presentation/providers/report_providers.dart';
import 'package:labuda/domains/system/report/presentation/providers/report/report_state.dart';
import 'package:labuda/shared/widgets/empty_state.dart';
import 'package:labuda/shared/widgets/loading_indicator.dart';
import 'package:labuda/shared/widgets/page_error_state.dart';

/// My Reports Screen
class MyReportsScreen extends ConsumerStatefulWidget {
  const MyReportsScreen({super.key});

  @override
  ConsumerState<MyReportsScreen> createState() => _MyReportsScreenState();
}

class _MyReportsScreenState extends ConsumerState<MyReportsScreen> {
  ReportDisplayState? _selectedStatus;

  @override
  void initState() {
    super.initState();
    // THE single initial-load trigger for this surface. The notifier
    // deliberately does not fetch in build(), so this microtask is the
    // only request on mount — never two.
    Future.microtask(() => _loadReports());
  }

  /// Canonical initial load, also the retry for a first-load failure.
  Future<void> _loadReports() async {
    await ref.read(reportListNotifierProvider.notifier).loadReports();
  }

  /// Canonical refresh: existing reports stay visible; failure renders the
  /// inline banner with this same operation as retry.
  Future<void> _refresh() async {
    await ref.read(reportListNotifierProvider.notifier).refresh();
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(reportListNotifierProvider);

    return Scaffold(
      appBar: AppBar(
        title: const Text('My Reports'),
        actions: [
          PopupMenuButton<ReportDisplayState?>(
            icon: const Icon(Icons.filter_list),
            tooltip: 'Filter by status',
            onSelected: (status) {
              setState(() => _selectedStatus = status);
            },
            itemBuilder: (context) => [
              const PopupMenuItem(value: null, child: Text('All Reports')),
              ...ReportDisplayState.values.map(
                (status) => PopupMenuItem(
                  value: status,
                  child: Text(status.displayName),
                ),
              ),
            ],
          ),
        ],
      ),
      // SAFE-AREA-27 — the ONE canonical bottom system-window authority on
      // this bar-less screen: the body SafeArea consumes the live system
      // inset (the CustomScrollView never auto-absorbs it), so the scroll
      // viewport ends exactly at the system-region start at every inset.
      body: SafeArea(child: _buildBody(state)),
    );
  }

  Widget _buildBody(ReportListState state) {
    // LOADING FOUNDATION (owner-locked):
    // - No reports yet → first-load states only: LoadingIndicator,
    //   PageErrorState, or EmptyState.
    // - Reports present → they stay visible during refresh/retry; the update
    //   indicator and refresh failure render inline, never as full-page
    //   loading/error.
    return RefreshIndicator(
      onRefresh: _refresh,
      child: CustomScrollView(
        physics: const AlwaysScrollableScrollPhysics(),
        slivers: [
          if (state.isLoading && state.reports.isEmpty)
            // First request with no data → LoadingIndicator. Never
            // EmptyState (not yet loaded) and never a raw spinner.
            const SliverFillRemaining(
              hasScrollBody: false,
              child: Center(child: LoadingIndicator()),
            )
          else if (state.error != null && state.reports.isEmpty)
            // CANONICAL page-level load error (PageErrorState): safe
            // localized copy only — the raw state error never reaches the
            // screen. Retry re-executes the canonical initial load.
            SliverFillRemaining(
              hasScrollBody: false,
              child: PageErrorState(onRetry: _loadReports),
            )
          else
            ..._buildCollectionSlivers(state),
        ],
      ),
    );
  }

  /// Collection branch: runs only when a settled collection exists
  /// (possibly preserved across a failed refresh/retry). Applies the local
  /// status filter, then renders EmptyState (zero-result success) or the
  /// rows with the inline refresh indicator / refresh-error banner on top.
  List<Widget> _buildCollectionSlivers(ReportListState state) {
    final filteredReports = _selectedStatus == null
        ? state.reports
        : state.reports
              .where((r) => r.displayState == _selectedStatus)
              .toList();

    if (filteredReports.isEmpty) {
      return [
        SliverFillRemaining(hasScrollBody: false, child: _buildEmptyView()),
      ];
    }

    return [
      // Refresh/retry with existing data: rows stay, update indication on top.
      if (state.isLoading)
        const SliverToBoxAdapter(child: LinearProgressIndicator(minHeight: 2)),
      // Refresh/retry failure: rows stay, inline banner with retry that
      // re-executes the canonical refresh. Never a full-page error here.
      if (state.error != null)
        SliverToBoxAdapter(child: _buildRefreshErrorBanner()),
      SliverPadding(
        padding: const EdgeInsets.all(AppMetrics.p16),
        sliver: SliverList(
          delegate: SliverChildBuilderDelegate((context, index) {
            return Padding(
              padding: EdgeInsets.only(
                bottom: index == filteredReports.length - 1
                    ? 0
                    : AppMetrics.p12,
              ),
              child: ReportCard(report: filteredReports[index]),
            );
          }, childCount: filteredReports.length),
        ),
      ),
    ];
  }

  /// Minimum bounded refresh-failure indication: persistent inline banner
  /// with safe localized copy ([pageErrorMessage]) and a retry action that
  /// re-executes the canonical refresh. Not a new foundation — composition
  /// of canonical tokens for this screen, matching the established refresh
  /// banners.
  Widget _buildRefreshErrorBanner() {
    final scheme = Theme.of(context).colorScheme;
    final l10n = context.l10n;
    return Semantics(
      container: true,
      liveRegion: true,
      child: Container(
        width: double.infinity,
        margin: const EdgeInsets.fromLTRB(
          AppMetrics.p16,
          AppMetrics.p12,
          AppMetrics.p16,
          AppMetrics.p4,
        ),
        padding: const EdgeInsets.symmetric(
          horizontal: AppMetrics.p12,
          vertical: AppMetrics.p8,
        ),
        decoration: BoxDecoration(
          color: scheme.errorContainer,
          borderRadius: BorderRadius.circular(AppShape.r12),
          border: Border.all(color: scheme.error),
        ),
        child: Row(
          children: [
            Icon(
              Icons.refresh_outlined,
              size: AppIconSize.action,
              color: scheme.onErrorContainer,
            ),
            const SizedBox(width: AppMetrics.p8),
            Expanded(
              child: Text(
                l10n.pageErrorMessage,
                style: context.typeRoles.bodyDense.copyWith(
                  color: scheme.onErrorContainer,
                ),
              ),
            ),
            TextButton(onPressed: _refresh, child: Text(l10n.retryAction)),
          ],
        ),
      ),
    );
  }

  Widget _buildEmptyView() {
    // Filter empty: reports exist, the active status filter matched none →
    // one primary action that clears that filter.
    if (_selectedStatus != null) {
      return EmptyState(
        icon: Icons.filter_alt_off_outlined,
        title: context.l10n.emptySearchTitle,
        subtitle: context.l10n.emptySearchMessage,
        actionLabel: context.l10n.resetFilterAction,
        onAction: () => setState(() => _selectedStatus = null),
      );
    }

    // Collection empty: this account never submitted a report.
    return EmptyState(
      icon: Icons.outbox_outlined,
      title: context.l10n.emptyReportsTitle,
      subtitle: context.l10n.emptyReportsMessage,
    );
  }
}

/// Report Card Widget
class ReportCard extends StatelessWidget {
  final Report report;

  const ReportCard({super.key, required this.report});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(AppMetrics.p16),
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.surface,
        borderRadius: BorderRadius.circular(AppShape.r12),
        border: Border.all(color: Theme.of(context).colorScheme.outlineVariant),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Row(
                children: [
                  Icon(
                    _getIconForTargetType(report.subjectType),
                    size: AppIconSize.inlineGlyph,
                    color: Theme.of(context).colorScheme.onSurfaceVariant,
                  ),
                  const SizedBox(width: 6),
                  Text(
                    report.subjectType.displayName,
                    style: context.typeRoles.bodyDense.copyWith(
                      color: Theme.of(context).colorScheme.onSurfaceVariant,
                      fontWeight: FontWeight.w500,
                    ),
                  ),
                ],
              ),
              _buildStatusChip(context, report.displayState),
            ],
          ),
          const SizedBox(height: 12),
          Text(
            report.targetTitle,
            style: context.typeRoles.titleCompact.copyWith(
              fontWeight: FontWeight.w600,
              color: Theme.of(context).colorScheme.onSurface,
            ),
            maxLines: 2,
            overflow: TextOverflow.ellipsis,
          ),
          const SizedBox(height: 8),
          Row(
            children: [
              Icon(
                Icons.flag_outlined,
                size: AppIconSize.inlineGlyph,
                color: Theme.of(context).colorScheme.primary,
              ),
              const SizedBox(width: 4),
              Expanded(
                child: Text(
                  report.reason.displayName,
                  style: context.typeRoles.bodyDense.copyWith(
                    color: Theme.of(context).colorScheme.primary,
                    fontWeight: FontWeight.w500,
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: 8),
          if (report.description != null && report.description!.isNotEmpty) ...[
            Text(
              report.description!,
              style: context.typeRoles.bodyDense.copyWith(
                color: Theme.of(context).colorScheme.onSurfaceVariant,
              ),
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
            ),
            const SizedBox(height: 8),
          ],
          Text(
            const TimeFormatService().formatTimeAgo(report.createdAt),
            style: context.typeRoles.labelMicro.copyWith(
              color: Theme.of(context).colorScheme.onSurfaceVariant,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildStatusChip(BuildContext context, ReportDisplayState state) {
    Color bgColor;
    Color textColor;
    IconData icon;

    switch (state) {
      case ReportDisplayState.submitted:
        bgColor = context.statusColors.warning.withValues(alpha: 0.15);
        textColor = context.statusColors.warning;
        icon = Icons.schedule;
        break;
      case ReportDisplayState.underReview:
        bgColor = Theme.of(
          context,
        ).colorScheme.secondary.withValues(alpha: 0.15);
        textColor = Theme.of(context).colorScheme.secondary;
        icon = Icons.search;
        break;
      case ReportDisplayState.reviewedNoViolation:
        bgColor = Theme.of(
          context,
        ).colorScheme.outlineVariant.withValues(alpha: 0.15);
        textColor = Theme.of(context).colorScheme.onSurfaceVariant;
        icon = Icons.check_circle_outline;
        break;
      case ReportDisplayState.reviewedViolation:
        bgColor = context.statusColors.success.withValues(alpha: 0.15);
        textColor = context.statusColors.success;
        icon = Icons.done_all;
        break;
    }

    return Container(
      padding: const EdgeInsets.symmetric(
        horizontal: AppMetrics.p8,
        vertical: AppMetrics.p4,
      ),
      decoration: BoxDecoration(
        color: bgColor,
        borderRadius: BorderRadius.circular(AppShape.r12),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: AppIconSize.inlineGlyph, color: textColor),
          const SizedBox(width: 4),
          Text(
            state.displayName,
            style: context.typeRoles.labelMicro.copyWith(
              color: textColor,
              fontWeight: FontWeight.w600,
            ),
          ),
        ],
      ),
    );
  }

  IconData _getIconForTargetType(ReportTargetType type) {
    switch (type) {
      case ReportTargetType.content:
        return Icons.article_outlined;
      case ReportTargetType.comment:
        return Icons.comment_outlined;
      case ReportTargetType.user:
        return Icons.person_outlined;
      case ReportTargetType.forSale:
        return Icons.shopping_bag_outlined;
      case ReportTargetType.auction:
        return Icons.gavel_outlined;
    }
  }
}
