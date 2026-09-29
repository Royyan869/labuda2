import 'dart:io';
import 'package:camera/camera.dart';
import 'package:flutter/material.dart';
import 'package:labuda/core/src/theme/app_theme.dart';

/// Custom Camera Screen
/// Supports both photo and video capture with toggle
/// Similar to WhatsApp camera experience
class CustomCameraScreen extends StatefulWidget {
  const CustomCameraScreen({super.key});

  static Future<List<String>?> show(BuildContext context) async {
    return await Navigator.of(context).push<List<String>>(
      MaterialPageRoute(
        builder: (context) => const CustomCameraScreen(),
        fullscreenDialog: true,
      ),
    );
  }

  @override
  State<CustomCameraScreen> createState() => _CustomCameraScreenState();
}

enum CameraMode { photo, video }

class _CustomCameraScreenState extends State<CustomCameraScreen>
    with WidgetsBindingObserver {
  CameraController? _controller;
  List<CameraDescription>? _cameras;
  CameraMode _mode = CameraMode.photo;
  bool _isRecording = false;
  bool _isInitializing = true;
  int _selectedCameraIndex = 0;

  // Preview state
  String? _capturedMediaPath;
  bool _isPhoto = true;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _initializeCamera();
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    _controller?.dispose();
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    final controller = _controller;
    if (controller == null || !controller.value.isInitialized) return;

    if (state == AppLifecycleState.inactive) {
      controller.dispose();
    } else if (state == AppLifecycleState.resumed) {
      _initializeCamera();
    }
  }

  Future<void> _initializeCamera() async {
    try {
      _cameras = await availableCameras();
      if (_cameras == null || _cameras!.isEmpty) {
        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            const SnackBar(
              content: Text('No cameras available'),
              duration: Duration(seconds: 4),
            ),
          );
        }
        return;
      }

      _controller = CameraController(
        _cameras![_selectedCameraIndex],
        ResolutionPreset.high,
        enableAudio: true,
      );

      await _controller!.initialize();
      if (mounted) {
        setState(() => _isInitializing = false);
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: const Text('Gagal membuka kamera. Coba lagi.'),
            duration: const Duration(seconds: 4),
          ),
        );
      }
    }
  }

  Future<void> _switchCamera() async {
    if (_cameras == null || _cameras!.length < 2) return;

    setState(() {
      _isInitializing = true;
      _selectedCameraIndex = (_selectedCameraIndex + 1) % _cameras!.length;
    });

    await _controller?.dispose();
    await _initializeCamera();
  }

  Future<void> _capturePhoto() async {
    if (_controller == null || !_controller!.value.isInitialized) return;

    try {
      final image = await _controller!.takePicture();
      if (mounted) {
        setState(() {
          _capturedMediaPath = image.path;
          _isPhoto = true;
        });
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: const Text('Gagal mengambil foto. Coba lagi.'),
            duration: const Duration(seconds: 4),
          ),
        );
      }
    }
  }

  Future<void> _startVideoRecording() async {
    if (_controller == null || !_controller!.value.isInitialized) return;

    try {
      await _controller!.startVideoRecording();
      setState(() => _isRecording = true);
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: const Text('Gagal memulai rekaman. Coba lagi.'),
            duration: const Duration(seconds: 4),
          ),
        );
      }
    }
  }

  Future<void> _stopVideoRecording() async {
    if (_controller == null || !_controller!.value.isRecordingVideo) return;

    try {
      final video = await _controller!.stopVideoRecording();
      setState(() {
        _isRecording = false;
        _capturedMediaPath = video.path;
        _isPhoto = false;
      });
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: const Text('Gagal menghentikan rekaman. Coba lagi.'),
            duration: const Duration(seconds: 4),
          ),
        );
      }
    }
  }

  void _useMedia() {
    if (_capturedMediaPath != null) {
      Navigator.of(context).pop([_capturedMediaPath!]);
    }
  }

  void _retakeMedia() {
    setState(() {
      _capturedMediaPath = null;
    });
  }

  @override
  Widget build(BuildContext context) {
    return AnimatedSwitcher(
      duration: AppMotion.fast,
      switchInCurve: Curves.easeIn,
      switchOutCurve: Curves.easeOut,
      transitionBuilder: (Widget child, Animation<double> animation) {
        return FadeTransition(opacity: animation, child: child);
      },
      child: _capturedMediaPath != null
          ? _buildPreviewScreen()
          : _buildCameraScreen(),
    );
  }

  Widget _buildCameraScreen() {
    final colorScheme = Theme.of(context).colorScheme;
    return Scaffold(
      key: const ValueKey('camera'),
      backgroundColor: colorScheme.scrim,
      body: Stack(
        children: [
          // Camera Preview
          if (_isInitializing)
            const Center(child: CircularProgressIndicator())
          else if (_controller != null && _controller!.value.isInitialized)
            SizedBox.expand(
              child: FittedBox(
                fit: BoxFit.cover,
                child: SizedBox(
                  width: _controller!.value.previewSize!.height,
                  height: _controller!.value.previewSize!.width,
                  child: CameraPreview(_controller!),
                ),
              ),
            )
          else
            Center(
              child: Text(
                'Camera not available',
                style: TextStyle(color: colorScheme.onPrimary),
              ),
            ),

          // Top Controls
          Positioned(
            top: 0,
            left: 0,
            right: 0,
            child: Container(
              padding: EdgeInsets.only(
                top: MediaQuery.of(context).padding.top + AppMetrics.p8,
                left: AppMetrics.p16,
                right: AppMetrics.p16,
                bottom: AppMetrics.p16,
              ),
              decoration: BoxDecoration(
                gradient: LinearGradient(
                  begin: Alignment.topCenter,
                  end: Alignment.bottomCenter,
                  colors: [
                    colorScheme.scrim.withValues(alpha: 0.7),
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
                    icon: Icon(
                      Icons.close,
                      color: colorScheme.onPrimary,
                      size: 28,
                    ),
                  ),

                  // Mode Toggle
                  Container(
                    padding: const EdgeInsets.all(AppMetrics.p4),
                    decoration: BoxDecoration(
                      color: colorScheme.scrim.withValues(alpha: 0.5),
                      borderRadius: BorderRadius.circular(AppShape.r20),
                    ),
                    child: Row(
                      children: [
                        _buildModeButton('PHOTO', CameraMode.photo),
                        const SizedBox(width: 4),
                        _buildModeButton('VIDEO', CameraMode.video),
                      ],
                    ),
                  ),

                  // Flip Camera Button
                  IconButton(
                    onPressed: _switchCamera,
                    icon: Icon(
                      Icons.flip_camera_ios,
                      color: colorScheme.onPrimary,
                      size: 28,
                    ),
                  ),
                ],
              ),
            ),
          ),

          // Bottom Controls
          Positioned(
            bottom: 0,
            left: 0,
            right: 0,
            child: Container(
              padding: EdgeInsets.only(
                bottom: MediaQuery.of(context).padding.bottom + AppMetrics.p16,
                top: AppMetrics.p16,
              ),
              decoration: BoxDecoration(
                gradient: LinearGradient(
                  begin: Alignment.bottomCenter,
                  end: Alignment.topCenter,
                  colors: [
                    colorScheme.scrim.withValues(alpha: 0.7),
                    Colors.transparent,
                  ],
                ),
              ),
              child: Column(
                children: [
                  // Recording Indicator
                  if (_isRecording)
                    Container(
                      margin: const EdgeInsets.only(bottom: AppMetrics.p16),
                      padding: const EdgeInsets.symmetric(
                        horizontal: AppMetrics.p16,
                        vertical: AppMetrics.p8,
                      ),
                      decoration: BoxDecoration(
                        color: colorScheme.primary,
                        borderRadius: BorderRadius.circular(AppShape.r20),
                      ),
                      child: Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Icon(
                            Icons.fiber_manual_record,
                            color: colorScheme.onPrimary,
                            size: 16,
                          ),
                          const SizedBox(width: 8),
                          Text(
                            'Recording...',
                            style: TextStyle(
                              color: colorScheme.onPrimary,
                              fontWeight: FontWeight.w600,
                            ),
                          ),
                        ],
                      ),
                    ),

                  // Capture Button
                  Center(
                    child: GestureDetector(
                      onTap: () {
                        if (_mode == CameraMode.photo) {
                          _capturePhoto();
                        } else {
                          if (_isRecording) {
                            _stopVideoRecording();
                          } else {
                            _startVideoRecording();
                          }
                        }
                      },
                      child: Container(
                        width: 80,
                        height: 80,
                        decoration: BoxDecoration(
                          shape: BoxShape.circle,
                          border: Border.all(
                            color: colorScheme.onPrimary,
                            width: 4,
                          ),
                          color: _isRecording
                              ? colorScheme.primary
                              : (_mode == CameraMode.video
                                    ? colorScheme.primary.withValues(alpha: 0.3)
                                    : Colors.transparent),
                        ),
                        child: _isRecording
                            ? Icon(
                                Icons.stop,
                                color: colorScheme.onPrimary,
                                size: 36,
                              )
                            : null,
                      ),
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

  Widget _buildModeButton(String label, CameraMode mode) {
    final isSelected = _mode == mode;
    final colorScheme = Theme.of(context).colorScheme;
    return GestureDetector(
      onTap: () => setState(() => _mode = mode),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p16, vertical: AppMetrics.p8),
        decoration: BoxDecoration(
          color: isSelected ? colorScheme.primary : Colors.transparent,
          borderRadius: BorderRadius.circular(AppShape.r16),
        ),
        child: Text(
          label,
          style: TextStyle(
            color: colorScheme.onPrimary,
            fontWeight: isSelected ? FontWeight.w600 : FontWeight.w400,
            fontSize: AppType.s13,
          ),
        ),
      ),
    );
  }

  Widget _buildPreviewScreen() {
    final colorScheme = Theme.of(context).colorScheme;
    return Scaffold(
      key: const ValueKey('preview'),
      backgroundColor: colorScheme.scrim,
      body: Stack(
        children: [
          // Media Preview
          Center(
            child: _isPhoto
                ? Image.file(File(_capturedMediaPath!), fit: BoxFit.contain)
                : Container(
                    color: colorScheme.scrim,
                    child: Center(
                      child: Icon(
                        Icons.play_circle_outline,
                        color: colorScheme.onPrimary,
                        size: 80,
                      ),
                    ),
                  ),
          ),

          // Top bar with close button
          Positioned(
            top: 0,
            left: 0,
            right: 0,
            child: Container(
              padding: EdgeInsets.only(
                top: MediaQuery.of(context).padding.top + AppMetrics.p8,
                left: AppMetrics.p16,
                right: AppMetrics.p16,
                bottom: AppMetrics.p16,
              ),
              decoration: BoxDecoration(
                gradient: LinearGradient(
                  begin: Alignment.topCenter,
                  end: Alignment.bottomCenter,
                  colors: [
                    colorScheme.scrim.withValues(alpha: 0.7),
                    Colors.transparent,
                  ],
                ),
              ),
              child: Row(
                children: [
                  IconButton(
                    onPressed: () => Navigator.of(context).pop(),
                    icon: Icon(
                      Icons.close,
                      color: colorScheme.onPrimary,
                      size: 28,
                    ),
                  ),
                ],
              ),
            ),
          ),

          // Bottom action buttons
          Positioned(
            bottom: 0,
            left: 0,
            right: 0,
            child: Container(
              padding: EdgeInsets.only(
                bottom: MediaQuery.of(context).padding.bottom + AppMetrics.p16,
                top: AppMetrics.p16,
                left: AppMetrics.p16,
                right: AppMetrics.p16,
              ),
              decoration: BoxDecoration(
                gradient: LinearGradient(
                  begin: Alignment.bottomCenter,
                  end: Alignment.topCenter,
                  colors: [
                    colorScheme.scrim.withValues(alpha: 0.7),
                    Colors.transparent,
                  ],
                ),
              ),
              child: Row(
                mainAxisAlignment: MainAxisAlignment.spaceEvenly,
                children: [
                  // Retake Button
                  Expanded(
                    child: OutlinedButton.icon(
                      onPressed: _retakeMedia,
                      icon: Icon(Icons.refresh, color: colorScheme.onPrimary),
                      label: Text(
                        'Retake',
                        style: TextStyle(
                          color: colorScheme.onPrimary,
                          fontSize: AppType.s16,
                        ),
                      ),
                      style: OutlinedButton.styleFrom(
                        side: BorderSide(
                          color: colorScheme.onPrimary,
                          width: 2,
                        ),
                        padding: const EdgeInsets.symmetric(vertical: AppMetrics.p16),
                      ),
                    ),
                  ),
                  const SizedBox(width: 16),
                  // Use Button
                  Expanded(
                    child: ElevatedButton.icon(
                      onPressed: _useMedia,
                      icon: Icon(Icons.check, color: colorScheme.onPrimary),
                      label: Text(
                        'Use',
                        style: TextStyle(
                          color: colorScheme.onPrimary,
                          fontSize: AppType.s16,
                        ),
                      ),
                      style: ElevatedButton.styleFrom(
                        padding: const EdgeInsets.symmetric(vertical: AppMetrics.p16),
                      ),
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
}
