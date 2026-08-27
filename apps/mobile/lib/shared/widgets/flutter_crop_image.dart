import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:crop_your_image/crop_your_image.dart';
import 'package:image_picker/image_picker.dart';
import 'package:labuda/core/core.dart';

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
    SystemChrome.setSystemUIOverlayStyle(
      const SystemUiOverlayStyle(
        statusBarColor: Colors.transparent,
        statusBarIconBrightness: Brightness.light,
        statusBarBrightness: Brightness.dark,
        systemNavigationBarColor: Colors.transparent,
        systemNavigationBarIconBrightness: Brightness.light,
      ),
    );
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
      backgroundColor: AppColors.dark,
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
                        if (context.mounted) {
                          Navigator.of(context).pop();
                          ScaffoldMessenger.of(context).showSnackBar(
                            SnackBar(
                              content: Text('Crop failed: $cause'),
                              duration: const Duration(seconds: 4),
                            ),
                          );
                        }
                    }
                  },
                  aspectRatio: widget.aspectRatio,
                  withCircleUi: widget.withCircleUi,
                  baseColor: AppColors.dark,
                  maskColor: Colors.black.withValues(alpha: 0.5),
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
                    color: Colors.black87,
                    padding: const EdgeInsets.symmetric(
                      horizontal: 8,
                      vertical: 8,
                    ),
                    child: Row(
                      mainAxisAlignment: MainAxisAlignment.spaceBetween,
                      children: [
                        IconButton(
                          icon: const Icon(Icons.close, color: AppColors.light),
                          onPressed: () => Navigator.of(context).pop(),
                        ),
                        Text(
                          widget.title,
                          style: const TextStyle(
                            color: AppColors.light,
                            fontSize: 18,
                            fontWeight: FontWeight.w600,
                          ),
                        ),
                        IconButton(
                          icon: const Icon(
                            Icons.check,
                            color: AppColors.success,
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
                    margin: const EdgeInsets.symmetric(horizontal: 20),
                    padding: const EdgeInsets.all(12),
                    decoration: BoxDecoration(
                      color: Colors.black87,
                      borderRadius: BorderRadius.circular(8),
                    ),
                    child: const Text(
                      'Pinch to zoom • Drag to move',
                      textAlign: TextAlign.center,
                      style: TextStyle(color: AppColors.light, fontSize: 13),
                    ),
                  ),
                ),
              ],
            ),
    );
  }
}
