import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/chat/chat/data/dto/chat_resource_occurrence_request.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_providers.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/new_chat_user_search_provider.dart';
import 'package:labuda/domains/social/share/domain/entities/share_target.dart';
import 'package:labuda/features/search/search/domain/entities/user_search.dart'
    show UserSearch;
import 'package:labuda/shared/shared.dart';
import 'package:labuda/shared/widgets/composer_action_buttons.dart';
import 'package:labuda/shared/helpers/user_identity_formatter.dart';
import 'share_preview_card.dart';

@visibleForTesting
ChatResourceOccurrenceResourceType shareTargetToResourceType(
  ShareTarget target,
) {
  return ChatResourceOccurrenceResourceType.fromWire(
    target.type.wireTargetType,
  );
}

@visibleForTesting
ChatResourceOccurrenceRequest buildShareToChatRequest(ShareTarget target) {
  return ChatResourceOccurrenceRequest.shareToChat(
    resourceType: shareTargetToResourceType(target),
    resourceId: target.id,
  );
}

class ShareToChatDialog extends ConsumerStatefulWidget {
  final ShareTarget target;
  final TextEditingController? messageController;

  const ShareToChatDialog({
    super.key,
    required this.target,
    this.messageController,
  });

  static Future<String?> show({
    required BuildContext context,
    required ShareTarget target,
    TextEditingController? messageController,
  }) {
    return showModalBottomSheet<String>(
      context: context,
      isScrollControlled: true,
      useRootNavigator: true,
      backgroundColor: Colors.transparent,
      builder: (context) => ShareToChatDialog(
        target: target,
        messageController: messageController,
      ),
    );
  }

  @override
  ConsumerState<ShareToChatDialog> createState() => _ShareToChatDialogState();
}

class _ShareToChatDialogState extends ConsumerState<ShareToChatDialog> {
  late final TextEditingController _messageController;
  late final bool _ownsMessageController;
  String _searchQuery = '';
  UserSearch? _selectedRecipient;
  bool _isSending = false;

  @override
  void initState() {
    super.initState();
    _ownsMessageController = widget.messageController == null;
    _messageController = widget.messageController ?? TextEditingController();
  }

  @override
  void dispose() {
    if (_ownsMessageController) {
      _messageController.dispose();
    }
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final backgroundColor = scheme.surfaceContainerLow;
    final borderColor = scheme.outlineVariant;
    final dividerColor = scheme.outlineVariant;
    final textColor = scheme.onSurface;
    final searchAsync = ref.watch(newChatUserSearchProvider(_searchQuery));

    return SafeArea(
      child: Padding(
        padding: EdgeInsets.only(
          bottom: MediaQuery.of(context).viewInsets.bottom,
        ),
        child: Container(
          constraints: BoxConstraints(
            maxHeight: MediaQuery.of(context).size.height * 0.92,
          ),
          decoration: BoxDecoration(
            color: backgroundColor,
            borderRadius: const BorderRadius.vertical(top: Radius.circular(AppShape.r20)),
          ),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Container(
                margin: const EdgeInsets.only(top: AppMetrics.p12),
                width: 40,
                height: 4,
                decoration: BoxDecoration(
                  color: scheme.outlineVariant,
                  borderRadius: BorderRadius.circular(AppShape.r2),
                ),
              ),
              Padding(
                padding: const EdgeInsets.fromLTRB(AppMetrics.p20, AppMetrics.p16, AppMetrics.p20, AppMetrics.p12),
                child: Row(
                  children: [
                    Expanded(
                      child: Text(
                        'Send to Chat',
                        style: AppTypography.h5.copyWith(
                          fontWeight: FontWeight.w700,
                          color: textColor,
                        ),
                      ),
                    ),
                    IconButton(
                      onPressed: _isSending
                          ? null
                          : () => Navigator.pop(context),
                      icon: Icon(Icons.close, color: textColor),
                      padding: EdgeInsets.zero,
                      constraints: const BoxConstraints(),
                    ),
                  ],
                ),
              ),
              Divider(height: 1, color: dividerColor),
              Flexible(
                child: SingleChildScrollView(
                  padding: const EdgeInsets.fromLTRB(AppMetrics.p20, AppMetrics.p16, AppMetrics.p20, AppMetrics.p16),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      SharePreviewCard(target: widget.target),
                      const SizedBox(height: 16),
                      Text(
                        'Recipient',
                        style: AppTypography.bodyMedium.copyWith(
                          fontWeight: FontWeight.w600,
                          color: textColor,
                        ),
                      ),
                      const SizedBox(height: 8),
                      _buildSearchField(context, borderColor),
                      const SizedBox(height: 12),
                      if (_selectedRecipient != null) _buildSelectedRecipient(),
                      const SizedBox(height: 12),
                      _buildComposerField(context, textColor),
                      const SizedBox(height: 16),
                      Text(
                        'Pick one recipient. The message is sent only when you press Send.',
                        style: AppTypography.bodySmall.copyWith(
                          color: scheme.onSurfaceVariant,
                        ),
                      ),
                      const SizedBox(height: 12),
                      _buildSearchResults(searchAsync),
                    ],
                  ),
                ),
              ),
              // Send lives in the composer row (canonical action row) —
              // header X is the only dismissal control.
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildSearchField(BuildContext context, Color borderColor) {
    final scheme = Theme.of(context).colorScheme;
    return TextField(
      onChanged: (value) {
        setState(() {
          _searchQuery = value.trim();
        });
      },
      decoration: InputDecoration(
        hintText: 'Search name or username',
        prefixIcon: const Icon(Icons.search),
        filled: true,
        fillColor: scheme.surfaceContainerHigh,
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(AppShape.r12),
          borderSide: BorderSide(color: borderColor),
        ),
        enabledBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(AppShape.r12),
          borderSide: BorderSide(color: borderColor),
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(AppShape.r12),
          borderSide: BorderSide(color: scheme.primary, width: 2),
        ),
      ),
    );
  }

