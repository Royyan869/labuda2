import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/shared/helpers/user_identity_formatter.dart';
import 'package:labuda/features/search/search/domain/entities/user_search.dart';
import 'package:labuda/domains/chat/chat/data/chat_providers.dart';

/// User List Item Widget untuk New Chat Screen.
///
/// Canonical shape (C1B2): the row consumes the lightweight [UserSearch]
/// projection from the search domain — it never touches ProfileEntity or any
/// profile metadata (location / farm / verification). Identity label is the
/// canonical handle only (Owner truth: public identity = username).
class NewChatUserListWidget extends ConsumerWidget {
  final UserSearch user;

  const NewChatUserListWidget({super.key, required this.user});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final scheme = Theme.of(context).colorScheme;
    return InkWell(
      onTap: () => _handleTap(context, ref),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
        decoration: BoxDecoration(
          border: Border(
            bottom: BorderSide(color: scheme.outlineVariant, width: 0.5),
          ),
        ),
        child: Row(
          children: [
            // Avatar — canonical: image or user icon. No initials exist
            // anywhere in the app (Owner decision 2026-09-24). The search
            // projection's avatarUrl is wired as the instant fallback while
            // AvatarCacheService resolves the fresh URL in the background.
            HybridAvatar(
              userId: user.userId,
              savedAvatarUrl: user.avatarUrl,
              size: 40,
            ),
            const SizedBox(width: 12),

            // Identity — canonical handle only: '@username', or 'User' when
            // the username is blank. No location / farm / verification data
            // exists in the search projection.
            Expanded(
              child: Text(
                UserIdentityFormatter.formatHandle(user.username) ?? 'User',
                style: TextStyle(
                  fontSize: 16,
                  fontWeight: FontWeight.w500,
                  color: scheme.onSurface,
                ),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
              ),
            ),

            // Arrow icon
            Icon(Icons.chevron_right, color: scheme.onSurfaceVariant),
          ],
        ),
      ),
    );
  }

  Future<void> _handleTap(BuildContext context, WidgetRef ref) async {
    // Live principal read at tap time — never a captured constructor prop,
    // so a principal switch between build and tap cannot start a chat as
    // the stale identity.
    final livePrincipalId = ref.read(currentUserIdProvider);

    // Self-chat guard uses the same live principal.
    if (livePrincipalId == user.userId) return;

    // Check email verification before starting new chat
    final isEmailVerified = ref.read(isEmailVerifiedProvider);

    if (!isEmailVerified) {
      AppSnackBar.showWarning(
        context,
        'Please verify your email to start new conversations.',
      );
      return;
    }

    // Show loading
    showDialog(
      context: context,
      barrierDismissible: false,
      builder: (_) => const Center(child: CircularProgressIndicator()),
    );

    try {
      // Get or create chat using chat repository
      final chatRepository = ref.read(chatRepositoryProvider);
      final result = await chatRepository.getOrCreateChat(
        participantIds: [livePrincipalId, user.userId],
      );

      if (!context.mounted) return;

      // Close loading
      Navigator.of(context).pop();

      if (result.isSuccess && result.data != null) {
        final chat = result.data!;
        // Pop new chat screen first, then navigate to chat
        context.pop();
        context.push('/chat/${chat.id}');
      } else {
        // Show error - check if user is blocked
        final error = result.error ?? '';
        final errorMessage = error.toLowerCase().contains('blocked')
            ? 'Tidak dapat memulai chat. Pengguna ini telah memblokir Anda.'
            : 'Gagal memulai chat. Coba lagi.';
        AppSnackBar.showError(context, errorMessage);
      }
    } catch (e) {
      if (!context.mounted) return;

      // Close loading
      Navigator.of(context).pop();

      // Show error
      AppSnackBar.showError(context, 'Terjadi kesalahan. Coba lagi.');
    }
  }
}
