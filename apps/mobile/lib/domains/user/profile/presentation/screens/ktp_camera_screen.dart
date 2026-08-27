import 'package:camera/camera.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:permission_handler/permission_handler.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/widgets/app_snackbar.dart';

/// KTP Camera Screen - Capture KTP with landscape orientation and frame overlay
///
/// Features:
/// - Forces landscape orientation
/// - KTP frame overlay (1.585:1 ratio)
/// - Back camera by default
/// - Flash toggle
/// - Capture and return image path
class KtpCameraScreen extends StatefulWidget {
  const KtpCameraScreen({super.key});

  @override
  State<KtpCameraScreen> createState() => _KtpCameraScreenState();
}

class _KtpCameraScreenState extends State<KtpCameraScreen>
    with WidgetsBindingObserver {
  CameraController? _controller;
  bool _isCameraInitialized = false;
  bool _isFlashOn = false;
  String? _errorMessage;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _initializeCamera();
    _setLandscapeOrientation();
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    _controller?.dispose();
    _resetOrientation();
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    final CameraController? cameraController = _controller;
    if (cameraController == null || !cameraController.value.isInitialized) {
      return;
    }

    if (state == AppLifecycleState.inactive) {
      cameraController.dispose();
    } else if (state == AppLifecycleState.resumed) {
      _initializeCamera();
    }
  }

  /// Force landscape orientation for KTP capture
  Future<void> _setLandscapeOrientation() async {
    await SystemChrome.setPreferredOrientations([
      DeviceOrientation.landscapeRight,
      DeviceOrientation.landscapeLeft,
    ]);
  }

  /// Reset orientation when leaving screen
  Future<void> _resetOrientation() async {
    await SystemChrome.setPreferredOrientations([
      DeviceOrientation.portraitUp,
      DeviceOrientation.portraitDown,
      DeviceOrientation.landscapeRight,
      DeviceOrientation.landscapeLeft,
    ]);
  }

  /// Initialize camera with back camera
  Future<void> _initializeCamera() async {
    try {
      // Request camera permission
      final status = await Permission.camera.request();
      if (!status.isGranted) {
        setState(() {
          _errorMessage =
              'Camera access denied. Please grant camera permission in settings.';
        });
        return;
      }

      final cameras = await availableCameras();
      if (cameras.isEmpty) {
        setState(() {
          _errorMessage = 'No camera available';
        });
        return;
      }

      // Find back camera
      final backCamera = cameras.firstWhere(
        (camera) => camera.lensDirection == CameraLensDirection.back,
        orElse: () => cameras.first,
      );

      _controller = CameraController(
        backCamera,
        ResolutionPreset.high,
        enableAudio: false,
        imageFormatGroup: ImageFormatGroup.jpeg,
      );

      await _controller!.initialize();

      if (mounted) {
        setState(() {
          _isCameraInitialized = true;
        });
      }
    } catch (e) {
      setState(() {
        _errorMessage = 'Failed to initialize camera: $e';
      });
    }
  }

  /// Toggle flash
  Future<void> _toggleFlash() async {
    if (_controller == null) return;

    try {
      if (_isFlashOn) {
        await _controller!.setFlashMode(FlashMode.off);
      } else {
        await _controller!.setFlashMode(FlashMode.torch);
      }
      setState(() {
        _isFlashOn = !_isFlashOn;
      });
    } catch (e) {
      // Flash not supported, ignore
    }
  }

  /// Capture photo
  Future<void> _capturePhoto() async {
    if (_controller == null || !_controller!.value.isInitialized) {
      return;
    }

    try {
      final image = await _controller!.takePicture();
      if (mounted) {
        // Return image path (simplified - no OCR)
        Navigator.of(context).pop(image.path);
      }
    } catch (e) {
      if (mounted) {
        AppSnackBar.showError(context, 'Gagal mengambil foto. Coba lagi.');
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.black,
      body: SafeArea(
        child: Stack(
          children: [
            // Camera Preview
            if (_isCameraInitialized && _controller != null)
              SizedBox.expand(child: CameraPreview(_controller!)),

            // Error Message
            if (_errorMessage != null)
              Center(
                child: Padding(
                  padding: const EdgeInsets.all(16.0),
                  child: Text(
                    _errorMessage!,
                    style: const TextStyle(color: Colors.white, fontSize: 16),
                    textAlign: TextAlign.center,
                  ),
                ),
              ),

            // Loading
            if (!_isCameraInitialized && _errorMessage == null)
              const Center(
                child: CircularProgressIndicator(color: Colors.white),
              ),

            // KTP Frame Overlay
            if (_isCameraInitialized)
              Center(
                child: AspectRatio(
                  aspectRatio: 1.585, // KTP ratio 85.6mm x 53.98mm
                  child: Container(
                    margin: const EdgeInsets.symmetric(
                      horizontal: 20,
                    ), // Lebih lebar (dari 40 ke 20)
                    decoration: BoxDecoration(
                      border: Border.all(color: Colors.white, width: 2),
                      borderRadius: BorderRadius.circular(12),
                    ),
                    child: Stack(
                      children: [
                        // Corner guides
                        ...List.generate(4, (index) {
                          final isTop = index < 2;
                          final isLeft = index % 2 == 0;
                          return Positioned(
                            top: isTop ? 8 : null,
                            bottom: isTop ? null : 8,
                            left: isLeft ? 8 : null,
                            right: isLeft ? null : 8,
                            child: Container(
                              width: 24,
                              height: 24,
                              decoration: BoxDecoration(
                                border: Border(
                                  top: isTop
                                      ? const BorderSide(
                                          color: AppColors.primaryRed,
                                          width: 3,
                                        )
                                      : BorderSide.none,
                                  bottom: isTop
                                      ? BorderSide.none
                                      : const BorderSide(
                                          color: AppColors.primaryRed,
                                          width: 3,
                                        ),
                                  left: isLeft
                                      ? const BorderSide(
                                          color: AppColors.primaryRed,
                                          width: 3,
                                        )
                                      : BorderSide.none,
                                  right: isLeft
                                      ? BorderSide.none
                                      : const BorderSide(
                                          color: AppColors.primaryRed,
                                          width: 3,
                                        ),
                                ),
                              ),
                            ),
                          );
                        }),
                      ],
                    ),
                  ),
                ),
              ),

            // Top Bar - Close & Flash
            Positioned(
              top: 0,
              left: 0,
              right: 0,
              child: Container(
                padding: const EdgeInsets.all(16),
                decoration: const BoxDecoration(
                  gradient: LinearGradient(
                    begin: Alignment.topCenter,
                    end: Alignment.bottomCenter,
                    colors: [
                      Color(0x99000000), // Black with 60% opacity
                      Colors.transparent,
                    ],
                  ),
                ),
                child: Row(
                  mainAxisAlignment: MainAxisAlignment.spaceBetween,
                  children: [
                    // Close Button
                    IconButton(
                      onPressed: () => Navigator.of(context).pop(),
                      icon: const Icon(Icons.close),
                      color: Colors.white,
                      iconSize: 28,
                    ),

                    // Flash Toggle
                    IconButton(
                      onPressed: _toggleFlash,
                      icon: Icon(_isFlashOn ? Icons.flash_on : Icons.flash_off),
                      color: Colors.white,
                      iconSize: 28,
                    ),
                  ],
                ),
              ),
            ),

            // Bottom Bar - Instructions & Capture
            Positioned(
              bottom: 0,
              left: 0,
              right: 0,
              child: Container(
                padding: const EdgeInsets.all(24),
                decoration: const BoxDecoration(
                  gradient: LinearGradient(
                    begin: Alignment.bottomCenter,
                    end: Alignment.topCenter,
                    colors: [
                      Color(0xCC000000), // Black with 80% opacity
                      Colors.transparent,
                    ],
                  ),
                ),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    // Instructions
                    const Text(
                      'Posisikan KTP di dalam frame',
                      style: TextStyle(
                        color: Colors.white,
                        fontSize: 16,
                        fontWeight: FontWeight.w500,
                      ),
                    ),
                    const SizedBox(height: 4),
                    const Text(
                      'Pastikan semua bagian KTP terlihat jelas',
                      style: TextStyle(
                        color: Color(0xB3FFFFFF), // White with 70% opacity
                        fontSize: 14,
                      ),
                    ),
                    const SizedBox(height: 24),

                    // Capture Button
                    GestureDetector(
                      onTap: _capturePhoto,
                      child: Container(
                        width: 72,
                        height: 72,
                        decoration: BoxDecoration(
                          shape: BoxShape.circle,
                          border: Border.all(color: Colors.white, width: 4),
                        ),
                        child: Center(
                          child: Container(
                            width: 56,
                            height: 56,
                            decoration: const BoxDecoration(
                              color: Colors.white,
                              shape: BoxShape.circle,
                            ),
                          ),
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
