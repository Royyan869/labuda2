/// My Reports Screen
///
/// PHASE 2: User-facing screen to view submitted reports and their status.
/// Provides minimal transparency into the moderation process.
///
/// Features:
/// - View all submitted reports
/// - See report status (pending, under review, resolved, etc.)
/// - Filter by status
/// - Pull to refresh
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/system/report/domain/entities/entities.dart';
import 'package:labuda/domains/system/report/presentation/providers/report_providers.dart';
import 'package:labuda/domains/system/report/presentation/providers/report/report_state.dart';

/// My Reports Screen
class MyReportsScreen extends ConsumerStatefulWidget {
  const MyReportsScreen({super.key});

  @override
  ConsumerState<MyReportsScreen> createState() => _MyReportsScreenState();
}

class _MyReportsScreenState extends ConsumerState<MyReportsScreen> {
  ReportStatus? _selectedStatus;

  @override
  void initState() {
    super.initState();
    // Load reports on init
    Future.microtask(() => _loadReports());
  }

  Future<void> _loadReports() async {
    await ref.read(reportListNotifierProvider.notifier).loadReports();
  }

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
          // Filter button
          PopupMenuButton<ReportStatus?>(
            icon: const Icon(Icons.filter_list),
            tooltip: 'Filter by status',
            onSelected: (status) {
              setState(() => _selectedStatus = status);
            },
            itemBuilder: (context) => [
              const PopupMenuItem(value: null, child: Text('All Reports')),
              ...ReportStatus.values.map(
                (status) => PopupMenuItem(
                  value: status,
                  child: Text(status.displayName),
                ),
              ),
            ],
          ),
        ],
      ),
      body: _buildBody(state),
    );
  }

  Widget _buildBody(ReportListState state) {
    if (state.isLoading && state.reports.isEmpty) {
      return const Center(child: CircularProgressIndicator());
    }

    if (state.error != null && state.reports.isEmpty) {
      return _buildErrorView(state.error!);
    }

    final filteredReports = _selectedStatus == null
        ? state.reports
        : state.reports.where((r) => r.status == _selectedStatus).toList();

    if (filteredReports.isEmpty) {
      return _buildEmptyView();
    }

    return RefreshIndicator(
      onRefresh: _refresh,
      child: ListView.separated(
        padding: const EdgeInsets.all(16),
        itemCount: filteredReports.length,
        separatorBuilder: (_, _) => const SizedBox(height: 12),
        itemBuilder: (context, index) {
          return ReportCard(report: filteredReports[index]);
        },
      ),
    );
  }

  Widget _buildErrorView(String error) {
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          const Icon(Icons.error_outline, size: 64, color: AppColors.error),
          const SizedBox(height: 16),
          Text(
            'Failed to load reports',
            style: Theme.of(context).textTheme.titleLarge,
          ),
          const SizedBox(height: 8),
          Text(
            error,
            style: Theme.of(context).textTheme.bodyMedium,
            textAlign: TextAlign.center,
          ),
          const SizedBox(height: 16),
          ElevatedButton(onPressed: _loadReports, child: const Text('Retry')),
        ],
      ),
    );
  }

  Widget _buildEmptyView() {
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(
            Icons.outbox_outlined,
            size: 64,
            color: AppColors.neutralGray400,
          ),
          const SizedBox(height: 16),
          Text(
            'No Reports Yet',
            style: Theme.of(
              context,
            ).textTheme.titleLarge?.copyWith(color: AppColors.neutralGray700),
          ),
          const SizedBox(height: 8),
          Text(
            _selectedStatus != null
                ? 'No reports with status "${_selectedStatus!.displayName}"'
                : 'You haven\'t submitted any reports yet.\nTap the report button on content to report it.',
            style: Theme.of(
              context,
            ).textTheme.bodyMedium?.copyWith(color: AppColors.neutralGray500),
            textAlign: TextAlign.center,
          ),
        ],
      ),
    );
  }
}

/// Report Card Widget
class ReportCard extends StatelessWidget {
  final Report report;

