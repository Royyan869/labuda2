import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/shared/providers/block_state_provider.dart';
import 'package:labuda/shared/widgets/app_snackbar.dart';

/// Utility class untuk mengecek dan menangani interaksi dengan blocked users
class BlockCheckUtil {
  /// Cek apakah bisa berinteraksi dengan user
  ///
  /// Returns true jika TIDAK diblokir (bisa interact)
  /// Returns false jika diblokir (tidak bisa interact)
  static bool canInteract(WidgetRef ref, String targetUserId) {
    return !ref.read(isUserBlockedProvider(targetUserId));
  }

  /// Cek dan tampilkan pesan jika user diblokir
  ///
  /// Returns true jika bisa interact (tidak diblokir)
  /// Returns false jika tidak bisa interact (diblokir) + show message
  static bool checkAndShowMessage(
    BuildContext context,
    WidgetRef ref,
    String targetUserId, {
    String? customMessage,
  }) {
    final isBlocked = ref.read(isUserBlockedProvider(targetUserId));

    if (isBlocked) {
      showBlockedMessage(context, customMessage: customMessage);
      return false;
    }

    return true;
  }

  /// Tampilkan pesan bahwa user diblokir
  static void showBlockedMessage(
    BuildContext context, {
    String? customMessage,
  }) {
    AppSnackBar.showWarning(
      context,
      customMessage ?? 'Tidak bisa berinteraksi dengan user yang diblokir',
    );
  }

  /// Cek apakah bisa follow user
  static bool canFollow(
    BuildContext context,
    WidgetRef ref,
    String targetUserId,
  ) {
    return checkAndShowMessage(
      context,
      ref,
      targetUserId,
      customMessage: 'Tidak bisa follow user yang diblokir',
    );
  }

  /// Cek apakah bisa mengirim chat ke user
  static bool canChat(
    BuildContext context,
    WidgetRef ref,
    String targetUserId,
  ) {
    return checkAndShowMessage(
      context,
      ref,
      targetUserId,
      customMessage: 'Tidak bisa chat dengan user yang diblokir',
    );
  }

  /// Cek apakah bisa comment pada konten user
  static bool canComment(
    BuildContext context,
    WidgetRef ref,
    String contentAuthorId,
  ) {
    return checkAndShowMessage(
      context,
      ref,
      contentAuthorId,
      customMessage: 'Tidak bisa comment pada konten user yang diblokir',
    );
  }
}
