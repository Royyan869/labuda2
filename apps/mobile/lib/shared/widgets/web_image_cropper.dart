import 'dart:typed_data';
import 'dart:ui' as ui;
import 'package:flutter/material.dart';
import 'package:image_picker/image_picker.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/core/src/theme/app_theme.dart';

/// Web-specific Image Cropper Widget
///
/// Canvas cropping is the single web crop engine (crop_your_image covers
/// mobile). No platform crop plugin is used on either platform.
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

      // Exported-pixel background stays white in both modes (canvas data,
      // not UI theme): onPrimary is white in both schemes — zero pixel
      // change, authority-clean.
      canvas.drawRect(
        Rect.fromLTWH(0, 0, _cropWidth, _cropHeight),
        Paint()..color = Theme.of(context).colorScheme.onPrimary,
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
    final scheme = Theme.of(context).colorScheme;

    return Scaffold(
      backgroundColor: scheme.surfaceContainerLowest,
      body: SafeArea(
        child: SizedBox(
          width: double.infinity,
          height: double.infinity,
          child: Center(
            child: Container(
              width: 500,
              height: 600,
              decoration: BoxDecoration(
                color: scheme.surface,
                borderRadius: BorderRadius.circular(AppShape.r16),
                boxShadow: [
                  BoxShadow(
                    color: scheme.shadow.withValues(alpha: 0.2),
                    blurRadius: 20,
                    offset: const Offset(0, 10),
                  ),
                ],
              ),
              child: Column(
                children: [
                  // Header
                  Container(
                    padding: const EdgeInsets.all(AppMetrics.p16),
                    decoration: BoxDecoration(
                      color: scheme.surfaceContainerHighest,
                      borderRadius: const BorderRadius.vertical(
                        top: Radius.circular(AppShape.r16),
                      ),
                    ),
                    child: Row(
                      children: [
                        Text(
                          widget.title,
                          style: TextStyle(
                            fontSize: AppType.s18,
                            fontWeight: FontWeight.w600,
                            color: scheme.onSurface,
                          ),
                        ),
                        const Spacer(),
                        IconButton(
                          onPressed: widget.onCancel,
                          icon: Icon(
                            Icons.close,
                            color: scheme.onSurfaceVariant,
                          ),
                        ),
                      ],
                    ),
                  ),

                  // Crop Area
                  Expanded(
                    child: _isLoading
                        ? Center(
                            child: CircularProgressIndicator(
                              color: scheme.primary,
                            ),
                          )
                        : Container(
                            padding: const EdgeInsets.all(AppMetrics.p16),
                            child: _buildCropArea(),
                          ),
                  ),

                  // Controls
                  Container(
                    padding: const EdgeInsets.all(AppMetrics.p16),
                    decoration: BoxDecoration(
                      color: scheme.surfaceContainerHighest,
                      borderRadius: const BorderRadius.vertical(
                        bottom: Radius.circular(AppShape.r16),
                      ),
                    ),
                    child: Column(
                      children: [
                        // Scale slider
                        Row(
                          children: [
                            Icon(
                              Icons.zoom_out,
                              color: scheme.onSurfaceVariant,
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
                                activeColor: scheme.primary,
                                onChanged: (value) {
                                  setState(() {
                                    _scale = value.clamp(0.5, 3.0);
                                  });
                                },
                              ),
                            ),
                            Icon(
                              Icons.zoom_in,
                              color: scheme.onSurfaceVariant,
                              size: 20,
                            ),
                          ],
                        ),

                        const SizedBox(height: 16),

                        // Action buttons
                        Row(
                          children: [
                            Expanded(
                              child: OutlinedButton(
                                onPressed: widget.onCancel,
                                child: const Text('Cancel'),
                              ),
                            ),
                            const SizedBox(width: 12),
                            Expanded(
                              child: ElevatedButton(
                                onPressed: _cropAndSave,
                                child: const Text('Crop'),
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
        : BorderRadius.circular(AppShape.r8);

    return Center(
      child: Container(
        width: _cropWidth,
        height: _cropHeight,
        decoration: BoxDecoration(
          border: Border.all(
            color: Theme.of(context).colorScheme.primary,
            width: 2,
          ),
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
