import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

/// CHAT MEDIA TILE CONTRACT — no crop, tap opens fullscreen.
///
/// - Tiles render `BoxFit.contain` (fins/patterns never cut), never `cover`.
/// - Tapping a tile opens the canonical `MediaViewerWidget` at the index.
/// - Video tiles never reach the image decoder (play badge instead).
void main() {
  group('chat media tile', () {
    final source = File(
      'lib/domains/chat/chat/presentation/widgets/message_bubble.dart',
    ).readAsStringSync();

    test('image tile uses contain, never cover', () {
      expect(source.contains('BoxFit.contain'), isTrue);
      expect(
        RegExp(r'fit:\s*BoxFit\.cover').hasMatch(source),
        isFalse,
        reason: 'cover crops koi fins — forbidden on chat media tiles',
      );
    });

    test('tile tap opens the canonical fullscreen viewer', () {
      expect(source.contains('MediaViewerWidget('), isTrue);
      expect(source.contains('_openMediaViewer('), isTrue);
      expect(source.contains('initialIndex:'), isTrue);
    });

    test('video tile keeps the play badge and maps to video type', () {
      expect(source.contains('Icons.play_circle_outline'), isTrue);
      expect(source.contains('MediaType.video'), isTrue);
      expect(source.contains('MediaType.image'), isTrue);
    });
  });
}
