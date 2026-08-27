import 'platform.dart';

/// Platform detector for web
/// Default to Android configuration (no iOS-specific URL differences on web)
class PlatformDetectorImpl implements PlatformDetector {
  @override
  bool get isIOS => false;

  @override
  bool get isAndroid => true;
}

final platformDetector = PlatformDetectorImpl();
