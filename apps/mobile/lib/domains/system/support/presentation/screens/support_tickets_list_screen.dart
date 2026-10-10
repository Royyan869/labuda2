library;

/// Support Tickets List Screen
///
/// Lists the authenticated user's own support tickets.
/// Shows: category, status, subject, last updated.
///
/// The Support API is the identity authority for this list — the chat room
/// list is NOT used to discover Support tickets. Each row navigates to the
/// ticket's own conversation thread by ticket id.

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/system/shared/domain/services/time_format_service.dart';
import 'package:hishumi/domains/system/support/domain/domain.dart';
import 'package:hishumi/domains/system/support/presentation/presentation.dart';
import 'package:hishumi/shared/shared.dart';

class SupportTicketsListScreen extends ConsumerStatefulWidget {
  const SupportTicketsListScreen({super.key});

  @override
  ConsumerState<SupportTicketsListScreen> createState() =>
      _SupportTicketsListScreenState();
}

class _SupportTicketsListScreenState
    extends ConsumerState<SupportTicketsListScreen> {
  /// Canonical reload: initial-load retry, pull-to-refresh, and the
  /// post-create reload all re-run the ONE canonical list operation.
  /// Failure stays observable through the provider's AsyncValue — it is never
  /// rethrown or rendered raw.
  Future<void> _reload() async {
    try {
      // The list reaches the screen through the provider state, not through
      // this future — `.then((_) {})` adapts it to `Future<void>` while
      // failures still propagate to the `catch` below.
      await ref.refresh(supportTicketsProvider.future).then((_) {});
    } catch (_) {
      // No-op: the failure remains observable via async.hasError with the
      // last-known-good collection preserved in async.value.
    }
  }

  @override
  Widget build(BuildContext context) {
    final currentUser = ref.watch(authenticatedUserProvider);

    if (currentUser == null) {
      return Scaffold(
        appBar: AppBarCustom(title: 'My Support Tickets'),
        body: Center(
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Icon(
                Icons.lock_outline,
                size: AppIconSize.display,
                color: Theme.of(context).colorScheme.onSurfaceVariant,
              ),
              SizedBox(height: 16),
              Text(
                'Please login to view your support tickets',
                style: TextStyle(
                  color: Theme.of(context).colorScheme.onSurfaceVariant,
                ),
              ),
            ],
          ),
        ),
      );
    }

    return Scaffold(
      appBar: AppBarCustom(title: 'My Support Tickets'),
      // SAFE-AREA-30 — the ONE canonical bottom system-window authority
      // for BODY content on this standalone route: the body SafeArea
      // wraps the ticket list so every branch ends at the system-region
      // start at every inset. The FAB keeps its SEPARATE Scaffold
      // endFloat lift (measured independently by the geometry contract).
      body: SafeArea(child: _buildTicketsBody(context)),
      floatingActionButton: FloatingActionButton.extended(
        onPressed: _showCreateTicketSheet,
        backgroundColor: Theme.of(context).colorScheme.primary,
        icon: Icon(Icons.add, color: Theme.of(context).colorScheme.onPrimary),
        label: Text(
          'New Ticket',
          style: TextStyle(color: Theme.of(context).colorScheme.onPrimary),
        ),
      ),
    );
  }

  /// Canonical page-state composition over the ONE ticket-list authority.
  ///
  /// - No data yet → first-load states only: LoadingIndicator, PageErrorState,
  ///   or EmptyState.
  /// - Tickets present → they stay visible during refresh; the update
  ///   indicator and refresh failure render inline, never as full-page
  ///   loading/error and never as an empty list.
  Widget _buildTicketsBody(BuildContext context) {
    final ticketsAsync = ref.watch(supportTicketsProvider);
    final tickets = ticketsAsync.value ?? const <SupportTicket>[];

    return RefreshIndicator(
      onRefresh: _reload,
      child: CustomScrollView(
        physics: const AlwaysScrollableScrollPhysics(),
        slivers: [
          if (ticketsAsync.isLoading && tickets.isEmpty)
            // First request with no data → LoadingIndicator. Never EmptyState
            // (not yet loaded) and never a raw spinner.
            const SliverFillRemaining(
              hasScrollBody: false,
              child: Center(child: LoadingIndicator()),
            )
          else if (ticketsAsync.hasError && tickets.isEmpty)
            // CANONICAL page-level load error (PageErrorState): controlled
            // localized copy only — the raw backend error never reaches the
            // screen. Retry re-executes the canonical load.
            SliverFillRemaining(
              hasScrollBody: false,
              child: PageErrorState(onRetry: _reload),
            )
          else if (tickets.isEmpty)
            // Successful zero-result: the ONE canonical EmptyState. First-use
            // action preserved (at most one primary action).
            SliverFillRemaining(
              hasScrollBody: false,
              child: EmptyState(
                icon: Icons.mail_outline,
                title: context.l10n.emptySupportTicketsTitle,
                subtitle: context.l10n.emptySupportTicketsMessage,
                actionLabel: context.l10n.createTicketAction,
                onAction: _showCreateTicketSheet,
              ),
            )
          else ...[
            // Refresh with existing data: rows stay, update indication on top.
            if (ticketsAsync.isLoading)
              const SliverToBoxAdapter(
                child: LinearProgressIndicator(minHeight: 2),
              ),
            // Refresh failure: rows stay, inline banner with retry that
            // re-executes the canonical load. Never a full-page error here.
            if (ticketsAsync.hasError)
              SliverToBoxAdapter(child: _buildRefreshErrorBanner()),
            SliverPadding(
              padding: const EdgeInsets.all(AppMetrics.p16),
              sliver: SliverList(
                delegate: SliverChildBuilderDelegate((context, index) {
                  final ticket = tickets[index];
                  return _SupportTicketListItem(
                    ticket: ticket,
                    onTap: () => _navigateToTicket(ticket.id),
                  );
                }, childCount: tickets.length),
              ),
            ),
          ],
        ],
      ),
    );
  }

  /// Minimum bounded refresh-failure indication: persistent inline banner
  /// with safe localized copy and a retry action. Not a new foundation —
  /// composition of canonical tokens, matching the established banners.
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
            TextButton(onPressed: _reload, child: Text(l10n.retryAction)),
          ],
        ),
      ),
    );
  }

  /// Navigates by the Support ticket id — the conversation thread resolves the
  /// ticket's own chat room through the Support API.
  void _navigateToTicket(String ticketId) {
    context.push(RoutePaths.supportTicketThreadPath(ticketId));
  }

  void _showCreateTicketSheet() {
    final user = ref.read(authenticatedUserProvider);
    if (user == null) return;
    showPreChatFormRefactored(
      context,
      userId: user.id,
      userName: user.username,
      userAvatar: user.avatarUrl,
      onChatCreated: _reload,
    );
  }
}

