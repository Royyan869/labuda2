import 'dart:ui' as ui;
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:qr_flutter/qr_flutter.dart';
import 'package:share_plus/share_plus.dart';
import 'package:path_provider/path_provider.dart';
import 'package:permission_handler/permission_handler.dart';
import 'dart:io';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/share/domain/entities/share_target.dart'
    show kPublicProfileBaseUrl;
import 'package:labuda/shared/shared.dart';

/// Profile QR Code Screen
///
/// Generates a QR code linking to user's profile page.
/// Visitors can see all info and content from the profile.
class ProfileQrScreen extends ConsumerStatefulWidget {
  const ProfileQrScreen({super.key});

  @override
  ConsumerState<ProfileQrScreen> createState() => _ProfileQrScreenState();
}

class _ProfileQrScreenState extends ConsumerState<ProfileQrScreen> {
  final GlobalKey _qrKey = GlobalKey();
  bool _isDownloading = false;

  String get _profileUrl {
    final authState = ref.read(authControllerProvider);
    if (authState is! AuthStateAuthenticated) return '';
    return '$kPublicProfileBaseUrl/profile/${authState.user.id}';
  }

  @override
  Widget build(BuildContext context) {
    final authState = ref.watch(authControllerProvider);
    final scheme = Theme.of(context).colorScheme;

    if (authState is! AuthStateAuthenticated) {
      return Scaffold(
        appBar: const AppBarCustom(title: 'My QR Code'),
        body: Center(
          child: Text(
            'Please login to generate QR code',
            style: TextStyle(color: scheme.onSurfaceVariant),
          ),
        ),
      );
    }

    return Scaffold(
      appBar: const AppBarCustom(title: 'My QR Code'),
      body: SafeArea(
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              _buildQRCodeCard(context, '@${authState.user.username}'),
              const SizedBox(height: 24),
              _buildActionButtons(context),
              const SizedBox(height: 24),
              _buildUseCaseInfo(context),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildQRCodeCard(BuildContext context, String displayName) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.all(24),
      decoration: BoxDecoration(
        color: scheme.surface,
        borderRadius: BorderRadius.circular(16),
        boxShadow: [
          BoxShadow(
            color: scheme.scrim.withValues(alpha: 0.1),
            blurRadius: 10,
            offset: const Offset(0, 4),
          ),
        ],
      ),
      child: Column(
        children: [
          // Export artifact: the captured PNG (save + share) must stay
          // black-on-white to remain scannable in any theme. Both scheme
          // roles pin the same pixels in light and dark (onPrimary = white,
          // scrim = black), so this never forks the theme.
          RepaintBoundary(
            key: _qrKey,
            child: Container(
              color: scheme.onPrimary,
              padding: const EdgeInsets.all(16),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  QrImageView(
                    data: _profileUrl,
                    version: QrVersions.auto,
                    size: 220,
                    backgroundColor: scheme.onPrimary,
                    errorCorrectionLevel: QrErrorCorrectLevel.H,
                    eyeStyle: QrEyeStyle(
                      eyeShape: QrEyeShape.square,
                      color: scheme.scrim,
                    ),
                    dataModuleStyle: QrDataModuleStyle(
                      dataModuleShape: QrDataModuleShape.square,
                      color: scheme.scrim,
                    ),
                  ),
                  const SizedBox(height: 12),
                  Text(
                    displayName,
                    style: TextStyle(
                      fontSize: 16,
                      fontWeight: FontWeight.bold,
                      color: scheme.scrim,
                    ),
                    textAlign: TextAlign.center,
                  ),
                  const SizedBox(height: 4),
                  Text(
                    'Scan to visit my profile',
                    style: TextStyle(
                      fontSize: 12,
                      color: scheme.scrim.withValues(alpha: 0.6),
                    ),
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildActionButtons(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        ElevatedButton.icon(
          onPressed: _isDownloading ? null : _downloadQRCode,
          icon: _isDownloading
              ? const SizedBox(
                  width: 16,
                  height: 16,
                  child: CircularProgressIndicator(strokeWidth: 2),
                )
              : const Icon(Icons.download),
          label: Text(_isDownloading ? 'Saving...' : 'Save to Gallery'),
          style: ElevatedButton.styleFrom(
            backgroundColor: scheme.secondary,
            foregroundColor: scheme.onSecondary,
            padding: const EdgeInsets.symmetric(vertical: 16),
          ),
        ),
        const SizedBox(height: 12),
        OutlinedButton.icon(
          onPressed: _shareQRCode,
          icon: const Icon(Icons.share),
          label: const Text('Share'),
          style: OutlinedButton.styleFrom(
            padding: const EdgeInsets.symmetric(vertical: 16),
          ),
        ),
      ],
    );
  }

  Widget _buildUseCaseInfo(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: scheme.surfaceContainer,
        borderRadius: BorderRadius.circular(12),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(
                Icons.lightbulb_outline,
                size: 18,
                color: AppColors.statusWarning,
              ),
              const SizedBox(width: 8),
              Text(
                'Tips',
                style: TextStyle(
                  fontSize: 14,
                  fontWeight: FontWeight.bold,
                  color: scheme.onSurface,
                ),
              ),
            ],
          ),
          const SizedBox(height: 8),
          _buildTipItem(context, 'Print on business cards'),
          _buildTipItem(context, 'Display at your booth or farm'),
          _buildTipItem(context, 'Share on social media'),
        ],
      ),
    );
  }

  Widget _buildTipItem(BuildContext context, String text) {
    final scheme = Theme.of(context).colorScheme;
    return Padding(
      padding: const EdgeInsets.only(top: 6),
      child: Row(
        children: [
          Icon(Icons.check, size: 14, color: AppColors.primaryGreen),
          const SizedBox(width: 8),
          Text(
            text,
            style: TextStyle(
              fontSize: 13,
              color: scheme.onSurfaceVariant,
            ),
          ),
        ],
      ),
    );
  }

  Future<void> _downloadQRCode() async {
    setState(() => _isDownloading = true);

    try {
      // Request permission
      if (Platform.isAndroid) {
        final status = await Permission.storage.request();
        if (!status.isGranted) {
          // Try photos for Android 13+
          final photosStatus = await Permission.photos.request();
          if (!photosStatus.isGranted && mounted) {
            AppSnackBar.showError(context, 'Storage permission required');
            return;
          }
        }
      }

      // Capture QR image
      final boundary =
          _qrKey.currentContext!.findRenderObject() as RenderRepaintBoundary;
      final image = await boundary.toImage(pixelRatio: 3.0);
      final byteData = await image.toByteData(format: ui.ImageByteFormat.png);
      final bytes = byteData!.buffer.asUint8List();

      // Get proper download directory
      final directory = await _getDownloadDirectory();
      final timestamp = DateTime.now().millisecondsSinceEpoch;
      final file = File('${directory.path}/labuda_qr_$timestamp.png');
      await file.writeAsBytes(bytes);

      if (mounted) {
        final location = Platform.isAndroid ? 'Downloads/Labuda' : 'Files';
        AppSnackBar.showSuccess(context, 'QR Code saved to $location');
      }
    } catch (e) {
      if (mounted) {
        AppSnackBar.showError(context, 'Gagal menyimpan. Coba lagi.');
      }
    } finally {
      if (mounted) {
        setState(() => _isDownloading = false);
      }
    }
  }

  Future<Directory> _getDownloadDirectory() async {
    if (Platform.isAndroid) {
      // Get Downloads folder
      final externalDir = await getExternalStorageDirectory();
      if (externalDir != null) {
        // Navigate from app-specific to public Downloads
        final downloadPath = externalDir.path.replaceAll(
          RegExp(r'/Android/data/[^/]+/files'),
          '/Download/Labuda',
        );
        final dir = Directory(downloadPath);
        if (!await dir.exists()) {
          await dir.create(recursive: true);
        }
        return dir;
      }
    }
    // Fallback
    return await getApplicationDocumentsDirectory();
  }

  Future<void> _shareQRCode() async {
    try {
      final boundary =
          _qrKey.currentContext!.findRenderObject() as RenderRepaintBoundary;
      final image = await boundary.toImage(pixelRatio: 3.0);
      final byteData = await image.toByteData(format: ui.ImageByteFormat.png);
      final bytes = byteData!.buffer.asUint8List();

      final directory = await getTemporaryDirectory();
      final file = File('${directory.path}/labuda_qr.png');
      await file.writeAsBytes(bytes);

      await SharePlus.instance.share(
        ShareParams(
          files: [XFile(file.path)],
          text: 'Visit my profile on LABUDA: $_profileUrl',
        ),
      );
    } catch (e) {
      if (mounted) {
        AppSnackBar.showError(context, 'Failed to share QR code');
      }
    }
  }
}
