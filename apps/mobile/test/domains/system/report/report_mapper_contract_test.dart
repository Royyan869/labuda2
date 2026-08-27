import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/system/report/data/dto/report_dto.dart';
import 'package:labuda/domains/system/report/data/mappers/report_mapper.dart';
import 'package:labuda/domains/system/report/domain/entities/report.dart';

void main() {
  // ===========================================================================
  // Resource type mapping — all 6 canonical types
  // ===========================================================================
  group('ReportMapper resource type mapping', () {
    final cases = [
      ('content', ReportTargetType.content),
      ('comment', ReportTargetType.comment),
      ('user', ReportTargetType.user),
      ('chat_message', ReportTargetType.message),
      ('fixed_price_sale', ReportTargetType.forSale),
      ('auction', ReportTargetType.auction),
    ];

    for (final (backendValue, expectedType) in cases) {
      test('"$backendValue" maps to ${expectedType.name}', () {
        final dto = _buildDto(resourceType: backendValue);
        final report = ReportMapper.toEntity(dto);
        expect(
          report.targetType,
          expectedType,
          reason:
              'Resource type "$backendValue" must map to ${expectedType.name}',
        );
      });
    }

    test('unknown resource type throws', () {
      final dto = _buildDto(resourceType: 'unknown_type');
      expect(() => ReportMapper.toEntity(dto), throwsA(isA<ArgumentError>()));
    });

    test('"removed" (legacy) throws — does not silently become content', () {
      final dto = _buildDto(resourceType: 'removed');
      expect(() => ReportMapper.toEntity(dto), throwsA(isA<ArgumentError>()));
    });
  });

  // ===========================================================================
  // Status mapping — all 4 canonical statuses
  // ===========================================================================
  group('ReportMapper status mapping', () {
    final cases = [
      ('pending', ReportStatus.pending),
      ('approved', ReportStatus.approved),
      ('rejected', ReportStatus.rejected),
      ('enforced', ReportStatus.enforced),
    ];

    for (final (backendValue, expectedStatus) in cases) {
      test('"$backendValue" maps to ${expectedStatus.name}', () {
        final dto = _buildDto(status: backendValue);
        final report = ReportMapper.toEntity(dto);
        expect(
          report.status,
          expectedStatus,
          reason: 'Status "$backendValue" must map to ${expectedStatus.name}',
        );
      });
    }

    test('unknown status throws', () {
      final dto = _buildDto(status: 'unknown_status');
      expect(() => ReportMapper.toEntity(dto), throwsA(isA<ArgumentError>()));
    });

    test('"removed" (legacy) throws — does not silently become pending', () {
      final dto = _buildDto(status: 'removed');
      expect(() => ReportMapper.toEntity(dto), throwsA(isA<ArgumentError>()));
    });

    test('"under_review" throws — no longer a valid status', () {
      final dto = _buildDto(status: 'under_review');
      expect(() => ReportMapper.toEntity(dto), throwsA(isA<ArgumentError>()));
    });
  });

  // ===========================================================================
  // Status display labels (Indonesian, owner Option A)
  // ===========================================================================
  group('ReportStatus display labels', () {
    test('pending → "Menunggu peninjauan"', () {
      expect(ReportStatus.pending.displayName, 'Menunggu peninjauan');
    });

    test('approved → "Tidak melanggar"', () {
      expect(ReportStatus.approved.displayName, 'Tidak melanggar');
    });

    test('rejected → "Laporan ditutup"', () {
      expect(ReportStatus.rejected.displayName, 'Laporan ditutup');
    });

    test('enforced → "Tindakan telah diambil"', () {
      expect(ReportStatus.enforced.displayName, 'Tindakan telah diambil');
    });
  });

  // ===========================================================================
  // toCreateRequestDto — reason formatting
  // ===========================================================================
  group('ReportMapper.toCreateRequestDto', () {
    test('formats reason with description', () {
      final request = CreateReportRequest(
        targetId: 'target-1',
        targetType: ReportTargetType.content,
        reason: ReportReasonType.spam,
        description: 'detailed spam description',
      );

      final dto = ReportMapper.toCreateRequestDto(request);

      expect(dto.entityType, 'content');
      expect(dto.entityId, 'target-1');
      expect(dto.reason, 'spam: detailed spam description');
    });

    test('formats reason without description', () {
      final request = CreateReportRequest(
        targetId: 'target-1',
        targetType: ReportTargetType.user,
        reason: ReportReasonType.harassment,
      );

      final dto = ReportMapper.toCreateRequestDto(request);

      expect(dto.entityType, 'user');
      expect(dto.reason, 'harassment');
    });

    test('truncates reason at 500 characters', () {
      final longDesc = 'x' * 600;
      final request = CreateReportRequest(
        targetId: 'target-1',
        targetType: ReportTargetType.content,
        reason: ReportReasonType.spam,
        description: longDesc,
      );

      final dto = ReportMapper.toCreateRequestDto(request);

      expect(dto.reason.length, 500);
      expect(dto.reason, startsWith('spam: '));
    });
  });

  // ===========================================================================
  // Report entity computed properties
  // ===========================================================================
  group('Report entity', () {
    test('isResolved includes enforced, approved, rejected', () {
      final pending = _buildReport(status: ReportStatus.pending);
      final approved = _buildReport(status: ReportStatus.approved);
      final rejected = _buildReport(status: ReportStatus.rejected);
      final enforced = _buildReport(status: ReportStatus.enforced);

      expect(pending.isResolved, isFalse);
      expect(approved.isResolved, isTrue);
      expect(rejected.isResolved, isTrue);
      expect(enforced.isResolved, isTrue);
    });

    test('canBeReviewed only for pending', () {
      final pending = _buildReport(status: ReportStatus.pending);
      final enforced = _buildReport(status: ReportStatus.enforced);
      final approved = _buildReport(status: ReportStatus.approved);

      expect(pending.canBeReviewed, isTrue);
      expect(enforced.canBeReviewed, isFalse);
      expect(approved.canBeReviewed, isFalse);
    });
  });
}

// =============================================================================
// Helpers
// =============================================================================

ModerationCaseDto _buildDto({
  String resourceType = 'content',
  String status = 'pending',
}) {
  return ModerationCaseDto(
    id: '00000000-0000-0000-0000-000000000001',
    resourceType: resourceType,
    resourceId: '00000000-0000-0000-0000-000000000002',
    status: status,
    reportedBy: '00000000-0000-0000-0000-000000000003',
    reason: 'test reason',
    createdAt: DateTime(2026, 7, 31),
  );
}

Report _buildReport({required ReportStatus status}) {
  return Report(
    id: '00000000-0000-0000-0000-000000000001',
    reporterId: 'r1',
    targetId: 't1',
    targetType: ReportTargetType.content,
    reason: ReportReasonType.spam,
    status: status,
    createdAt: DateTime(2026, 7, 31),
  );
}
