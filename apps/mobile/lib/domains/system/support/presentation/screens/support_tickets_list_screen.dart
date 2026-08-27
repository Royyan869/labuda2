library;

/// Support Tickets List Screen
///
/// Lists all user support tickets
/// Shows: category, status, last updated

import 'package:flutter/material.dart' hide ConnectionState;
import 'package:flutter/material.dart' as flutter show ConnectionState;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/chat/chat/data/chat_providers.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart'
    as chat_ent;
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
  @override
  Widget build(BuildContext context) {
    final currentUser = ref.watch(authenticatedUserProvider);

    if (currentUser == null) {
      return Scaffold(
        appBar: AppBarCustom(title: 'My Support Tickets'),
        body: const Center(
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Icon(
                Icons.lock_outline,
                size: 48,
                color: AppColors.neutralGray400,
              ),
              SizedBox(height: 16),
              Text(
                'Please login to view your support tickets',
                style: TextStyle(color: AppColors.neutralGray600),
              ),
            ],
          ),
        ),
      );
    }

    return Scaffold(
      appBar: AppBarCustom(title: 'My Support Tickets'),
      body: _buildTicketsList(currentUser.id),
      floatingActionButton: FloatingActionButton.extended(
        onPressed: () => _showCreateTicketSheet(currentUser),
        backgroundColor: AppColors.primaryRed,
        icon: const Icon(Icons.add, color: AppColors.neutralWhite),
        label: const Text(
          'New Ticket',
          style: TextStyle(color: AppColors.neutralWhite),
        ),
      ),
    );
  }

  Widget _buildTicketsList(String userId) {
    final chatRepository = ref.watch(chatRepositoryProvider);

    return FutureBuilder(
      future: chatRepository.getUserChats(userId: userId, limit: 50),
      builder: (context, snapshot) {
        if (snapshot.connectionState == flutter.ConnectionState.waiting) {
          return const Center(child: CircularProgressIndicator());
        }

        if (snapshot.hasError) {
          return Center(
            child: Column(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                const Icon(
                  Icons.error_outline,
                  size: 48,
                  color: AppColors.error,
                ),
                const SizedBox(height: 16),
                Text(
                  'Failed to load tickets',
                  style: const TextStyle(color: AppColors.error),
                ),
                const SizedBox(height: 16),
                ElevatedButton(
                  onPressed: () => setState(() {}),
                  child: const Text('Retry'),
                ),
              ],
            ),
          );
        }

        if (!snapshot.hasData) {
          return const Center(child: CircularProgressIndicator());
        }

        final result = snapshot.data!;

        // Handle Result type from chat repository using fold
        final chats = result.fold((error) => <chat_ent.Chat>[], (data) => data);

        // Filter only support chats (type == ChatType.support)
        final supportChats = chats
            .where((chat) => chat.type == chat_ent.ChatType.support)
            .toList();

        if (supportChats.isEmpty) {
          return Center(
            child: Column(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                const Icon(
                  Icons.mail_outline,
                  size: 64,
                  color: AppColors.neutralGray400,
                ),
                const SizedBox(height: 16),
                const Text(
                  'No support tickets yet',
                  style: TextStyle(
                    fontSize: 18,
                    fontWeight: FontWeight.bold,
                    color: AppColors.neutralGray600,
                  ),
                ),
                const SizedBox(height: 8),
                const Text(
                  'Create a ticket to get help from our support team',
                  textAlign: TextAlign.center,
                  style: TextStyle(
                    fontSize: 14,
                    color: AppColors.neutralGray600,
                  ),
                ),
                const SizedBox(height: 24),
                ElevatedButton.icon(
                  onPressed: () =>
                      _showCreateTicketSheet(ref.read(authenticatedUserProvider)!),
                  icon: const Icon(Icons.add),
                  label: const Text('Create Ticket'),
                  style: ElevatedButton.styleFrom(
                    backgroundColor: AppColors.primaryRed,
                    foregroundColor: AppColors.neutralWhite,
                  ),
                ),
              ],
            ),
          );
        }

        return RefreshIndicator(
          onRefresh: () async => setState(() {}),
          child: ListView.builder(
            padding: const EdgeInsets.all(16),
            itemCount: supportChats.length,
            itemBuilder: (context, index) {
              final chat = supportChats[index];
              return _SupportTicketListItem(
                chat: chat,
                onTap: () => _navigateToTicket(chat.id),
              );
            },
          ),
        );
      },
    );
  }

  void _navigateToTicket(String ticketId) {
    Navigator.of(context).push(
      MaterialPageRoute(
        builder: (context) => SupportTicketThreadScreen(ticketId: ticketId),
      ),
    );
  }

  void _showCreateTicketSheet(dynamic user) {
    showPreChatForm(
      context,
      userId: user.id,
      userName: user.name,
      userAvatar: user.avatar,
    );
  }
}

/// Support Ticket List Item Widget
class _SupportTicketListItem extends StatelessWidget {
  final chat_ent.Chat chat;
  final VoidCallback onTap;

  const _SupportTicketListItem({required this.chat, required this.onTap});

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    // Extract support category and status from chat fields
    final category = chat.supportCategory != null
        ? SupportCategory.values.byName(chat.supportCategory!.name)
        : SupportCategory.general;
    final status = chat.supportStatus != null
        ? SupportStatus.values.byName(chat.supportStatus!.name)
        : SupportStatus.open;
    final categoryConfig = CategoryConfig.get(category);
    final statusConfig = StatusConfig.get(status);

    final lastMessageTime = chat.updatedAt != null
        ? SupportUtils.formatTimeAgo(chat.updatedAt!)
        : SupportUtils.formatTimeAgo(chat.createdAt);

    return Card(
      margin: const EdgeInsets.only(bottom: 12),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(12),
        child: Padding(
          padding: const EdgeInsets.all(16),
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
                    lastMessageTime,
                    style: TextStyle(
                      fontSize: 11,
                      color: isDark
                          ? AppColors.neutralGray500
                          : AppColors.neutralGray600,
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 12),

              // Last message preview
              if (chat.lastMessage != null) ...[
                Text(
                  chat.lastMessage!.content,
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(
                    fontSize: 14,
                    color: isDark
                        ? AppColors.neutralGray300
                        : AppColors.neutralGray700,
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
                  padding: const EdgeInsets.symmetric(vertical: 8),
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(8),
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
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: BoxDecoration(
        color: Color(colorValue).withAlpha(40),
        borderRadius: BorderRadius.circular(6),
        border: Border.all(color: Color(colorValue).withAlpha(128)),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(icon, style: const TextStyle(fontSize: 11)),
          const SizedBox(width: 4),
          Text(
            label,
            style: TextStyle(
              fontSize: 10,
              fontWeight: FontWeight.bold,
              color: Color(colorValue),
            ),
          ),
        ],
      ),
    );
  }
}
