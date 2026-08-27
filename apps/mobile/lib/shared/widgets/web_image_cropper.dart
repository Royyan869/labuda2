import 'package:labuda/core/src/theme/app_colors.dart';
import 'dart:typed_data';
import 'dart:ui' as ui;
import 'package:flutter/material.dart';
import 'package:image_picker/image_picker.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';

/// Web-specific Image Cropper Widget
///
/// Menggunakan Flutter Canvas untuk cropping karena image_cropper
/// tidak support blob URL dengan baik di web
class WebImageCropper extends StatefulWidget {
  final XFile imageFile;
  final Function(Uint8List croppedBytes) onCropped;
  final VoidCallback onCancel;
  final double aspectRatio;
  final bool withCircleUi;
  final String title;

  const WebImageCropper({
    super.key,
    required this.imageFile,
    required this.onCropped,
    required this.onCancel,
    this.aspectRatio = 1.0,
    this.withCircleUi = true,
    this.title = 'Crop Avatar',
  });

  @override
  State<WebImageCropper> createState() => _WebImageCropperState();
}

class _WebImageCropperState extends State<WebImageCropper> {
  ui.Image? _image;
  bool _isLoading = true;
  double _scale = 1.0;
  Offset _offset = Offset.zero;
  final GlobalKey _cropKey = GlobalKey();

  // Calculate crop dimensions based on aspect ratio
  // Base size is 300, adjust width or height based on aspect ratio
  double get _cropWidth =>
      widget.aspectRatio >= 1.0 ? 300.0 : 300.0 * widget.aspectRatio;
  double get _cropHeight =>
      widget.aspectRatio >= 1.0 ? 300.0 / widget.aspectRatio : 300.0;

  @override
  void initState() {
    super.initState();
    _loadImage();
  }

  Future<void> _loadImage() async {
    try {
      final bytes = await widget.imageFile.readAsBytes();
      final codec = await ui.instantiateImageCodec(bytes);
      final frame = await codec.getNextFrame();

      if (mounted) {
        setState(() {
          _image = frame.image;
          _isLoading = false;
          // Center the image initially
          _centerImage();
        });
      }
    } catch (e) {
      widget.onCancel();
    }
  }

  void _centerImage() {
    if (_image == null) return;

    final imageAspect = _image!.width / _image!.height;
    final cropAspect = _cropWidth / _cropHeight;

    if (imageAspect > cropAspect) {
      // Image is wider than crop area - fit to height
      _scale = _cropHeight / _image!.height;
    } else {
      // Image is taller than crop area - fit to width
      _scale = _cropWidth / _image!.width;
    }

    // Ensure scale is within valid range (0.5 to 3.0)
    _scale = _scale.clamp(0.5, 3.0);

    _offset = Offset.zero;
  }

