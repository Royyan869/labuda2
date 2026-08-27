/// Contract tests for S3UploadResult model.
///
/// Verifies that S3UploadResult holds key and url as independent fields,
/// and that the expected key format conventions are testable at the model layer.
///
/// No real S3 calls are made — these are pure value-object unit tests.
library;

import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';

void main() {
  group('S3UploadResult model', () {
    test('key and url are stored as independent fields', () {
      const result = S3UploadResult(
        key: 'images/1749600000000_photo.jpg',
        url: 'https://cdn.example.com/images/1749600000000_photo.jpg',
      );

      expect(result.key, 'images/1749600000000_photo.jpg');
      expect(
        result.url,
        'https://cdn.example.com/images/1749600000000_photo.jpg',
      );
      expect(result.key, isNot(equals(result.url)));
    });

    test('image key follows images/ prefix without scheme', () {
      const result = S3UploadResult(
        key: 'images/1749600000000_photo.jpg',
        url: 'https://cdn.example.com/images/1749600000000_photo.jpg',
      );

      expect(result.key.startsWith('images/'), isTrue);
      expect(
        result.key.contains('://'),
        isFalse,
        reason: 'key is a path segment — no https:// scheme',
      );
    });

    test('video key follows videos/ prefix without scheme', () {
      const result = S3UploadResult(
        key: 'videos/1749600000000_clip.mp4',
        url: 'https://cdn.example.com/videos/1749600000000_clip.mp4',
      );

      expect(result.key.startsWith('videos/'), isTrue);
      expect(
        result.key.contains('://'),
        isFalse,
        reason: 'key is a path segment — no https:// scheme',
      );
    });

    test('url carries https scheme for display', () {
      const result = S3UploadResult(
        key: 'images/1749600000000_photo.jpg',
        url: 'https://cdn.example.com/images/1749600000000_photo.jpg',
      );

      expect(result.url.startsWith('https://'), isTrue);
    });

    test('key does not duplicate CDN host prefix from url', () {
      const cdnHost = 'cdn.example.com';
      const result = S3UploadResult(
        key: 'images/1749600000000_photo.jpg',
        url: 'https://$cdnHost/images/1749600000000_photo.jpg',
      );

      expect(
        result.key.contains(cdnHost),
        isFalse,
        reason: 'key must not contain the CDN host',
      );
    });

    test(
      'two results with same filename but different timestamps are distinct',
      () {
        const r1 = S3UploadResult(
          key: 'images/1749600000000_photo.jpg',
          url: 'https://cdn.example.com/images/1749600000000_photo.jpg',
        );
        const r2 = S3UploadResult(
          key: 'images/1749600000001_photo.jpg',
          url: 'https://cdn.example.com/images/1749600000001_photo.jpg',
        );

        expect(r1.key, isNot(equals(r2.key)));
        expect(r1.url, isNot(equals(r2.url)));
      },
    );
  });
}
