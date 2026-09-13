import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_resource_projection.dart';

Map<String, dynamic> _liveProfileProjectionJson() => <String, dynamic>{
  'state': 'LIVE',
  'resource_type': 'profile',
  'resource_id': 'user-resource-1',
  'canonical_url': '/user/user-resource-1',
  'viewer_capabilities': {
    'can_view': true,
    'can_interact': false,
    'blocked_by_tombstone': false,
  },
  'profile': {
    'username': 'alice',
    'avatar_url': null,
    'store_name': 'Toko Alice',
    'is_seller': true,
    'lifecycle': 'active',
  },
};

void main() {
  group('resource projection contract', () {
    test('resource_projection parser parses LIVE profile projection canonically', () {
      final projection = ChatResourceProjection.fromJson(_liveProfileProjectionJson());
      expect(projection.state, ChatResourceProjectionState.live);
      expect(projection.resourceType, ChatResourceType.profile);
      expect(projection.canonicalUrl, '/user/user-resource-1');
      expect(projection.compactPreviewText, '@alice');
    });

    test('resource_projection parser rejects UNKNOWN_STATE state', () {
      expect(
        () => ChatResourceProjection.fromJson({
          'state': 'UNKNOWN_STATE',
          'resource_type': 'profile',
          'viewer_capabilities': {
            'can_view': true,
            'can_interact': false,
            'blocked_by_tombstone': false,
          },
        }),
        throwsFormatException,
      );
    });
  });
}
