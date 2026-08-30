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
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final backgroundColor = isDark
        ? AppColors.darkGray800
        : AppColors.neutralWhite;
    final borderColor = isDark
        ? AppColors.darkGray600
        : AppColors.neutralGray200;
    final dividerColor = isDark
        ? AppColors.darkGray600
        : AppColors.neutralGray200;
    final textColor = isDark
        ? AppColors.neutralGray100
        : AppColors.neutralGray900;
    final searchAsync = ref.watch(newChatUserSearchProvider(_searchQuery));
    final canSend = !_isSending && _selectedRecipient != null;

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
            borderRadius: const BorderRadius.vertical(top: Radius.circular(20)),
          ),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Container(
                margin: const EdgeInsets.only(top: 12),
                width: 40,
                height: 4,
                decoration: BoxDecoration(
                  color: isDark
                      ? AppColors.darkGray500
                      : AppColors.neutralGray300,
                  borderRadius: BorderRadius.circular(2),
                ),
              ),
              Padding(
                padding: const EdgeInsets.fromLTRB(20, 16, 20, 12),
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
                  padding: const EdgeInsets.fromLTRB(20, 16, 20, 16),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      SharePreviewCard(target: widget.target, isDark: isDark),
                      const SizedBox(height: 16),
                      Text(
                        'Recipient',
                        style: AppTypography.bodyMedium.copyWith(
                          fontWeight: FontWeight.w600,
                          color: textColor,
                        ),
                      ),
                      const SizedBox(height: 8),
                      _buildSearchField(isDark, borderColor),
                      const SizedBox(height: 12),
                      if (_selectedRecipient != null) _buildSelectedRecipient(),
                      const SizedBox(height: 12),
                      _buildComposerField(isDark, borderColor, textColor),
                      const SizedBox(height: 16),
                      Text(
                        'Pick one recipient. The message is sent only when you press Send.',
                        style: AppTypography.bodySmall.copyWith(
                          color: isDark
                              ? AppColors.neutralGray400
                              : AppColors.neutralGray500,
                        ),
                      ),
                      const SizedBox(height: 12),
                      _buildSearchResults(searchAsync, isDark),
                    ],
                  ),
                ),
              ),
              Divider(height: 1, color: dividerColor),
              Padding(
                padding: const EdgeInsets.all(20),
                child: Row(
                  children: [
                    Expanded(
                      child: OutlinedButton(
                        onPressed: _isSending
                            ? null
                            : () => Navigator.pop(context),
                        style: OutlinedButton.styleFrom(
                          padding: const EdgeInsets.symmetric(vertical: 14),
                          side: BorderSide(color: borderColor),
                          shape: RoundedRectangleBorder(
                            borderRadius: BorderRadius.circular(12),
                          ),
                        ),
                        child: Text(
                          'Cancel',
                          style: AppTypography.button.copyWith(
                            color: textColor,
                          ),
                        ),
                      ),
                    ),
                    const SizedBox(width: 12),
                    Expanded(
                      child: ElevatedButton(
                        onPressed: canSend ? _handleSend : null,
                        style: ElevatedButton.styleFrom(
                          backgroundColor: AppColors.primaryRed,
                          padding: const EdgeInsets.symmetric(vertical: 14),
                          shape: RoundedRectangleBorder(
                            borderRadius: BorderRadius.circular(12),
                          ),
                        ),
                        child: _isSending
                            ? const SizedBox(
                                width: 20,
                                height: 20,
                                child: CircularProgressIndicator(
                                  strokeWidth: 2,
                                  valueColor: AlwaysStoppedAnimation<Color>(
                                    AppColors.neutralWhite,
                                  ),
                                ),
                              )
                            : Text(
                                'Send',
                                style: AppTypography.button.copyWith(
                                  color: AppColors.neutralWhite,
                                ),
                              ),
                      ),
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildSearchField(bool isDark, Color borderColor) {
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
        fillColor: isDark ? AppColors.darkGray700 : AppColors.neutralGray100,
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: BorderSide(color: borderColor),
        ),
        enabledBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: BorderSide(color: borderColor),
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: const BorderSide(color: AppColors.primaryRed, width: 2),
        ),
      ),
    );
  }

  Widget _buildSelectedRecipient() {
    final recipient = _selectedRecipient!;
    final label =
        UserIdentityFormatter.formatHandle(recipient.username) ??
        recipient.username;
    final initialSource = recipient.username.trim();
    final initial = initialSource.isNotEmpty
        ? initialSource[0].toUpperCase()
        : '?';

    return InputChip(
      avatar: CircleAvatar(
        backgroundColor: AppColors.neutralGray200,
        child: Text(
          initial,
          style: const TextStyle(
            fontWeight: FontWeight.w700,
            color: AppColors.neutralGray600,
          ),
        ),
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

  Widget _buildComposerField(bool isDark, Color borderColor, Color textColor) {
    return TextField(
      controller: _messageController,
      maxLines: 4,
      minLines: 1,
      style: AppTypography.bodyMedium.copyWith(color: textColor),
      decoration: InputDecoration(
        hintText: 'Write a message (optional)',
        hintStyle: AppTypography.bodyMedium.copyWith(
          color: isDark ? AppColors.neutralGray500 : AppColors.neutralGray400,
        ),
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: BorderSide(color: borderColor),
        ),
        enabledBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: BorderSide(color: borderColor),
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: const BorderSide(color: AppColors.primaryRed, width: 2),
        ),
      ),
    );
  }

  Widget _buildSearchResults(
    AsyncValue<List<UserSearch>> searchAsync,
    bool isDark,
  ) {
    return searchAsync.when(
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (error, stackTrace) => Text(
        error.toString(),
        style: TextStyle(
          color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray600,
        ),
      ),
      data: (users) {
        if (_searchQuery.isEmpty) {
          return Text(
            'Search for a recipient to continue.',
            style: TextStyle(
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray600,
            ),
          );
        }

        if (users.isEmpty) {
          return Text(
            'No users found.',
            style: TextStyle(
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray600,
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
              isDark: isDark,
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
      case ExternalShareType.listing:
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
  final bool isDark;
  final VoidCallback onTap;

  const _ShareRecipientRow({
    required this.user,
    required this.isDark,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final label = UserIdentityFormatter.formatHandle(user.username) ?? 'User';
    final labelColor = isDark
        ? AppColors.neutralWhite
        : AppColors.neutralGray900;
    final subtitleColor = isDark
        ? AppColors.neutralGray400
        : AppColors.neutralGray600;
    final initialSource = user.username.trim();
    final initial = initialSource.isNotEmpty
        ? initialSource[0].toUpperCase()
        : '?';

    return Material(
      color: Colors.transparent,
      child: ListTile(
        onTap: onTap,
        contentPadding: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
        leading: CircleAvatar(
          backgroundColor: isDark
              ? AppColors.darkGray700
              : AppColors.neutralGray200,
          child: Text(
            initial,
            style: TextStyle(color: subtitleColor, fontWeight: FontWeight.w700),
          ),
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
