import 'package:labuda/core/src/theme/app_colors.dart';
import 'dart:async';
import 'dart:ui' as ui;
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:image_picker/image_picker.dart';

/// Custom Image Cropper Widget for Web
/// Features:
/// - Pinch to zoom
/// - Drag to move
/// - Resize crop area
/// - Real-time preview
class CustomImageCropper extends StatefulWidget {
  final XFile imageFile;
  final Function(Uint8List) onCropped;

  const CustomImageCropper({
    super.key,
    required this.imageFile,
    required this.onCropped,
  });

  @override
  State<CustomImageCropper> createState() => _CustomImageCropperState();
}

class _CustomImageCropperState extends State<CustomImageCropper> {
  ui.Image? _image;
  Uint8List? _imageBytes;

  // Transform controls
  double _scale = 1.0;
  Offset _offset = Offset.zero;

  // Crop area
  Rect _cropRect = const Rect.fromLTWH(100, 100, 200, 200);

  // Gesture detectors
  Offset? _lastFocalPoint;
  double? _lastScale;

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
      if (bytes.isEmpty) {
        debugPrint('Error: Image bytes are empty');
        if (mounted) Navigator.of(context).pop();
        return;
      }

      final codec = await ui.instantiateImageCodec(bytes);
      final frame = await codec.getNextFrame();

      if (mounted) {
        setState(() {
          _imageBytes = bytes;
          _image = frame.image;
          _isLoading = false;
        });

        // Initialize crop rect after build completes
        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (mounted) {
            final size = MediaQuery.of(context).size;
            final cropSize = size.width * 0.6;
            setState(() {
              _cropRect = Rect.fromCenter(
                center: Offset(size.width / 2, size.height / 2 - 100),
                width: cropSize,
                height: cropSize,
              );
            });
          }
        });
      }
    } catch (e) {
      debugPrint('Error loading image: $e');
      if (mounted) {
        Navigator.of(context).pop();
      }
    }
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
                // Image with transform
                GestureDetector(
                  onScaleStart: _onScaleStart,
                  onScaleUpdate: _onScaleUpdate,
                  onScaleEnd: _onScaleEnd,
                  child: SizedBox(
                    width: double.infinity,
                    height: double.infinity,
                    child: CustomPaint(
                      painter: ImageCropPainter(
                        image: _image!,
                        scale: _scale,
                        offset: _offset,
                        cropRect: _cropRect,
                      ),
                    ),
                  ),
                ),

                // Crop area overlay
                CustomPaint(painter: CropOverlayPainter(cropRect: _cropRect)),

                // Crop area handles
                _buildCropHandle(_cropRect.topLeft, 'tl'),
                _buildCropHandle(_cropRect.topRight, 'tr'),
                _buildCropHandle(_cropRect.bottomLeft, 'bl'),
                _buildCropHandle(_cropRect.bottomRight, 'br'),

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
                        const Text(
                          'Crop Image',
                          style: TextStyle(
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
                          onPressed: _cropImage,
                        ),
                      ],
                    ),
                  ),
                ),

                // Bottom Controls
                Positioned(
                  bottom: bottomPadding,
                  left: 0,
                  right: 0,
                  child: Container(
                    color: Colors.black87,
                    padding: const EdgeInsets.symmetric(
                      horizontal: 20,
                      vertical: 16,
                    ),
                    child: Row(
                      mainAxisAlignment: MainAxisAlignment.center,
                      children: [
                        IconButton(
                          icon: const Icon(
                            Icons.zoom_out,
                            color: AppColors.light,
                          ),
                          onPressed: () {
                            setState(() {
                              _scale = (_scale - 0.1).clamp(0.5, 3.0);
                            });
                          },
                        ),
                        const SizedBox(width: 8),
                        Text(
                          '${(_scale * 100).toInt()}%',
                          style: const TextStyle(color: AppColors.light),
                        ),
                        const SizedBox(width: 8),
                        IconButton(
                          icon: const Icon(
                            Icons.zoom_in,
                            color: AppColors.light,
                          ),
                          onPressed: () {
                            setState(() {
                              _scale = (_scale + 0.1).clamp(0.5, 3.0);
                            });
                          },
                        ),
                      ],
                    ),
                  ),
                ),
              ],
            ),
    );
  }

  Widget _buildCropHandle(Offset position, String type) {
    return Positioned(
      left: position.dx - 10,
      top: position.dy - 10,
      child: GestureDetector(
        onPanUpdate: (details) {
          setState(() {
            switch (type) {
              case 'tl':
                _cropRect = Rect.fromLTRB(
                  _cropRect.left + details.delta.dx,
                  _cropRect.top + details.delta.dy,
                  _cropRect.right,
                  _cropRect.bottom,
                );
                break;
              case 'tr':
                _cropRect = Rect.fromLTRB(
                  _cropRect.left,
                  _cropRect.top + details.delta.dy,
                  _cropRect.right + details.delta.dx,
                  _cropRect.bottom,
                );
                break;
              case 'bl':
                _cropRect = Rect.fromLTRB(
                  _cropRect.left + details.delta.dx,
                  _cropRect.top,
                  _cropRect.right,
                  _cropRect.bottom + details.delta.dy,
                );
                break;
              case 'br':
                _cropRect = Rect.fromLTRB(
                  _cropRect.left,
                  _cropRect.top,
                  _cropRect.right + details.delta.dx,
                  _cropRect.bottom + details.delta.dy,
                );
                break;
            }

            // Keep aspect ratio 1:1
            final size = _cropRect.width.abs();
            _cropRect = Rect.fromLTWH(
              _cropRect.left,
              _cropRect.top,
              size,
              size,
            );
          });
        },
        child: Container(
          width: 20,
          height: 20,
          decoration: BoxDecoration(
            color: AppColors.light,
            border: Border.all(color: AppColors.dark, width: 2),
            shape: BoxShape.circle,
          ),
        ),
      ),
    );
  }

  void _onScaleStart(ScaleStartDetails details) {
    _lastFocalPoint = details.localFocalPoint;
    _lastScale = _scale;
  }

  void _onScaleUpdate(ScaleUpdateDetails details) {
    setState(() {
      // Update scale
      if (details.scale != 1.0) {
        _scale = (_lastScale! * details.scale).clamp(0.5, 3.0);
      }

      // Update position
      final delta = details.localFocalPoint - _lastFocalPoint!;
      _offset = _offset + delta;
      _lastFocalPoint = details.localFocalPoint;
    });
  }

  void _onScaleEnd(ScaleEndDetails details) {
    _lastFocalPoint = null;
    _lastScale = null;
  }

  Future<void> _cropImage() async {
    // TODO: Implement actual crop logic
    // For now, return original image
    if (_imageBytes != null) {
      widget.onCropped(_imageBytes!);
      Navigator.of(context).pop();
    }
  }
}

