import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/domains/commerce/negotiation/negotiation/data/dto/negotiation_dto.dart';
import 'package:hishumi/domains/commerce/negotiation/negotiation/data/mappers/negotiation_mapper.dart';
import 'package:hishumi/domains/commerce/negotiation/negotiation/domain/entities/negotiation.dart';

/// NEGOTIATION STATUS MAPPING CONTRACT (T3 status mapping purge)
///
/// The backend `negotiation_sessions.status` authority recognizes exactly four
/// states: active, accepted, cancelled, expired.
///
/// Legacy mobile aliases (`pending`, `countered`, `rejected`, `completed`)
/// have been purged. These tests are the negative contract: they fail if the
/// legacy alias mapping is ever reintroduced, and lock the fail-fast behavior
/// for unknown wire values (a silent `active` fallback would let a buyer
/// attempt actions on a state the backend never authorized).
void main() {
  group('NegotiationStatusExtension.fromString — positive contract', () {
    test('parses all four canonical backend states', () {
      expect(
        NegotiationStatusExtension.fromString('active'),
        NegotiationStatus.active,
      );
      expect(
        NegotiationStatusExtension.fromString('accepted'),
        NegotiationStatus.accepted,
      );
      expect(
        NegotiationStatusExtension.fromString('cancelled'),
        NegotiationStatus.cancelled,
      );
      expect(
        NegotiationStatusExtension.fromString('expired'),
        NegotiationStatus.expired,
      );
    });
  });

  group('NegotiationStatusExtension.fromString — negative contract', () {
    test('rejects forbidden legacy status aliases', () {
      // Forbidden legacy states — must never map back onto a canonical enum.
      for (final legacy in <String>['pending', 'countered', 'rejected']) {
        expect(
          () => NegotiationStatusExtension.fromString(legacy),
          throwsArgumentError,
          reason: 'legacy status "$legacy" must not parse',
        );
      }
    });

    test('rejects legacy completed alias for accepted', () {
      expect(
        () => NegotiationStatusExtension.fromString('completed'),
        throwsArgumentError,
      );
    });

    test('rejects unknown wire values (no silent active fallback)', () {
      for (final unknown in <String>['', 'ACTIVE', 'in_progress', 'closed']) {
        expect(
          () => NegotiationStatusExtension.fromString(unknown),
          throwsArgumentError,
          reason: 'unknown status "$unknown" must not collapse into active',
        );
      }
    });
  });

  group('NegotiationMapper.toEntity — wire contract', () {
    NegotiationResponseDto dtoWithStatus(String status) {
      return NegotiationResponseDto(
        id: 'session-1',
        resourceType: 'for_sale',
        resourceId: 'fps-1',
        buyerId: 'buyer-1',
        sellerId: 'seller-1',
        status: status,
        proposalSequence: 1,
        createdAt: DateTime.utc(2026, 1, 1),
        updatedAt: DateTime.utc(2026, 1, 1),
        viewerCanAct: false,
        forSaleId: 'fps-1',
      );
    }

    test('maps canonical accepted status end-to-end', () {
      final negotiation = NegotiationMapper.toEntity(
        dtoWithStatus('accepted'),
      );
      expect(negotiation.status, NegotiationStatus.accepted);
      expect(negotiation.agreedPrice, isNull);
    });

    test('rejects legacy wire status through the mapper', () {
      expect(
        () => NegotiationMapper.toEntity(dtoWithStatus('pending')),
        throwsArgumentError,
      );
      expect(
        () => NegotiationMapper.toEntity(dtoWithStatus('countered')),
        throwsArgumentError,
      );
      expect(
        () => NegotiationMapper.toEntity(dtoWithStatus('rejected')),
        throwsArgumentError,
      );
    });
  });
}
