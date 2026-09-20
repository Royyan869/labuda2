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
  // Report entity displayState computation (derived from Case + Decision)
  // ===========================================================================
  group('Report entity displayState derivation', () {
    test('no Case projection -> ReportDisplayState.submitted', () {
      final report = _buildReport();
      expect(report.displayState, ReportDisplayState.submitted);
    });

    test('Case open + no Decision -> ReportDisplayState.underReview', () {
      final report = _buildReport(
        caseProjection: ReportCaseProjection(
          id: 'case-1',
          status: 'open',
          createdAt: _testDate,
        ),
      );
      expect(report.displayState, ReportDisplayState.underReview);
    });

    test('Case resolved + no_violation -> ReportDisplayState.reviewedNoViolation', () {
      final report = _buildReport(
        caseProjection: ReportCaseProjection(
          id: 'case-1',
          status: 'resolved',
          createdAt: _testDate,
        ),
        decisionProjection: ReportDecisionProjection(
          outcome: 'no_violation',
          createdAt: _testDate,
        ),
      );
      expect(report.displayState, ReportDisplayState.reviewedNoViolation);
    });

    test('Case resolved + violation -> ReportDisplayState.reviewedViolation', () {
      final report = _buildReport(
        caseProjection: ReportCaseProjection(
          id: 'case-1',
          status: 'resolved',
          createdAt: _testDate,
        ),
        decisionProjection: ReportDecisionProjection(
          outcome: 'violation',
          createdAt: _testDate,
        ),
      );
      expect(report.displayState, ReportDisplayState.reviewedViolation);
    });
  });
}

// =============================================================================
// Helpers
// =============================================================================

final _testDate = DateTime.utc(2026, 7, 31);

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
    createdAt: _testDate,
  );
}

Report _buildReport({
  ReportCaseProjection? caseProjection,
  ReportDecisionProjection? decisionProjection,
}) {
  return Report(
    id: '00000000-0000-0000-0000-000000000001',
    reporterId: 'r1',
    subjectId: 't1',
    subjectType: ReportTargetType.content,
    reason: ReportReasonType.other,
    createdAt: _testDate,
    caseProjection: caseProjection,
    decisionProjection: decisionProjection,
  );
}
