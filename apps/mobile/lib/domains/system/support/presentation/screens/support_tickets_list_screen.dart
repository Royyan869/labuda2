library;

/// Support Tickets List Screen
///
/// Lists the authenticated user's own support tickets.
/// Shows: category, status, subject, last updated.
///
/// The Support API is the identity authority for this list — the chat room
/// list is NOT used to discover Support tickets. Each row navigates to the
/// ticket's own conversation thread by ticket id.

import 'package:flutter/material.dart' hide ConnectionState;
import 'package:flutter/material.dart' as flutter show ConnectionState;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/system/support/domain/domain.dart';
import 'package:labuda/domains/system/support/presentation/presentation.dart';
import 'package:labuda/shared/shared.dart';

class SupportTicketsListScreen extends ConsumerStatefulWidget {
  const SupportTicketsListScreen({super.key});

  @override
  ConsumerState<SupportTicketsListScreen> createState() =>
      _SupportTicketsListScreenState();
}

class _SupportTicketsListScreenState
    extends ConsumerState<SupportTicketsListScreen> {
  late Future<SupportResult<List<SupportTicket>>> _ticketsFuture;

  @override
  void initState() {
    super.initState();
    _ticketsFuture = _loadTickets();
  }

  Future<SupportResult<List<SupportTicket>>> _loadTickets() {
    return ref.read(supportRepositoryProvider).getMyTickets();
  }

  void _reload() {
    setState(() {
      _ticketsFuture = _loadTickets();
    });
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
                size: 48,
                color: Theme.of(context).colorScheme.onSurfaceVariant,
              ),
              SizedBox(height: 16),
              Text(
                'Please login to view your support tickets',
                style: TextStyle(color: Theme.of(context).colorScheme.onSurfaceVariant),
              ),
            ],
          ),
        ),
      );
    }

    return Scaffold(
      appBar: AppBarCustom(title: 'My Support Tickets'),
      body: _buildTicketsList(),
      floatingActionButton: FloatingActionButton.extended(
        onPressed: () => _showCreateTicketSheet(currentUser),
        backgroundColor: Theme.of(context).colorScheme.primary,
        icon: Icon(Icons.add, color: Theme.of(context).colorScheme.onPrimary),
        label: Text(
          'New Ticket',
          style: TextStyle(color: Theme.of(context).colorScheme.onPrimary),
        ),
      ),
    );
  }

  Widget _buildTicketsList() {
    return FutureBuilder<SupportResult<List<SupportTicket>>>(
      future: _ticketsFuture,
      builder: (context, snapshot) {
        if (snapshot.connectionState == flutter.ConnectionState.waiting) {
          return const Center(child: CircularProgressIndicator());
        }

        final result = snapshot.data;
        if (snapshot.hasError || result == null || result.isFailure) {
          return Center(
            child: Column(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                Icon(
                  Icons.error_outline,
                  size: 48,
                  color: context.statusColors.error,
                ),
                const SizedBox(height: 16),
                Text(
                  result?.failure?.message ?? 'Failed to load tickets',
                  textAlign: TextAlign.center,
                  style: TextStyle(color: context.statusColors.error),
                ),
                const SizedBox(height: 16),
                ElevatedButton(
                  onPressed: _reload,
                  child: const Text('Retry'),
                ),
              ],
            ),
          );
        }

        final tickets = result.dataOrThrow;

        if (tickets.isEmpty) {
          return Center(
            child: Column(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [                Icon(
                Icons.mail_outline,
                size: 64,
                color: Theme.of(context).colorScheme.onSurfaceVariant,
                ),
                const SizedBox(height: 16),
                Text(
                  'No support tickets yet',
                  style: TextStyle(
                    fontSize: AppType.s18,
                    fontWeight: FontWeight.bold,
                    color: Theme.of(context).colorScheme.onSurfaceVariant,
                  ),
                ),
                const SizedBox(height: 8),
                Text(
                  'Create a ticket to get help from our support team',
                  textAlign: TextAlign.center,
                  style: TextStyle(
                    fontSize: AppType.s14,
                    color: Theme.of(context).colorScheme.onSurfaceVariant,
                  ),
                ),
                const SizedBox(height: 24),
                ElevatedButton.icon(
                  onPressed: () => _showCreateTicketSheet(
                    ref.read(authenticatedUserProvider)!,
                  ),
                  icon: const Icon(Icons.add),
                  label: const Text('Create Ticket'),
                  style: ElevatedButton.styleFrom(
                    backgroundColor: Theme.of(context).colorScheme.primary,
                    foregroundColor: Theme.of(context).colorScheme.onPrimary,
                  ),
                ),
              ],
            ),
          );
        }

        return RefreshIndicator(
          onRefresh: () async => _reload(),
          child: ListView.builder(
            padding: const EdgeInsets.all(AppMetrics.p16),
            itemCount: tickets.length,
            itemBuilder: (context, index) {
              final ticket = tickets[index];
              return _SupportTicketListItem(
                ticket: ticket,
                onTap: () => _navigateToTicket(ticket.id),
              );
            },
          ),
        );
      },
    );
  }

  /// Navigates by the Support ticket id — the conversation thread resolves the
  /// ticket's own chat room through the Support API.
  void _navigateToTicket(String ticketId) {
    Navigator.of(context).push(
      MaterialPageRoute(
        builder: (context) => SupportTicketThreadScreen(ticketId: ticketId),
      ),
    );
  }

  void _showCreateTicketSheet(dynamic user) {
    showPreChatFormRefactored(
      context,
      userId: user.id,
      userName: user.name,
      userAvatar: user.avatar,
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
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(AppShape.r12)),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(AppShape.r12),
        child: Padding(
          padding: const EdgeInsets.all(AppMetrics.p16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // Category and Status badges
              Row(
                children: [
                  _buildBadge(
                    icon: categoryConfig.icon,
                    label: categoryConfig.nameId,
                    colorValue: categoryConfig.colorValue,
                  ),
                  const SizedBox(width: 8),
                  _buildBadge(
                    icon: statusConfig.icon,
                    label: statusConfig.labelId,
                    colorValue: statusConfig.colorValue,
                  ),
                  const Spacer(),
                  Text(
                    SupportUtils.formatTimeAgo(lastActivity),
                    style: TextStyle(
                      fontSize: AppType.s11,
                      color: Theme.of(context).colorScheme.onSurfaceVariant,
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
                  style: TextStyle(
                    fontSize: AppType.s14,
                    color: Theme.of(context).colorScheme.onSurfaceVariant,
                  ),
                ),
                const SizedBox(height: 8),
              ],

              // View ticket button
              OutlinedButton.icon(
                onPressed: onTap,
                icon: const Icon(Icons.mail_outline, size: 16),
                label: const Text('View Ticket'),
                style: OutlinedButton.styleFrom(
                  padding: const EdgeInsets.symmetric(vertical: AppMetrics.p8),
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(AppShape.r8),
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildBadge({
    required String icon,
    required String label,
    required int colorValue,
  }) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p8, vertical: AppMetrics.p4),
      decoration: BoxDecoration(
        color: Color(colorValue).withAlpha(40),
        borderRadius: BorderRadius.circular(AppShape.r6),
        border: Border.all(color: Color(colorValue).withAlpha(128)),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(icon, style: const TextStyle(fontSize: AppType.s11)),
          const SizedBox(width: 4),
          Text(
            label,
            style: TextStyle(
              fontSize: AppType.s10,
              fontWeight: FontWeight.bold,
              color: Color(colorValue),
            ),
          ),
        ],
      ),
    );
  }
}
