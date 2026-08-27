/// Regression-lock tests for Pass 1D-F3: presenceEnabledProvider truthfulness.
///
/// Backend has zero presence endpoints. presenceProvider.isUserOnline() always
/// returns false. presenceEnabledProvider must be false to match reality.
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_providers.dart';

void main() {
  group('presenceEnabledProvider', () {
    test('is false — backend has no presence endpoints', () {
      final container = ProviderContainer();
      addTearDown(container.dispose);
      expect(container.read(presenceEnabledProvider), isFalse);
    });
  });

  group('typingIndicatorEnabledProvider', () {
    test('is false — backend has no typing indicator endpoints', () {
      final container = ProviderContainer();
      addTearDown(container.dispose);
      expect(container.read(typingIndicatorEnabledProvider), isFalse);
    });
  });

  group('readReceiptEnabledProvider', () {
    test('is true — mark-as-read endpoint exists', () {
      final container = ProviderContainer();
      addTearDown(container.dispose);
      expect(container.read(readReceiptEnabledProvider), isTrue);
    });
  });
}
