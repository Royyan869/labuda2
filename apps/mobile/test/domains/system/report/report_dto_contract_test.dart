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

  // ===========================================================================
  // PagedModerationCases.fromDataJson
  // ===========================================================================
  group('PagedModerationCases.fromDataJson', () {
    test('parses valid non-empty payload', () {
      final json = {
        'cases': [
          {
            'id': '00000000-0000-0000-0000-000000000001',
            'resource_type': 'content',
            'resource_id': '00000000-0000-0000-0000-000000000002',
            'status': 'pending',
            'created_at': '2026-07-31T10:30:00Z',
          },
        ],
        'page': 1,
        'limit': 20,
        'count': 25,
      };

      final paged = PagedModerationCases.fromDataJson(json);

      expect(paged.cases, hasLength(1));
      expect(paged.page, 1);
      expect(paged.limit, 20);
      expect(paged.count, 25);
      expect(paged.hasMore, isTrue,
          reason: '1 < 25 → hasMore must be true');
    });

    test('parses valid explicit empty payload', () {
      final json = {
        'cases': [],
        'page': 1,
        'limit': 20,
        'count': 0,
      };

      final paged = PagedModerationCases.fromDataJson(json);

      expect(paged.cases, isEmpty);
      expect(paged.page, 1);
      expect(paged.limit, 20);
      expect(paged.count, 0);
      expect(paged.hasMore, isFalse);
    });

    test('hasMore false when all items on current page (last page)', () {
      final json = {
        'cases': List.generate(20, (_) => {
          'id': '00000000-0000-0000-0000-000000000001',
          'resource_type': 'content',
          'resource_id': '00000000-0000-0000-0000-000000000002',
          'status': 'pending',
          'created_at': '2026-07-31T10:30:00Z',
        }),
        'page': 1,
        'limit': 20,
        'count': 20,
      };

      final paged = PagedModerationCases.fromDataJson(json);

      expect(paged.cases, hasLength(20));
      expect(paged.count, 20);
      expect(paged.hasMore, isFalse,
          reason: '20 items, total=20 → hasMore must be false');
    });

    test('throws on missing cases key', () {
      final json = {
        'page': 1,
        'limit': 20,
        'count': 0,
        // 'cases' is missing
      };

      expect(
        () => PagedModerationCases.fromDataJson(json),
        throwsA(isA<FormatException>()),
      );
    });

    test('throws on null cases', () {
      final json = {
        'cases': null,
        'page': 1,
        'limit': 20,
        'count': 0,
      };

      expect(
        () => PagedModerationCases.fromDataJson(json),
        throwsA(isA<FormatException>()),
      );
    });

    test('throws on wrong cases type', () {
      final json = {
        'cases': 'not_a_list',
        'page': 1,
        'limit': 20,
        'count': 0,
      };

      expect(
        () => PagedModerationCases.fromDataJson(json),
        throwsA(isA<FormatException>()),
      );
    });

    test('throws on missing page', () {
      final json = {
        'cases': [],
        'limit': 20,
        'count': 0,
      };

      expect(
        () => PagedModerationCases.fromDataJson(json),
        throwsA(isA<FormatException>()),
      );
    });

    test('throws on missing limit', () {
      final json = {
        'cases': [],
        'page': 1,
        'count': 0,
      };

      expect(
        () => PagedModerationCases.fromDataJson(json),
        throwsA(isA<FormatException>()),
      );
    });

    test('throws on missing count', () {
      final json = {
        'cases': [],
        'page': 1,
        'limit': 20,
      };

      expect(
        () => PagedModerationCases.fromDataJson(json),
        throwsA(isA<FormatException>()),
      );
    });

    test('throws on wrong page type (string)', () {
      final json = {
        'cases': [],
        'page': '1',
        'limit': 20,
        'count': 0,
      };

      expect(
        () => PagedModerationCases.fromDataJson(json),
        throwsA(isA<FormatException>()),
      );
    });

    test('throws on wrong count type (string)', () {
      final json = {
        'cases': [],
        'page': 1,
        'limit': 20,
        'count': '0',
      };

      expect(
        () => PagedModerationCases.fromDataJson(json),
        throwsA(isA<FormatException>()),
      );
    });
  });
}
