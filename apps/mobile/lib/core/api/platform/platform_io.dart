import 'dart:io' show Platform;

import 'platform.dart';

/// Platform detector for mobile (Android/iOS)
class PlatformDetectorImpl implements PlatformDetector {
  @override
  bool get isIOS => Platform.isIOS;

  @override
  bool get isAndroid => Platform.isAndroid;
}

final platformDetector = PlatformDetectorImpl();
