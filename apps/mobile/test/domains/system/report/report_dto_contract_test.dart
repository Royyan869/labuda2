import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/system/report/data/dto/report_dto.dart';

void main() {
  // ===========================================================================
  // ModerationCaseDto.fromJson
  // ===========================================================================
  group('ModerationCaseDto.fromJson', () {
    test('parses complete payload with all optional fields', () {
      final json = {
        'id': '00000000-0000-0000-0000-000000000001',
        'resource_type': 'content',
        'resource_id': '00000000-0000-0000-0000-000000000002',
        'status': 'pending',
        'reported_by': '00000000-0000-0000-0000-000000000003',
        'reason': 'spam: repeated posts',
        'created_at': '2026-07-31T10:30:00Z',
        'reviewed_by': '00000000-0000-0000-0000-000000000004',
        'decision_note': 'Content removed',
        'reviewed_at': '2026-07-31T12:00:00Z',
      };

      final dto = ModerationCaseDto.fromJson(json);

      expect(dto.id, '00000000-0000-0000-0000-000000000001');
      expect(dto.resourceType, 'content');
      expect(dto.resourceId, '00000000-0000-0000-0000-000000000002');
      expect(dto.status, 'pending');
      expect(dto.reportedBy, '00000000-0000-0000-0000-000000000003');
      expect(dto.reason, 'spam: repeated posts');
      expect(dto.decisionNote, 'Content removed');
      expect(dto.reviewedBy, '00000000-0000-0000-0000-000000000004');
      expect(dto.reviewedAt, isNotNull);
    });

    test('parses minimal payload without optional fields', () {
      final json = {
        'id': '00000000-0000-0000-0000-000000000001',
        'resource_type': 'comment',
        'resource_id': '00000000-0000-0000-0000-000000000002',
        'status': 'enforced',
        'created_at': '2026-07-31T10:30:00Z',
      };

      final dto = ModerationCaseDto.fromJson(json);

      expect(dto.id, '00000000-0000-0000-0000-000000000001');
      expect(dto.resourceType, 'comment');
      expect(dto.status, 'enforced');
      expect(dto.reportedBy, isNull);
      expect(dto.reason, isNull);
      expect(dto.reviewedBy, isNull);
      expect(dto.decisionNote, isNull);
      expect(dto.reviewedAt, isNull);
    });

    test('parses fromCreateJson with case_id key', () {
      final json = {
        'case_id': '00000000-0000-0000-0000-000000000001',
        'resource_type': 'content',
        'resource_id': '00000000-0000-0000-0000-000000000002',
        'status': 'pending',
        'created_at': '2026-07-31T10:30:00Z',
      };

      final dto = ModerationCaseDto.fromCreateJson(json);

      expect(dto.id, '00000000-0000-0000-0000-000000000001');
      expect(dto.resourceType, 'content');
      expect(dto.status, 'pending');
    });
  });
}
