import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_state.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_providers.dart';
import 'package:labuda/domains/chat/chat/presentation/utils/chat_identity_display.dart';
import 'package:labuda/domains/chat/chat/presentation/widgets/chat_card.dart';
import 'package:labuda/shared/shared.dart';

/// Chat List Screen
///
/// Displays list of all user's chats with search and filter functionality.
class ChatListScreen extends ConsumerStatefulWidget {
  const ChatListScreen({super.key});

  @override
  ConsumerState<ChatListScreen> createState() => _ChatListScreenState();
}

class _ChatListScreenState extends ConsumerState<ChatListScreen> {
  final TextEditingController _searchController = TextEditingController();
  String _searchQuery = '';

  @override
  void initState() {
    super.initState();
    // Load chats on init
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _loadChats();
    });
  }

  Future<void> _loadChats() async {
    try {
      final userId = ref.read(currentUserIdProvider);
      if (userId.isEmpty) {
        // User ID not available yet - will retry when auth state changes
        return;
      }
      await ref.read(chatListProvider.notifier).loadChats(userId);
    } catch (e) {
      // Error will be reflected in state through the notifier
      // State handles the error and shows error UI
    }
  }

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  void _onSearchChanged(String query) {
    setState(() {
      _searchQuery = query.toLowerCase();
    });
  }

  List<Chat> _filterChats(List<Chat> chats) {
    if (_searchQuery.isEmpty) return chats;

    return chats.where((chat) {
      // Search by participant names
      return chat.participantNames.values.any(
        (name) => name.toLowerCase().contains(_searchQuery),
      );
    }).toList();
  }

  @override
  Widget build(BuildContext context) {
    final chatListState = ref.watch(chatListProvider);
    final totalUnread = ref.watch(totalUnreadCountProvider);

    return Scaffold(
      appBar: _buildAppBar(context, totalUnread),
      body: _buildBody(chatListState),
      floatingActionButton: _buildNewChatButton(context),
    );
  }

  PreferredSizeWidget _buildAppBar(BuildContext context, int totalUnread) {
    return AppBar(
      title: const Text('Messages'),
      actions: [
        // Unread count badge
        if (totalUnread > 0)
          Center(
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p16),
              child: Container(
                padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p8, vertical: AppMetrics.p4),
                decoration: BoxDecoration(
                  color: Theme.of(context).colorScheme.error,
                  borderRadius: BorderRadius.circular(AppShape.r12),
                ),
                child: Text(
                  totalUnread > 99 ? '99+' : totalUnread.toString(),
                  style: TextStyle(
                    color: Theme.of(context).colorScheme.onPrimary,
                    fontSize: AppType.s12,
                    fontWeight: FontWeight.bold,
                  ),
                ),
              ),
            ),
          ),
        // Mark all as read
        IconButton(
          icon: const Icon(Icons.done_all),
          tooltip: 'Mark all as read',
          onPressed: totalUnread > 0 ? _handleMarkAllRead : null,
        ),
      ],
      bottom: _searchQuery.isEmpty || _searchController.text.isEmpty
          ? null
          : PreferredSize(
              preferredSize: const Size.fromHeight(kToolbarHeight),
              child: Padding(
                padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p16),
                child: Row(
                  children: [
                    Expanded(
                      child: TextField(
                        controller: _searchController,
                        autofocus: true,
                        decoration: const InputDecoration(
                          hintText: 'Search chats...',
                          border: InputBorder.none,
                        ),
                        onChanged: _onSearchChanged,
                      ),
                    ),
                    if (_searchQuery.isNotEmpty)
                      IconButton(
                        icon: const Icon(Icons.close),
                        onPressed: () {
                          _searchController.clear();
                          _onSearchChanged('');
                        },
                      ),
                  ],
                ),
              ),
            ),
    );
  }

  Widget _buildBody(ChatListState state) {
    if (state.isLoading && state.chats.isEmpty) {
      return const Center(child: CircularProgressIndicator());
    }

    if (state.error != null && state.chats.isEmpty) {
      return _buildErrorView(state.error!);
    }

    final filteredChats = _filterChats(state.chats);

    if (filteredChats.isEmpty) {
      return _buildEmptyView(state.chats.isEmpty);
    }

    return RefreshIndicator(
      onRefresh: () async {
        try {
          final userId = ref.read(currentUserIdProvider);
          if (userId.isEmpty) {
            throw Exception('User ID tidak tersedia');
          }
          await ref.read(chatListProvider.notifier).loadChats(userId);
        } catch (e) {
          // RefreshIndicator will handle the error by showing the error state
          // Re-throw to let RefreshIndicator show the failure
          rethrow;
        }
      },
      child: ListView.builder(
        itemCount: filteredChats.length,
        itemBuilder: (context, index) {
          final chat = filteredChats[index];
          return ChatCard(
            chat: chat,
            onTap: () => _openChatDetail(chat),
            onLongPress: () => _showChatOptions(chat),
          );
        },
      ),
    );
  }

  Widget _buildErrorView(String error) {
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(
            Icons.error_outline,
            size: AppIconSize.display,
            color: Theme.of(context).colorScheme.error,
          ),
          const SizedBox(height: 16),
          Text(
            'Failed to load chats',
            style: Theme.of(context).textTheme.titleLarge,
          ),
          const SizedBox(height: 8),
          Text(
            error,
            style: Theme.of(context).textTheme.bodyMedium,
            textAlign: TextAlign.center,
          ),
          const SizedBox(height: 16),
          ElevatedButton(onPressed: _loadChats, child: const Text('Retry')),
        ],
      ),
    );
  }

  Widget _buildEmptyView(bool hasNoChats) {

    return Center(
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p48),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            // Icon with background
            Container(
              padding: const EdgeInsets.all(AppMetrics.p16),
              decoration: BoxDecoration(
                color: Theme.of(
                  context,
                ).colorScheme.onSurfaceVariant.withValues(alpha: 0.1),
                shape: BoxShape.circle,
              ),
              child: Icon(
                hasNoChats ? Icons.message_outlined : Icons.search_off,
                size: AppIconSize.display,
                color: Theme.of(context).colorScheme.onSurfaceVariant,
              ),
            ),
            const SizedBox(height: 24),

            // Title
            Text(
              hasNoChats ? 'Belum Ada Pesan' : 'Tidak Ada Chat Ditemukan',
              style: TextStyle(
                fontSize: AppType.s20,
                fontWeight: FontWeight.w600,
                color: Theme.of(context).colorScheme.onSurface,
              ),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 12),

            // Subtitle with clearer guidance
            Text(
              hasNoChats
                  ? 'Hubungi penjual untuk menanyakan produk'
                  : 'Coba kata kunci pencarian lain',
              style: TextStyle(
                fontSize: AppType.s14,
                color: Theme.of(context).colorScheme.onSurfaceVariant,
              ),
              textAlign: TextAlign.center,
            ),

            // Action button for empty chats
            if (hasNoChats) ...[
              const SizedBox(height: 32),
              SizedBox(
                width: AppContentSize.actionWidth,
                child: FilledButton.icon(
                  icon: const Icon(Icons.chat_outlined, size: AppIconSize.action),
                  label: const Text('Mulai Chat'),
                  onPressed: () => _showNewChatDialog(context),
                  style: FilledButton.styleFrom(
                    padding: const EdgeInsets.symmetric(
                      vertical: AppMetrics.p16,
                      horizontal: AppMetrics.p24,
                    ),
                  ),
                ),
              ),
              const SizedBox(height: 16),
              Text(
                'atau jelajahi marketplace untuk menemukan penjual',
                style: TextStyle(
                  fontSize: AppType.s12,
                  color: Theme.of(context).colorScheme.onSurfaceVariant,
                ),
              ),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildNewChatButton(BuildContext context) {
    return FloatingActionButton(
      onPressed: () => _showNewChatDialog(context),
      child: const Icon(Icons.chat),
    );
  }

  void _openChatDetail(Chat chat) {
    // Navigate to chat detail screen using go_router for proper navigation
    context.push('/chat/${chat.id}');
  }

  void _showNewChatDialog(BuildContext context) {
    final isEmailVerified = ref.read(isEmailVerifiedProvider);

    if (!isEmailVerified) {
      AppSnackBar.showWarning(
        context,
        'Please verify your email to start new conversations.',
      );
      return;
    }

    // Navigate to new chat screen
    context.push(RoutePaths.newChat);
  }

  void _showChatOptions(Chat chat) {
    showModalBottomSheet(
      context: context,
      builder: (context) => SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            ListTile(
              leading: const Icon(Icons.delete_outline),
              title: const Text('Delete chat'),
              onTap: () {
                Navigator.pop(context);
                _handleDeleteChat(chat);
              },
            ),
            if (!chat.isSupportChat)
              ListTile(
                leading: const Icon(Icons.block),
                title: const Text('Block user'),
                onTap: () {
                  Navigator.pop(context);
                  _handleBlockUser(chat);
                },
              ),
          ],
        ),
      ),
    );
  }

  Future<void> _handleMarkAllRead() async {
    final userId = ref.read(currentUserIdProvider);

    if (userId.isEmpty) {
      if (mounted) {
        AppSnackBar.showError(context, 'User ID tidak tersedia');
      }
      return;
    }

    await ref.read(chatListProvider.notifier).markAllRead(userId);

    if (mounted) {
      AppSnackBar.showSuccess(context, 'Semua chat ditandai sudah dibaca');
    }
  }

  Future<void> _handleDeleteChat(Chat chat) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Delete chat?'),
        content: const Text(
          'This will delete the chat for you. The other person will still be able to see it.',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('Cancel'),
          ),
          TextButton(
            onPressed: () => Navigator.pop(context, true),
            child: const Text('Delete'),
          ),
        ],
      ),
    );

    if (confirmed == true) {
      try {
        ref.read(chatListProvider.notifier).removeChat(chat.id);
        if (mounted) {
          ScaffoldMessenger.of(
            context,
          ).showSnackBar(const SnackBar(content: Text('Chat dihapus')));
        }
      } catch (e) {
        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            const SnackBar(
              content: Text('Gagal menghapus chat. Silakan coba lagi.'),
            ),
          );
        }
      }
    }
  }

  Future<void> _handleBlockUser(Chat chat) async {
    final blockedUserHandle = formatChatHandle(
      chat.getOtherParticipantName(ref.read(currentUserIdProvider)),
    );
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Block user?'),
        content: Text('Are you sure you want to block $blockedUserHandle?'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('Cancel'),
          ),
          TextButton(
            onPressed: () => Navigator.pop(context, true),
            style: TextButton.styleFrom(
              foregroundColor: Theme.of(context).colorScheme.error,
            ),
            child: const Text('Block'),
          ),
        ],
      ),
    );

    if (confirmed == true) {
      // Block user feature requires backend API implementation
      // Currently shows info message until backend is ready
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('Block user feature is not available yet'),
          ),
        );
      }
    }
  }
}
