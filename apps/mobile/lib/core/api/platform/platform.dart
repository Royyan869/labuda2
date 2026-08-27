/// Platform detection abstract interface
/// Implementasi berbeda untuk mobile vs web
abstract class PlatformDetector {
  bool get isIOS;
  bool get isAndroid;
}
