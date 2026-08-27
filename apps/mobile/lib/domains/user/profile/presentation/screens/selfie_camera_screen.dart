import 'package:camera/camera.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:permission_handler/permission_handler.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/widgets/app_snackbar.dart';

/// Selfie Camera Screen - Capture selfie with KTP in portrait orientation
///
/// Features:
/// - Forces portrait orientation
/// - Front camera (selfie mode)
/// - Circular face guide overlay
/// - KTP holding guide
/// - Capture and return image path
class SelfieCameraScreen extends StatefulWidget {
  const SelfieCameraScreen({super.key});

  @override
  State<SelfieCameraScreen> createState() => _SelfieCameraScreenState();
}

class _SelfieCameraScreenState extends State<SelfieCameraScreen>
    with WidgetsBindingObserver {
  CameraController? _controller;
  bool _isCameraInitialized = false;
  String? _errorMessage;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _initializeCamera();
    _setPortraitOrientation();
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

  /// Force portrait orientation for selfie capture
  Future<void> _setPortraitOrientation() async {
    await SystemChrome.setPreferredOrientations([DeviceOrientation.portraitUp]);
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

  /// Initialize camera with front camera
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

      // Find front camera
      final frontCamera = cameras.firstWhere(
        (camera) => camera.lensDirection == CameraLensDirection.front,
        orElse: () => cameras.first,
      );

      _controller = CameraController(
        frontCamera,
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

  /// Capture photo
  Future<void> _capturePhoto() async {
    if (_controller == null || !_controller!.value.isInitialized) {
      return;
    }

    try {
      final image = await _controller!.takePicture();
      if (mounted) {
        // Return image path to wizard
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

            // Selfie Frame Overlay
            if (_isCameraInitialized)
              Center(
                child: AspectRatio(
                  aspectRatio: 3 / 4, // Portrait ratio
                  child: ConstrainedBox(
                    constraints: const BoxConstraints(maxWidth: 320),
                    child: Container(
                      margin: const EdgeInsets.symmetric(
                        horizontal: 32,
                        vertical: 60,
                      ),
                      child: Stack(
                        children: [
                          // Circular Face Guide
                          Center(
                            child: AspectRatio(
                              aspectRatio: 1,
                              child: Container(
                                margin: const EdgeInsets.all(40),
                                decoration: BoxDecoration(
                                  shape: BoxShape.circle,
                                  border: Border.all(
                                    color: Colors.white,
                                    width: 2,
                                  ),
                                ),
                                child: Stack(
                                  children: [
                                    // Corner guides for face circle
                                    ...List.generate(4, (index) {
                                      final isTop = index < 2;
                                      final isLeft = index % 2 == 0;
                                      return Positioned(
                                        top: isTop ? 8 : null,
                                        bottom: isTop ? null : 8,
                                        left: isLeft ? 8 : null,
                                        right: isLeft ? null : 8,
                                        child: Container(
                                          width: 20,
                                          height: 20,
                                          decoration: BoxDecoration(
                                            border: Border(
                                              top: isTop
                                                  ? const BorderSide(
                                                      color:
                                                          AppColors.primaryRed,
                                                      width: 3,
                                                    )
                                                  : BorderSide.none,
                                              bottom: isTop
                                                  ? BorderSide.none
                                                  : const BorderSide(
                                                      color:
                                                          AppColors.primaryRed,
                                                      width: 3,
                                                    ),
                                              left: isLeft
                                                  ? const BorderSide(
                                                      color:
                                                          AppColors.primaryRed,
                                                      width: 3,
                                                    )
                                                  : BorderSide.none,
                                              right: isLeft
                                                  ? BorderSide.none
                                                  : const BorderSide(
                                                      color:
                                                          AppColors.primaryRed,
                                                      width: 3,
                                                    ),
                                            ),
                                            borderRadius: BorderRadius.circular(
                                              10,
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

                          // KTP Holding Guide (bottom right area)
                          Positioned(
                            bottom: 20,
                            right: 20,
                            child: Container(
                              padding: const EdgeInsets.symmetric(
                                horizontal: 12,
                                vertical: 8,
                              ),
                              decoration: BoxDecoration(
                                color: const Color(0xCC000000),
                                borderRadius: BorderRadius.circular(8),
                                border: Border.all(
                                  color: AppColors.primaryRed,
                                  width: 2,
                                ),
                              ),
                              child: const Row(
                                mainAxisSize: MainAxisSize.min,
                                children: [
                                  Icon(
                                    Icons.credit_card,
                                    color: AppColors.primaryRed,
                                    size: 20,
                                  ),
                                  SizedBox(width: 6),
                                  Text(
                                    'Pegang KTP',
                                    style: TextStyle(
                                      color: Colors.white,
                                      fontSize: 12,
                                      fontWeight: FontWeight.w500,
                                    ),
                                  ),
                                ],
                              ),
                            ),
                          ),
                        ],
                      ),
                    ),
                  ),
                ),
              ),

            // Top Bar - Close
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
                    const SizedBox(width: 48), // Balance for centering
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
                      'Posisikan wajah di dalam lingkaran',
                      style: TextStyle(
                        color: Colors.white,
                        fontSize: 16,
                        fontWeight: FontWeight.w500,
                      ),
                    ),
                    const SizedBox(height: 4),
                    const Text(
                      'Pegang KTP di samping wajah Anda',
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