/// Painter for the image
class ImageCropPainter extends CustomPainter {
  final ui.Image image;
  final double scale;
  final Offset offset;
  final Rect cropRect;

  ImageCropPainter({
    required this.image,
    required this.scale,
    required this.offset,
    required this.cropRect,
  });

  @override
  void paint(Canvas canvas, Size size) {
    final paint = Paint();

    // Draw image with transform
    canvas.save();
    canvas.translate(size.width / 2 + offset.dx, size.height / 2 + offset.dy);
    canvas.scale(scale);
    canvas.drawImage(image, Offset(-image.width / 2, -image.height / 2), paint);
    canvas.restore();
  }

  @override
  bool shouldRepaint(ImageCropPainter oldDelegate) {
    return oldDelegate.scale != scale ||
        oldDelegate.offset != offset ||
        oldDelegate.cropRect != cropRect;
  }
}

/// Painter for crop overlay
class CropOverlayPainter extends CustomPainter {
  final Rect cropRect;

  CropOverlayPainter({required this.cropRect});

  @override
  void paint(Canvas canvas, Size size) {
    final paint = Paint()
      ..color = Colors.black54
      ..style = PaintingStyle.fill;

    // Draw dark overlay except crop area
    final path = Path()
      ..addRect(Rect.fromLTWH(0, 0, size.width, size.height))
      ..addRect(cropRect)
      ..fillType = PathFillType.evenOdd;

    canvas.drawPath(path, paint);

    // Draw crop border
    final borderPaint = Paint()
      ..color = AppColors.light
      ..style = PaintingStyle.stroke
      ..strokeWidth = 2;

    canvas.drawRect(cropRect, borderPaint);

    // Draw grid
    final gridPaint = Paint()
      ..color = Colors.white30
      ..style = PaintingStyle.stroke
      ..strokeWidth = 1;

    final thirdWidth = cropRect.width / 3;
    final thirdHeight = cropRect.height / 3;

    // Vertical lines
    canvas.drawLine(
      Offset(cropRect.left + thirdWidth, cropRect.top),
      Offset(cropRect.left + thirdWidth, cropRect.bottom),
      gridPaint,
    );
    canvas.drawLine(
      Offset(cropRect.left + thirdWidth * 2, cropRect.top),
      Offset(cropRect.left + thirdWidth * 2, cropRect.bottom),
      gridPaint,
    );

    // Horizontal lines
    canvas.drawLine(
      Offset(cropRect.left, cropRect.top + thirdHeight),
      Offset(cropRect.right, cropRect.top + thirdHeight),
      gridPaint,
    );
    canvas.drawLine(
      Offset(cropRect.left, cropRect.top + thirdHeight * 2),
      Offset(cropRect.right, cropRect.top + thirdHeight * 2),
      gridPaint,
    );
  }

  @override
  bool shouldRepaint(CropOverlayPainter oldDelegate) {
    return oldDelegate.cropRect != cropRect;
  }
}
