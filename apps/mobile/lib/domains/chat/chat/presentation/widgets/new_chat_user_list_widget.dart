import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/domains/user/profile/profile.dart';
import 'package:labuda/domains/chat/chat/data/chat_providers.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_providers.dart';

/// User List Item Widget untuk New Chat Screen
class NewChatUserListWidget extends ConsumerWidget {
  final ProfileEntity profile;
  final String currentUserId;
  final bool isDark;

  const NewChatUserListWidget({
    super.key,
    required this.profile,
    required this.currentUserId,
    required this.isDark,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return InkWell(
      onTap: () => _handleTap(context, ref),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
        decoration: BoxDecoration(
          border: Border(
            bottom: BorderSide(
              color: isDark ? AppColors.darkGray700 : AppColors.neutralGray200,
              width: 0.5,
            ),
          ),
        ),
        child: Row(
          children: [
            // Avatar
            // Initials must come from real identity, never from UUID bytes.
            // ProfileEntity carries no name/username (moved to AuthUser); the
            // only truthful name in this widget is FarmInfo.farmName (sellers).
            // For non-sellers we pass '' so ProfileAvatar renders a blank
            // gradient circle instead of UUID-derived fake initials.
            HybridAvatar.medium(
              userId: profile.userId,
              initials: profile.farmInfo?.farmName.trim().isNotEmpty == true
                  ? UserInitialsHelper.fromName(profile.farmInfo!.farmName)
                  : '',
            ),
            const SizedBox(width: 12),

            // User info
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  // Location (if available)
                  if (profile.location != null)
                    Text(
                      profile.location!,
                      style: TextStyle(
                        fontSize: 16,
                        fontWeight: FontWeight.w500,
                        color: isDark
                            ? AppColors.neutralWhite
                            : AppColors.neutralGray900,
                      ),
                    ),

                  // Phone verification indicator
                  const SizedBox(height: 2),
                  Icon(
                    Icons.verified,
                    size: 14,
                    color: profile.verification.isPhoneVerified
                        ? AppColors.success
                        : AppColors.neutralGray400,
                  ),
                ],
              ),
            ),

            // Arrow icon
            Icon(
              Icons.chevron_right,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray500,
            ),
          ],
        ),
      ),
    );
  }

  Future<void> _handleTap(BuildContext context, WidgetRef ref) async {
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
        participantIds: [currentUserId, profile.userId],
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
