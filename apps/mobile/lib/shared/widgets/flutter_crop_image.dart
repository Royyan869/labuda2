import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:crop_your_image/crop_your_image.dart';
import 'package:image_picker/image_picker.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/shared/widgets/app_snackbar.dart';

/// Flutter Image Cropper - Pure Flutter Implementation
///
/// Best Practice:
/// - Full Flutter widget (no native code)
/// - Proper edge-to-edge handling
/// - Smooth UX with native-like gestures
/// - Clean architecture
/// - Configurable aspect ratio and crop UI
class FlutterImageCropper extends StatefulWidget {
  final XFile imageFile;
  final Function(Uint8List) onCropped;
  final double aspectRatio;
  final bool withCircleUi;
  final String title;

  const FlutterImageCropper({
    super.key,
    required this.imageFile,
    required this.onCropped,
    this.aspectRatio = 1.0,
    this.withCircleUi = true,
    this.title = 'Crop Avatar',
  });

  @override
  State<FlutterImageCropper> createState() => _FlutterImageCropperState();
}

class _FlutterImageCropperState extends State<FlutterImageCropper> {
  final _cropController = CropController();
  Uint8List? _imageData;
  bool _isLoading = true;

  @override
  void initState() {
    super.initState();
    _setSystemUIMode();
    _loadImage();
  }

  @override
  void dispose() {
    _restoreSystemUIMode();
    super.dispose();
  }

  void _setSystemUIMode() {
    SystemChrome.setEnabledSystemUIMode(SystemUiMode.edgeToEdge);
    SystemChrome.setSystemUIOverlayStyle(AppTheme.immersiveOverlayStyle);
  }

  void _restoreSystemUIMode() {
    SystemChrome.setEnabledSystemUIMode(
      SystemUiMode.manual,
      overlays: SystemUiOverlay.values,
    );
  }

  Future<void> _loadImage() async {
    try {
      final bytes = await widget.imageFile.readAsBytes();
      if (mounted) {
        setState(() {
          _imageData = bytes;
          _isLoading = false;
        });
      }
    } catch (e) {
      if (mounted) {
        Navigator.of(context).pop();
      }
    }
  }

  void _crop() {
    _cropController.crop();
  }

  @override
  Widget build(BuildContext context) {
    final topPadding = MediaQuery.of(context).padding.top;
    final bottomPadding = MediaQuery.of(context).padding.bottom;

    return Scaffold(
      backgroundColor: Theme.of(context).colorScheme.scrim,
      body: _isLoading
          ? const Center(child: CircularProgressIndicator())
          : Stack(
              children: [
                // Crop widget
                Crop(
                  image: _imageData!,
                  controller: _cropController,
                  onCropped: (result) {
                    switch (result) {
                      case CropSuccess(:final croppedImage):
                        // Pop FIRST before calling callback to avoid double pop
                        Navigator.of(context).pop();
                        widget.onCropped(croppedImage);
                      case CropFailure(:final cause):
                        // Technical cause stays in the debug log; the user only
                        // sees safe, actionable copy.
                        debugPrint('FlutterImageCropper: crop failed - $cause');
                        if (context.mounted) {
                          Navigator.of(context).pop();
                          AppSnackBar.showError(
                            context,
                            'Gagal memotong gambar. Coba lagi.',
                          );
                        }
                    }
                  },
                  aspectRatio: widget.aspectRatio,
                  withCircleUi: widget.withCircleUi,
                  baseColor: Theme.of(context).colorScheme.scrim,
                  maskColor: Theme.of(
                    context,
                  ).colorScheme.scrim.withValues(alpha: 0.5),
                  radius: 0,
                  cornerDotBuilder: (size, edgeAlignment) => const DotControl(),
                  interactive: true,
                  fixCropRect: false,
                  clipBehavior: Clip.none,
                ),

                // Top AppBar
                Positioned(
                  top: topPadding,
                  left: 0,
                  right: 0,
                  child: Container(
                    color: Theme.of(
                      context,
                    ).colorScheme.scrim.withValues(alpha: 0.87),
                    padding: const EdgeInsets.symmetric(
                      horizontal: AppMetrics.p8,
                      vertical: AppMetrics.p8,
                    ),
                    child: Row(
                      mainAxisAlignment: MainAxisAlignment.spaceBetween,
                      children: [
                        IconButton(
                          icon: Icon(
                            Icons.close,
                            color: Theme.of(context).colorScheme.onPrimary,
                           semanticLabel: 'Tutup',
                           ),
                          onPressed: () => Navigator.of(context).pop(),
                        ),
                        Text(
                          widget.title,
                          style: context.typeRoles.titleSection.copyWith(
                            color: Theme.of(context).colorScheme.onPrimary,
                            fontWeight: FontWeight.w600,
                          ),
                        ),
                        IconButton(
                          icon: Icon(
                            Icons.check,
                            color: context.statusColors.success,
                           semanticLabel: 'Simpan',
                           ),
                          onPressed: _crop,
                        ),
                      ],
                    ),
                  ),
                ),

                // Bottom info
                Positioned(
                  bottom: bottomPadding + 16,
                  left: 0,
                  right: 0,
                  child: Container(
                    margin: const EdgeInsets.symmetric(
                      horizontal: AppMetrics.p24,
                    ),
                    padding: const EdgeInsets.all(AppMetrics.p12),
                    decoration: BoxDecoration(
                      color: Theme.of(
                        context,
                      ).colorScheme.scrim.withValues(alpha: 0.87),
                      borderRadius: BorderRadius.circular(AppShape.r8),
                    ),
                    child: Text(
                      'Pinch to zoom • Drag to move',
                      textAlign: TextAlign.center,
                      style: context.typeRoles.bodyDense.copyWith(
                        color: Theme.of(context).colorScheme.onPrimary,
                      ),
                    ),
                  ),
                ),
              ],
            ),
    );
  }
}
