import 'package:labuda/core/core.dart';
import 'package:share_plus/share_plus.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:flutter/services.dart';

/// Service for handling native platform sharing
/// Wraps share_plus and url_launcher packages
/// Data layer - contains platform-specific code
abstract class NativeShareService {
  Future<Result<bool>> shareViaDialog({
    required String text,
    String? subject,
  });

  Future<Result<bool>> shareToWhatsApp({required String text});

  Future<Result<bool>> shareToTelegram({required String text});

  Future<Result<bool>> shareToInstagramStory({
    required String imageUrl,
  });

  Future<Result<bool>> copyToClipboard({required String text});

  Future<Result<bool>> shareViaEmail({
    required String subject,
    required String body,
  });
}

/// Implementation of NativeShareService
class NativeShareServiceImpl implements NativeShareService {
  @override
  Future<Result<bool>> shareViaDialog({
    required String text,
    String? subject,
  }) async {
    try {
      await SharePlus.instance.share(ShareParams(text: text, subject: subject));
      return Result.success(true);
    } catch (e) {
      return Result.error('Failed to share: $e');
    }
  }

  @override
  Future<Result<bool>> shareToWhatsApp({
    required String text,
  }) async {
    try {
      final url = 'whatsapp://send?text=${Uri.encodeComponent(text)}';
      final uri = Uri.parse(url);
      final canLaunch = await canLaunchUrl(uri);

      if (!canLaunch) {
        return Result.error('WhatsApp is not installed');
      }

      await launchUrl(uri, mode: LaunchMode.externalApplication);
      return Result.success(true);
    } catch (e) {
      return Result.error('Failed to share to WhatsApp: $e');
    }
  }

  @override
  Future<Result<bool>> shareToTelegram({
    required String text,
  }) async {
    try {
      final url = 'https://t.me/share/url?url=${Uri.encodeComponent(text)}';
      final uri = Uri.parse(url);

      await launchUrl(uri, mode: LaunchMode.externalApplication);
      return Result.success(true);
    } catch (e) {
      return Result.error('Failed to share to Telegram: $e');
    }
  }

  @override
  Future<Result<bool>> shareToInstagramStory({
    required String imageUrl,
  }) async {
    // Instagram Story sharing requires native platform implementation
    // This is a placeholder - actual implementation needs platform channels
    return Result.error('Instagram Story feature not available yet');
  }

  @override
  Future<Result<bool>> copyToClipboard({
    required String text,
  }) async {
    try {
      await Clipboard.setData(ClipboardData(text: text));
      return Result.success(true);
    } catch (e) {
      return Result.error('Failed to copy link: $e');
    }
  }

  @override
  Future<Result<bool>> shareViaEmail({
    required String subject,
    required String body,
  }) async {
    try {
      final url =
          'mailto:?subject=${Uri.encodeComponent(subject)}'
          '&body=${Uri.encodeComponent(body)}';

      final uri = Uri.parse(url);
      await launchUrl(uri);
      return Result.success(true);
    } catch (e) {
      return Result.error('Failed to open email: $e');
    }
  }
}
