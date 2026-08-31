import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/system/report/data/dto/report_dto.dart';
import 'package:labuda/domains/system/report/data/mappers/report_mapper.dart';
import 'package:labuda/domains/system/report/domain/entities/report.dart';

void main() {
  // ===========================================================================
  // Subject type mapping — canonical targets only
  // ===========================================================================
  group('ReportMapper subject type mapping', () {
    final cases = [
      ('content', ReportTargetType.content),
      ('comment', ReportTargetType.comment),
      ('user', ReportTargetType.user),
      ('for_sale', ReportTargetType.forSale),
      ('auction', ReportTargetType.auction),
    ];

    for (final (backendValue, expectedType) in cases) {
      test('"$backendValue" maps to ${expectedType.name}', () {
        final dto = _buildDto(subjectType: backendValue);
        final report = ReportMapper.toEntity(dto);
        expect(
          report.subjectType,
          expectedType,
          reason:
              'Subject type "$backendValue" must map to ${expectedType.name}',
        );
      });
    }
  });

  // ===========================================================================
  // Reason code mapping — locked taxonomy
  // ===========================================================================
  group('ReportMapper reason code mapping', () {
    final cases = [
      ('scam_or_fraud', ReportReasonType.scamOrFraud),
      ('prohibited_content', ReportReasonType.prohibitedContent),
      ('harassment_or_abuse', ReportReasonType.harassmentOrAbuse),
      ('impersonation', ReportReasonType.impersonation),
      ('misleading_information', ReportReasonType.misleadingInformation),
      ('commerce_violation', ReportReasonType.commerceViolation),
      ('other', ReportReasonType.other),
    ];

    for (final (backendValue, expectedReason) in cases) {
      test('"$backendValue" maps to ${expectedReason.name}', () {
        final dto = _buildDto(reasonCode: backendValue);
        final report = ReportMapper.toEntity(dto);
        expect(
          report.reason,
          expectedReason,
          reason: 'Reason code "$backendValue" must map to ${expectedReason.name}',
        );
      });
    }
  });

  // ===========================================================================
  // toCreateRequestDto — canonical request shape
  // ===========================================================================
  group('ReportMapper.toCreateRequestDto', () {
    test('maps request to canonical subject_type/subject_id/reason_code', () {
      final request = CreateReportRequest(
        subjectId: 'target-1',
        subjectType: ReportTargetType.content,
        reason: ReportReasonType.scamOrFraud,
        description: 'detailed scam description',
      );

      final dto = ReportMapper.toCreateRequestDto(request);

      expect(dto.subjectType, 'content');
      expect(dto.subjectId, 'target-1');
      expect(dto.reasonCode, 'scam_or_fraud');
      expect(dto.reasonNote, 'detailed scam description');
    });

    test('maps request without description (reason_note null)', () {
      final request = CreateReportRequest(
        subjectId: 'target-1',
        subjectType: ReportTargetType.user,
        reason: ReportReasonType.harassmentOrAbuse,
      );

      final dto = ReportMapper.toCreateRequestDto(request);

      expect(dto.subjectType, 'user');
      expect(dto.subjectId, 'target-1');
      expect(dto.reasonCode, 'harassment_or_abuse');
      expect(dto.reasonNote, isNull);
    });
  });

  // ===========================================================================
  // Report entity computed properties
  // ===========================================================================
  group('Report entity', () {
    test('isResolved includes resolved, approved, rejected', () {
      final pending = _buildReport(status: ReportStatus.pending);
      final approved = _buildReport(status: ReportStatus.approved);
      final rejected = _buildReport(status: ReportStatus.rejected);
      final resolved = _buildReport(status: ReportStatus.resolved);

      expect(pending.isResolved, isFalse);
      expect(approved.isResolved, isTrue);
      expect(rejected.isResolved, isTrue);
      expect(resolved.isResolved, isTrue);
    });

    test('canBeReviewed only for pending/underReview', () {
      final pending = _buildReport(status: ReportStatus.pending);
      final underReview = _buildReport(status: ReportStatus.underReview);
      final resolved = _buildReport(status: ReportStatus.resolved);
      final approved = _buildReport(status: ReportStatus.approved);

      expect(pending.canBeReviewed, isTrue);
      expect(underReview.canBeReviewed, isTrue);
      expect(resolved.canBeReviewed, isFalse);
      expect(approved.canBeReviewed, isFalse);
    });
  });
}

// =============================================================================
// Helpers
// =============================================================================

ReportDto _buildDto({
  String subjectType = 'content',
  String reasonCode = 'other',
}) {
  return ReportDto(
    id: '00000000-0000-0000-0000-000000000001',
    reporterId: '00000000-0000-0000-0000-000000000003',
    subjectType: subjectType,
    subjectId: '00000000-0000-0000-0000-000000000002',
    reasonCode: reasonCode,
    createdAt: DateTime(2026, 7, 31),
  );
}

Report _buildReport({required ReportStatus status}) {
  return Report(
    id: '00000000-0000-0000-0000-000000000001',
    reporterId: 'r1',
    subjectId: 't1',
    subjectType: ReportTargetType.content,
    reason: ReportReasonType.other,
    status: status,
    createdAt: DateTime(2026, 7, 31),
  );
}