  const ReportCard({super.key, required this.report});

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray700 : AppColors.neutralWhite,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: isDark ? AppColors.darkGray600 : AppColors.neutralGray200,
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Header: Target type + Status
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Row(
                children: [
                  Icon(
                    _getIconForTargetType(report.subjectType),
                    size: 16,
                    color: AppColors.neutralGray500,
                  ),
                  const SizedBox(width: 6),
                  Text(
                    report.subjectType.displayName,
                    style: TextStyle(
                      fontSize: 13,
                      color: AppColors.neutralGray500,
                      fontWeight: FontWeight.w500,
                    ),
                  ),
                ],
              ),
              _buildStatusChip(context, report.status),
            ],
          ),
          const SizedBox(height: 12),

          // Target info
          if (report.targetTitle != null) ...[
            Text(
              report.targetTitle!,
              style: TextStyle(
                fontSize: 15,
                fontWeight: FontWeight.w600,
                color: isDark
                    ? AppColors.neutralWhite
                    : AppColors.neutralGray900,
              ),
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
            ),
            const SizedBox(height: 8),
          ],

          // Reason
          Row(
            children: [
              Icon(Icons.flag_outlined, size: 14, color: AppColors.primaryRed),
              const SizedBox(width: 4),
              Expanded(
                child: Text(
                  report.reason.displayName,
                  style: TextStyle(
                    fontSize: 13,
                    color: AppColors.primaryRed,
                    fontWeight: FontWeight.w500,
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: 8),

          // Description (if any)
          if (report.description != null && report.description!.isNotEmpty) ...[
            Text(
              report.description!,
              style: TextStyle(
                fontSize: 13,
                color: isDark
                    ? AppColors.neutralGray400
                    : AppColors.neutralGray600,
              ),
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
            ),
            const SizedBox(height: 8),
          ],

          // Footer: Date + Action info
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text(
                _formatDate(report.createdAt),
                style: TextStyle(fontSize: 12, color: AppColors.neutralGray400),
              ),
              if (report.action != ReportAction.none)
                Text(
                  report.action.displayName,
                  style: TextStyle(
                    fontSize: 12,
                    color: _getActionColor(report.action),
                    fontWeight: FontWeight.w500,
                  ),
                ),
            ],
          ),
        ],
      ),
    );
  }

  Widget _buildStatusChip(BuildContext context, ReportStatus status) {
    Color bgColor;
    Color textColor;
    IconData icon;

    switch (status) {
      case ReportStatus.pending:
        bgColor = AppColors.warning.withValues(alpha: 0.15);
        textColor = AppColors.warning;
        icon = Icons.schedule;
        break;
      case ReportStatus.underReview:
        bgColor = AppColors.primaryBlue.withValues(alpha: 0.15);
        textColor = AppColors.primaryBlue;
        icon = Icons.search;
        break;
      case ReportStatus.approved:
        bgColor = AppColors.successGreen.withValues(alpha: 0.15);
        textColor = AppColors.successGreen;
        icon = Icons.check_circle;
        break;
      case ReportStatus.rejected:
        bgColor = AppColors.neutralGray400.withValues(alpha: 0.15);
        textColor = AppColors.neutralGray500;
        icon = Icons.cancel;
        break;
      case ReportStatus.resolved:
        bgColor = AppColors.successGreen.withValues(alpha: 0.15);
        textColor = AppColors.successGreen;
        icon = Icons.done_all;
        break;
    }

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: BoxDecoration(
        color: bgColor,
        borderRadius: BorderRadius.circular(12),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 12, color: textColor),
          const SizedBox(width: 4),
          Text(
            status.displayName,
            style: TextStyle(
              fontSize: 11,
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

  Color _getActionColor(ReportAction action) {
    switch (action) {
      case ReportAction.none:
        return AppColors.neutralGray400;
      case ReportAction.warning:
        return AppColors.warning;
      case ReportAction.contentRemoved:
        return AppColors.error;
      case ReportAction.userSuspended:
      case ReportAction.userBanned:
        return AppColors.error;
      case ReportAction.dismissed:
        return AppColors.neutralGray400;
    }
  }

  String _formatDate(DateTime date) {
    final now = DateTime.now();
    final difference = now.difference(date);

    if (difference.inDays == 0) {
      if (difference.inHours == 0) {
        if (difference.inMinutes == 0) {
          return 'Just now';
        }
        return '${difference.inMinutes}m ago';
      }
      return '${difference.inHours}h ago';
    } else if (difference.inDays == 1) {
      return 'Yesterday';
    } else if (difference.inDays < 7) {
      return '${difference.inDays}d ago';
    } else {
      return '${date.day}/${date.month}/${date.year}';
    }
  }
}
