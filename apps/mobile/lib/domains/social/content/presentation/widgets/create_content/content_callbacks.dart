import 'package:flutter/material.dart';

/// Helper class to generate callbacks for post screen
class ContentCallbacks {
  /// Detect hashtags from text
  static List<String> detectHashtags(String text) {
    final RegExp hashtagRegex = RegExp(r'#[a-zA-Z0-9_]+');
    final matches = hashtagRegex.allMatches(text);
    return matches.map((match) => match.group(0)!.substring(1)).toList();
  }

  /// Create hashtag change callback
  static void Function(String) createHashtagDetector({
    required Function(List<String>) onHashtagsChanged,
    required VoidCallback onMarkUnsaved,
  }) {
    return (value) {
      onMarkUnsaved();
      final hashtags = detectHashtags(value);
      onHashtagsChanged(hashtags);
    };
  }

  /// Create navigation callback with unsaved changes check
  static void Function() createNavigateBack({
    required Function() getCanPop,
    required VoidCallback pop,
    required VoidCallback goHome,
  }) {
    return () {
      if (getCanPop()) {
        pop();
      } else {
        goHome();
      }
    };
  }

  /// Calculate total media count
  static int getTotalMediaCount(int images, int videos) {
    return images + videos;
  }

  /// Calculate total upload steps based on media presence
  static int getUploadSteps(int mediaCount) {
    return mediaCount > 0 ? 3 : 2;
  }
}
