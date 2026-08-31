import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/system/report/data/dto/report_dto.dart';

void main() {
  // ===========================================================================
  // ReportDto.fromJson
  // ===========================================================================
  group('ReportDto.fromJson', () {
    test('parses complete payload with optional fields', () {
      final json = {
        'id': '00000000-0000-0000-0000-000000000001',
        'reporter_id': '00000000-0000-0000-0000-000000000003',
        'subject_type': 'content',
        'subject_id': '00000000-0000-0000-0000-000000000002',
        'reason_code': 'scam_or_fraud',
        'reason_note': 'repeated scam posts',
        'created_at': '2026-07-31T10:30:00Z',
      };

      final dto = ReportDto.fromJson(json);

      expect(dto.id, '00000000-0000-0000-0000-000000000001');
      expect(dto.reporterId, '00000000-0000-0000-0000-000000000003');
      expect(dto.subjectType, 'content');
      expect(dto.subjectId, '00000000-0000-0000-0000-000000000002');
      expect(dto.reasonCode, 'scam_or_fraud');
      expect(dto.reasonNote, 'repeated scam posts');
    });

    test('parses minimal payload without optional fields', () {
      final json = {
        'id': '00000000-0000-0000-0000-000000000001',
        'reporter_id': '00000000-0000-0000-0000-000000000003',
        'subject_type': 'comment',
        'subject_id': '00000000-0000-0000-0000-000000000002',
        'reason_code': 'other',
        'created_at': '2026-07-31T10:30:00Z',
      };

      final dto = ReportDto.fromJson(json);

      expect(dto.id, '00000000-0000-0000-0000-000000000001');
      expect(dto.subjectType, 'comment');
      expect(dto.reasonCode, 'other');
      expect(dto.reasonNote, isNull);
    });
  });

  // ===========================================================================
  // CreateReportRequestDto.toJson
  // ===========================================================================
  group('CreateReportRequestDto.toJson', () {
    test('serializes canonical request with reason_note', () {
      const dto = CreateReportRequestDto(
        subjectType: 'content',
        subjectId: '00000000-0000-0000-0000-000000000002',
        reasonCode: 'scam_or_fraud',
        reasonNote: 'note',
      );

      expect(dto.toJson(), {
        'subject_type': 'content',
        'subject_id': '00000000-0000-0000-0000-000000000002',
        'reason_code': 'scam_or_fraud',
        'reason_note': 'note',
      });
    });

    test('serializes canonical request without reason_note', () {
      const dto = CreateReportRequestDto(
        subjectType: 'user',
        subjectId: '00000000-0000-0000-0000-000000000002',
        reasonCode: 'harassment_or_abuse',
      );

      expect(dto.toJson(), {
        'subject_type': 'user',
        'subject_id': '00000000-0000-0000-0000-000000000002',
        'reason_code': 'harassment_or_abuse',
      });
    });
  });
}