  Widget _buildSelectedRecipient() {
    final recipient = _selectedRecipient!;
    final label =
        UserIdentityFormatter.formatHandle(recipient.username) ??
        recipient.username;
    return InputChip(
      avatar: ProfileAvatar(
        userId: recipient.userId,
        size: 28,
        imageUrl: recipient.avatarUrl,
        showShadow: false,
      ),
      label: Text(label),
      onDeleted: _isSending
          ? null
          : () {
              setState(() {
                _selectedRecipient = null;
              });
            },
    );
  }

  Widget _buildComposerField(BuildContext context, Color textColor) {
    final scheme = Theme.of(context).colorScheme;
    // Canonical action row: [pill] [send]. The message stays optional —
    // sendability is gated by recipient selection, not by draft text.
    return Row(
      crossAxisAlignment: CrossAxisAlignment.end,
      children: [
        Expanded(
          child: TextField(
            controller: _messageController,
            maxLines: 4,
            minLines: 1,
            style: AppTypography.bodyMedium.copyWith(color: textColor),
            decoration: AppTheme.composerDecoration(
              scheme,
              hintText: 'Write a message (optional)',
            ),
          ),
        ),
        const SizedBox(width: 8),
        ComposerSendButton(
          key: const ValueKey('share-send-button'),
          loading: _isSending,
          onPressed: _selectedRecipient != null ? _handleSend : null,
        ),
      ],
    );
  }

  Widget _buildSearchResults(
    AsyncValue<List<UserSearch>> searchAsync,
  ) {
    final scheme = Theme.of(context).colorScheme;
    return searchAsync.when(
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (error, stackTrace) => Text(
        error.toString(),
        style: TextStyle(
          color: scheme.onSurfaceVariant,
        ),
      ),
      data: (users) {
        if (_searchQuery.isEmpty) {
          return Text(
            'Search for a recipient to continue.',
            style: TextStyle(
              color: scheme.onSurfaceVariant,
            ),
          );
        }

        if (users.isEmpty) {
          return Text(
            'No users found.',
            style: TextStyle(
              color: scheme.onSurfaceVariant,
            ),
          );
        }

        return ListView.separated(
          shrinkWrap: true,
          physics: const NeverScrollableScrollPhysics(),
          itemCount: users.length,
          separatorBuilder: (_, _) => const SizedBox(height: 4),
          itemBuilder: (context, index) {
            final user = users[index];
            return _ShareRecipientRow(
              user: user,
              onTap: () {
                setState(() {
                  _selectedRecipient = user;
                });
              },
            );
          },
        );
      },
    );
  }