  Future<void> _cropAndSave() async {
    if (_image == null) return;

    try {
      final recorder = ui.PictureRecorder();
      final canvas = Canvas(recorder);

      // Calculate the source rectangle
      final scaledWidth = _image!.width * _scale;
      final scaledHeight = _image!.height * _scale;

      // Center the scaled image in the crop area
      final imageX = (_cropWidth - scaledWidth) / 2 + _offset.dx;
      final imageY = (_cropHeight - scaledHeight) / 2 + _offset.dy;

      // Ensure we have a white background
      canvas.drawRect(
        Rect.fromLTWH(0, 0, _cropWidth, _cropHeight),
        Paint()..color = AppColors.light,
      );

      // Draw the image with high quality
      canvas.drawImageRect(
        _image!,
        Rect.fromLTWH(
          0,
          0,
          _image!.width.toDouble(),
          _image!.height.toDouble(),
        ),
        Rect.fromLTWH(imageX, imageY, scaledWidth, scaledHeight),
        Paint()..filterQuality = FilterQuality.high,
      );

      final picture = recorder.endRecording();
      final croppedImage = await picture.toImage(
        _cropWidth.toInt(),
        _cropHeight.toInt(),
      );

      // Use PNG format for better web compatibility and quality
      final byteData = await croppedImage.toByteData(
        format: ui.ImageByteFormat.png,
      );

      if (byteData != null && mounted) {
        final croppedBytes = byteData.buffer.asUint8List();

        // Validate minimum size and PNG header
        if (croppedBytes.length < 100) {
          AppSnackBar.showError(context, 'Cropped image is invalid');
          return;
        }

        // Validate PNG signature
        if (croppedBytes.length >= 8) {
          final pngSignature = [0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A];
          bool isValidPNG = true;
          for (int i = 0; i < 8; i++) {
            if (croppedBytes[i] != pngSignature[i]) {
              isValidPNG = false;
              break;
            }
          }

          if (!isValidPNG) {
            AppSnackBar.showError(context, 'Cropped image format is invalid');
            return;
          }
        }

        widget.onCropped(croppedBytes);
      } else {}
    } catch (e) {
      if (mounted) {
        AppSnackBar.showError(context, 'Failed to crop image');
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Scaffold(
      backgroundColor: isDark
          ? AppColors.darkGray900
          : AppColors.neutralGray100,
      body: SafeArea(
        child: SizedBox(
          width: double.infinity,
          height: double.infinity,
          child: Center(
            child: Container(
              width: 500,
              height: 600,
              decoration: BoxDecoration(
                color: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
                borderRadius: BorderRadius.circular(16),
                boxShadow: [
                  BoxShadow(
                    color: AppColors.dark.withValues(alpha: 0.2),
                    blurRadius: 20,
                    offset: const Offset(0, 10),
                  ),
                ],
              ),
              child: Column(
                children: [
                  // Header
                  Container(
                    padding: const EdgeInsets.all(16),
                    decoration: BoxDecoration(
                      color: isDark
                          ? AppColors.darkGray700
                          : AppColors.neutralGray100,
                      borderRadius: const BorderRadius.vertical(
                        top: Radius.circular(16),
                      ),
                    ),
                    child: Row(
                      children: [
                        Text(
                          widget.title,
                          style: TextStyle(
                            fontSize: 18,
                            fontWeight: FontWeight.w600,
                            color: isDark
                                ? AppColors.neutralWhite
                                : AppColors.neutralGray900,
                          ),
                        ),
                        const Spacer(),
                        IconButton(
                          onPressed: widget.onCancel,
                          icon: Icon(
                            Icons.close,
                            color: isDark
                                ? AppColors.neutralGray400
                                : AppColors.neutralGray600,
                          ),
                        ),
                      ],
                    ),
                  ),

                  // Crop Area
                  Expanded(
                    child: _isLoading
                        ? const Center(
                            child: CircularProgressIndicator(
                              color: AppColors.primaryRed,
                            ),
                          )
                        : Container(
                            padding: const EdgeInsets.all(16),
                            child: _buildCropArea(),
                          ),
                  ),

                  // Controls
                  Container(
                    padding: const EdgeInsets.all(16),
                    decoration: BoxDecoration(
                      color: isDark
                          ? AppColors.darkGray700
                          : AppColors.neutralGray100,
                      borderRadius: const BorderRadius.vertical(
                        bottom: Radius.circular(16),
                      ),
                    ),
                    child: Column(
                      children: [
                        // Scale slider
                        Row(
                          children: [
                            Icon(
                              Icons.zoom_out,
                              color: isDark
                                  ? AppColors.neutralGray400
                                  : AppColors.neutralGray600,
                              size: 20,
                            ),
                            Expanded(
                              child: Slider(
                                value: _scale.clamp(
                                  0.5,
                                  3.0,
                                ), // Ensure value is always in range
                                min: 0.5,
                                max: 3.0,
                                activeColor: AppColors.primaryRed,
                                onChanged: (value) {
                                  setState(() {
                                    _scale = value.clamp(0.5, 3.0);
                                  });
                                },
                              ),
                            ),
                            Icon(
                              Icons.zoom_in,
                              color: isDark
                                  ? AppColors.neutralGray400
                                  : AppColors.neutralGray600,
                              size: 20,
                            ),
                          ],
                        ),

                        const SizedBox(height: 16),

                        // Action buttons
                        Row(
                          children: [
                            Expanded(
                              child: AppButton(
                                text: 'Cancel',
                                onPressed: widget.onCancel,
                                type: AppButtonType.secondary,
                              ),
                            ),
                            const SizedBox(width: 12),
                            Expanded(
                              child: AppButton(
                                text: 'Crop',
                                onPressed: _cropAndSave,
                                type: AppButtonType.primary,
                              ),
                            ),
                          ],
                        ),
                      ],
                    ),
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildCropArea() {
    if (_image == null) return const SizedBox.shrink();

    // Border radius: circular for avatar (half of size), rectangular for cover
    final borderRadius = widget.withCircleUi
        ? BorderRadius.circular(_cropWidth / 2)
        : BorderRadius.circular(8);

    return Center(
      child: Container(
        width: _cropWidth,
        height: _cropHeight,
        decoration: BoxDecoration(
          border: Border.all(color: AppColors.primaryRed, width: 2),
          borderRadius: borderRadius,
        ),
        child: ClipRRect(
          borderRadius: borderRadius,
          child: GestureDetector(
            onPanUpdate: (details) {
              setState(() {
                _offset += details.delta;
              });
            },
            child: CustomPaint(
              key: _cropKey,
              size: Size(_cropWidth, _cropHeight),
              painter: _ImagePainter(
                image: _image!,
                scale: _scale,
                offset: _offset,
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _ImagePainter extends CustomPainter {
  final ui.Image image;
  final double scale;
  final Offset offset;

  _ImagePainter({
    required this.image,
    required this.scale,
    required this.offset,
  });

  @override
  void paint(Canvas canvas, Size size) {
    final paint = Paint()..filterQuality = FilterQuality.high;

    // Calculate scaled dimensions
    final scaledWidth = image.width * scale;
    final scaledHeight = image.height * scale;

    // Center the image in the crop area
    final imageX = (size.width - scaledWidth) / 2 + offset.dx;
    final imageY = (size.height - scaledHeight) / 2 + offset.dy;

    canvas.drawImageRect(
      image,
      Rect.fromLTWH(0, 0, image.width.toDouble(), image.height.toDouble()),
      Rect.fromLTWH(imageX, imageY, scaledWidth, scaledHeight),
      paint,
    );
  }

  @override
  bool shouldRepaint(_ImagePainter oldDelegate) {
    return oldDelegate.scale != scale ||
        oldDelegate.offset != offset ||
        oldDelegate.image != image;
  }
}
