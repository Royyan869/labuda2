import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_state.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_providers.dart';
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

  Future<void> _loadChats({bool isRefresh = false}) async {
    try {
      final userId = ref.read(currentUserIdProvider);
      if (userId.isEmpty) {
        // User ID not available yet - will retry when auth state changes
        return;
      }
      await ref
          .read(chatListProvider.notifier)
          .loadChats(userId, isRefresh: isRefresh);
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
                padding: const EdgeInsets.symmetric(
                  horizontal: AppMetrics.p8,
                  vertical: AppMetrics.p4,
                ),
                decoration: BoxDecoration(
                  color: Theme.of(context).colorScheme.error,
                  borderRadius: BorderRadius.circular(AppShape.r12),
                ),
                child: Text(
                  totalUnread > 99 ? '99+' : totalUnread.toString(),
                  style: context.typeRoles.labelMicro.copyWith(
                    color: Theme.of(context).colorScheme.onPrimary,
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
                        decoration: AppTheme.searchDecoration(
                          Theme.of(context).colorScheme,
                          hintText: 'Search chats...',
                        ),
                        onChanged: _onSearchChanged,
                      ),
                    ),
                    if (_searchQuery.isNotEmpty)
                      IconButton(
                        icon: const Icon(Icons.close, semanticLabel: 'Tutup'),
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
    // LOADING FOUNDATION (owner-locked):
    // - No chats yet → first-load states only: loading, PageErrorState, or
    //   EmptyState.
    // - Chats present → they stay visible during refresh; update progress and
    //   refresh failure render inline, never as full-page loading/error.
    if (state.chats.isEmpty) {
      if (state.isLoading || state.isRefreshing) {
        return const Center(child: LoadingIndicator());
      }
      if (state.error != null) {
        return _buildErrorView();
      }
      return _buildEmptyView(true);
    }

    final filteredChats = _filterChats(state.chats);

    if (filteredChats.isEmpty) {
      return _buildEmptyView(false);
    }

    return RefreshIndicator(
      onRefresh: () => _refreshChats(),
      child: ListView.builder(
        itemCount:
            filteredChats.length +
            (state.isRefreshing || state.refreshError != null ? 1 : 0),
        itemBuilder: (context, index) {
          if (index == 0 &&
              (state.isRefreshing || state.refreshError != null)) {
            return Column(
              children: [
                if (state.isRefreshing)
                  const LinearProgressIndicator(minHeight: 2),
                if (state.refreshError != null) _buildRefreshErrorBanner(),
              ],
            );
          }
          final offset = (state.isRefreshing || state.refreshError != null)
              ? 1
              : 0;
          final chat = filteredChats[index - offset];
          return ChatCard(
            chat: chat,
            onTap: () => _openChatDetail(chat),
            onLongPress: () => _showChatOptions(chat),
          );
        },
      ),
    );
  }

  Future<void> _refreshChats() async {
    try {
      final userId = ref.read(currentUserIdProvider);
      if (userId.isEmpty) {
        throw Exception('User ID tidak tersedia');
      }
      await ref
          .read(chatListProvider.notifier)
          .loadChats(userId, isRefresh: true);
    } catch (e) {
      // RefreshIndicator will handle the error by showing the error state
      // Re-throw to let RefreshIndicator show the failure
      rethrow;
    }
  }

  /// Minimum bounded refresh-failure indication: persistent inline banner
  /// with safe localized copy ([pageErrorMessage]) and a retry action.
  /// Not a new foundation — composition of canonical tokens for this screen.
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
            TextButton(
              onPressed: () {
                ref.read(chatListProvider.notifier).clearRefreshError();
                _loadChats(isRefresh: true);
              },
              child: Text(l10n.retryAction),
            ),
          ],
        ),
      ),
    );
  }

  /// CANONICAL page-level load error (PageErrorState). The raw chat
  /// state.error never reaches the screen — safe localized copy only.
  Widget _buildErrorView() {
    return PageErrorState(onRetry: _loadChats);
  }

  Widget _buildEmptyView(bool hasNoChats) {
    final l10n = context.l10n;

    // First-use empty: this account has no chat yet → ONE primary action
    // (start one). The FAB stays the persistent screen-level affordance.
    if (hasNoChats) {
      return EmptyState(
        icon: Icons.message_outlined,
        title: l10n.emptyChatsTitle,
        subtitle: l10n.emptyChatsMessage,
        actionLabel: l10n.startChatAction,
        onAction: () => _showNewChatDialog(context),
      );
    }

    // Search empty: chats exist, the active query matched none → reset the
    // query instead of pretending the inbox is empty.
    return EmptyState(
      icon: Icons.search_off,
      title: l10n.emptyChatSearchTitle,
      subtitle: l10n.emptySearchMessage,
      actionLabel: l10n.resetFilterAction,
      onAction: () {
        _searchController.clear();
        _onSearchChanged('');
      },
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
        'Verifikasi email Anda untuk memulai percakapan baru.',
      );
      return;
    }

    // Navigate to new chat screen
    context.push(RoutePaths.newChat);
  }

  void _showChatOptions(Chat chat) {
    AppBottomSheetActions.showActions<void>(
      context: context,
      actions: [
        BottomSheetAction<void>(
          title: 'Delete chat',
          icon: Icons.delete_outline,
          style: BottomSheetActionStyle.destructive,
          onPressed: () {
            // Dismiss the action sheet, then run the destructive confirmation.
            Navigator.of(context).pop();
            _handleDeleteChat(chat);
          },
        ),
      ],
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
    final confirmed = await AppDialog.confirm(
      context: context,
      title: 'Delete chat?',
      message:
          'This will delete the chat for you. The other person will still be able to see it.',
      confirmLabel: 'Delete',
      cancelLabel: 'Cancel',
      intent: AppDialogIntent.destructive,
    );

    if (!confirmed) return;

    try {
      ref.read(chatListProvider.notifier).removeChat(chat.id);
      if (mounted) {
        AppSnackBar.showSuccess(context, 'Chat dihapus');
      }
    } catch (e) {
      if (mounted) {
        AppSnackBar.showError(
          context,
          'Gagal menghapus chat. Silakan coba lagi.',
        );
      }
    }
  }
}