/// Support Ticket List Item Widget
class _SupportTicketListItem extends StatelessWidget {
  final SupportTicket ticket;
  final VoidCallback onTap;

  const _SupportTicketListItem({required this.ticket, required this.onTap});

  @override
  Widget build(BuildContext context) {
    final categoryConfig = CategoryConfig.get(ticket.category);
    final statusConfig = StatusConfig.get(ticket.status);

    final lastActivity = ticket.updatedAt ?? ticket.createdAt;
    final preview = ticket.subject?.trim().isNotEmpty == true
        ? ticket.subject!.trim()
        : ticket.description;

    return Card(
      margin: const EdgeInsets.only(bottom: AppMetrics.p12),
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(AppShape.r12),
      ),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(AppShape.r12),
        child: Padding(
          padding: const EdgeInsets.all(AppMetrics.p16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // Category and Status badges + relative last-activity timestamp.
              // All dynamic text is flex-bounded with compact single-line
              // ellipsis so narrow width / high text scale cannot overflow.
              Row(
                children: [
                  Flexible(
                    child: _buildBadge(
                      context,
                      icon: categoryConfig.icon,
                      label: ticket.category.label(context.l10n),
                      colorValue: categoryConfig.colorValue,
                    ),
                  ),
                  const SizedBox(width: 8),
                  Flexible(
                    child: _buildBadge(
                      context,
                      icon: statusConfig.icon,
                      label: ticket.status.label(context.l10n),
                      colorValue: statusConfig.colorValue,
                    ),
                  ),
                  const SizedBox(width: 8),
                  Flexible(
                    child: Text(
                      const TimeFormatService().formatTimeAgo(lastActivity),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: context.typeRoles.labelMicro.copyWith(
                        color: Theme.of(context).colorScheme.onSurfaceVariant,
                      ),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 12),

              // Subject / description preview
              if (preview != null && preview.isNotEmpty) ...[
                Text(
                  preview,
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  style: context.typeRoles.bodyDense.copyWith(
                    color: Theme.of(context).colorScheme.onSurfaceVariant,
                  ),
                ),
                const SizedBox(height: 8),
              ],

              // View ticket button
              OutlinedButton.icon(
                onPressed: onTap,
                icon: const Icon(
                  Icons.mail_outline,
                  size: AppIconSize.inlineGlyph,
                ),
                label: const Text('View Ticket'),
                style: OutlinedButton.styleFrom(
                  padding: const EdgeInsets.symmetric(vertical: AppMetrics.p8),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildBadge(
    BuildContext context, {
    required String icon,
    required String label,
    required int colorValue,
  }) {
    return Container(
      padding: const EdgeInsets.symmetric(
        horizontal: AppMetrics.p8,
        vertical: AppMetrics.p4,
      ),
      decoration: BoxDecoration(
        color: Color(colorValue).withAlpha(40),
        borderRadius: BorderRadius.circular(AppShape.r6),
        border: Border.all(color: Color(colorValue).withAlpha(128)),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(icon, style: context.typeRoles.labelMicro),
          const SizedBox(width: 4),
          Flexible(
            child: Text(
              label,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: context.typeRoles.labelMicro.copyWith(
                fontWeight: FontWeight.bold,
                color: Color(colorValue),
              ),
            ),
          ),
        ],
      ),
    );
  }
}