  ShareReference _shareTargetToShareReference(ShareTarget target) {
    switch (target.type) {
      case ExternalShareType.post:
      case ExternalShareType.request:
        return ShareReference.content(
          contentId: target.id,
          title: target.title,
          imageUrl: target.imageUrl,
        );
      case ExternalShareType.forSale:
        return ShareReference.forSale(
          forSaleId: target.id,
          title: target.title,
          imageUrl: target.imageUrl,
        );
      case ExternalShareType.auction:
        return ShareReference.auction(
          auctionId: target.id,
          title: target.title,
          imageUrl: target.imageUrl,
        );
      case ExternalShareType.profile:
        return ShareReference.profile(
          profileId: target.id,
          name: target.title,
          avatarUrl: target.imageUrl,
        );
      // content is not a value of ExternalShareType but is a ShareTargetType.
      // ShareTarget currently only carries the 5 values above; this default
      // keeps the switch exhaustive if the enum grows.
    }
  }

  Future<void> _handleSend() async {
    final recipient = _selectedRecipient;
    if (recipient == null || _isSending) return;

    final authState = ref.read(authControllerProvider);
    final sender = ref.read(authenticatedUserProvider);
    final senderId = ref.read(currentUserIdProvider);
    if (authState is! AuthStateAuthenticated ||
        sender == null ||
        senderId.isEmpty) {
      if (mounted) {
        AppSnackBar.showWarning(context, 'Please log in to send to chat.');
      }
      return;
    }

    final content = _messageController.text.trim();
    final messageType = content.isEmpty ? MessageType.system : MessageType.text;

    setState(() {
      _isSending = true;
    });

    try {
      final chat = await ref
          .read(chatListProvider.notifier)
          .getOrCreateChat(userId: senderId, otherUserId: recipient.userId);

      if (!mounted) return;

      if (chat == null) {
        AppSnackBar.showError(context, 'Failed to open chat. Try again.');
        return;
      }

      final shareReference = _shareTargetToShareReference(widget.target);
      final result = await ref
          .read(chatDetailProvider(chat.id).notifier)
          .sendMessage(
            senderId: senderId,
            senderName: sender.username,
            content: content,
            type: messageType,
            objectReference: shareReference,
          );

      if (!mounted) return;

      if (result == null) {
        final chatState = ref.read(chatDetailProvider(chat.id));
        final errorMessage =
            chatState.error?.toLowerCase().contains('blocked') == true
            ? 'Tidak dapat mengirim chat share. Pengguna ini memblokir Anda.'
            : 'Failed to send chat share. Try again.';
        AppSnackBar.showError(context, errorMessage);
        return;
      }

      Navigator.pop(context, chat.id);
    } catch (e) {
      if (mounted) {
        AppSnackBar.showError(context, 'Failed to send chat share.');
      }
    } finally {
      if (mounted) {
        setState(() {
          _isSending = false;
        });
      }
    }
  }
}

class _ShareRecipientRow extends StatelessWidget {
  final UserSearch user;
  final VoidCallback onTap;

  const _ShareRecipientRow({
    required this.user,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final label = UserIdentityFormatter.formatHandle(user.username) ?? 'User';
    final labelColor = scheme.onSurface;
    final subtitleColor = scheme.onSurfaceVariant;
    return Material(
      color: Colors.transparent,
      child: ListTile(
        onTap: onTap,
        contentPadding: const EdgeInsets.symmetric(horizontal: AppMetrics.p12, vertical: AppMetrics.p4),
        leading: ProfileAvatar(
          userId: user.userId,
          size: 40,
          imageUrl: user.avatarUrl,
        ),
        title: Text(
          label,
          style: TextStyle(color: labelColor, fontWeight: FontWeight.w600),
        ),
        subtitle: Text(
          user.userId,
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
          style: TextStyle(color: subtitleColor),
        ),
        trailing: Icon(Icons.chevron_right, color: subtitleColor),
      ),
    );
  }
}
