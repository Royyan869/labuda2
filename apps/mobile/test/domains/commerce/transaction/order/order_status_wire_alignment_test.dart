// Order Status Wire Alignment Test
//
// Verifies that the mobile OrderStatus parser correctly handles ALL
// backend wire values, including:
// - "pending_payment" (backend canonical for StatusPending)
// - "cancelled_timeout" (backend StatusCancelledTimeout)
// - All legacy aliases still parse correctly
//
// Backend source of truth: backend/internal/commerce/order/entity/order_status.go

import 'package:flutter_test/flutter_test.dart';

import 'package:labuda/domains/commerce/transaction/order/domain/entities/order_status.dart';
import 'package:labuda/domains/commerce/transaction/order/data/mappers/order_mapper.dart';

void main() {
  // =========================================================================
  // OrderStatusExtension.parse — canonical wire values
  // =========================================================================
  group('OrderStatusExtension.parse — backend canonical wire values', () {
    test('"pending_payment" parses to OrderStatus.pending', () {
      expect(
        OrderStatusExtension.parse('pending_payment'),
        equals(OrderStatus.pending),
      );
    });

    test('"pending" parses to OrderStatus.pending', () {
      expect(
        OrderStatusExtension.parse('pending'),
        equals(OrderStatus.pending),
      );
    });

    test('"paid" parses to OrderStatus.paid', () {
      expect(OrderStatusExtension.parse('paid'), equals(OrderStatus.paid));
    });

    test('"shipped" parses to OrderStatus.shipped', () {
      expect(
        OrderStatusExtension.parse('shipped'),
        equals(OrderStatus.shipped),
      );
    });

    test('"delivered" parses to OrderStatus.delivered', () {
      expect(
        OrderStatusExtension.parse('delivered'),
        equals(OrderStatus.delivered),
      );
    });

    test('"completed" parses to OrderStatus.completed', () {
      expect(
        OrderStatusExtension.parse('completed'),
        equals(OrderStatus.completed),
      );
    });

    test('"cancelled" parses to OrderStatus.cancelled', () {
      expect(
        OrderStatusExtension.parse('cancelled'),
        equals(OrderStatus.cancelled),
      );
    });

    test('"cancelled_timeout" parses to OrderStatus.cancelledTimeout', () {
      expect(
        OrderStatusExtension.parse('cancelled_timeout'),
        equals(OrderStatus.cancelledTimeout),
      );
    });

    test('"refunded" parses to OrderStatus.refunded', () {
      expect(
        OrderStatusExtension.parse('refunded'),
        equals(OrderStatus.refunded),
      );
    });

    test('"dispute_open" parses to OrderStatus.disputeOpen', () {
      expect(
        OrderStatusExtension.parse('dispute_open'),
        equals(OrderStatus.disputeOpen),
      );
    });

    test('"partially_refunded" parses to OrderStatus.partiallyRefunded', () {
      expect(
        OrderStatusExtension.parse('partially_refunded'),
        equals(OrderStatus.partiallyRefunded),
      );
    });

    test('"expired" parses to OrderStatus.expired', () {
      expect(
        OrderStatusExtension.parse('expired'),
        equals(OrderStatus.expired),
      );
    });
  });

  // =========================================================================
  // OrderStatusExtension.parse — legacy aliases (backward compat)
  // =========================================================================
  group('OrderStatusExtension.parse — legacy aliases', () {
    test('"waiting_payment" → OrderStatus.pending', () {
      expect(
        OrderStatusExtension.parse('waiting_payment'),
        equals(OrderStatus.pending),
      );
    });

    test('"waitingpayment" → OrderStatus.pending', () {
      expect(
        OrderStatusExtension.parse('waitingpayment'),
        equals(OrderStatus.pending),
      );
    });

    test('"confirmed" → OrderStatus.paid', () {
      expect(OrderStatusExtension.parse('confirmed'), equals(OrderStatus.paid));
    });

    test('"processing" → OrderStatus.paid', () {
      expect(
        OrderStatusExtension.parse('processing'),
        equals(OrderStatus.paid),
      );
    });

    test('"disputeopen" (no underscore) → OrderStatus.disputeOpen', () {
      expect(
        OrderStatusExtension.parse('disputeopen'),
        equals(OrderStatus.disputeOpen),
      );
    });

    test(
      '"partiallyrefunded" (no underscore) → OrderStatus.partiallyRefunded',
      () {
        expect(
          OrderStatusExtension.parse('partiallyrefunded'),
          equals(OrderStatus.partiallyRefunded),
        );
      },
    );

    test(
      '"cancelledtimeout" (no underscore) → OrderStatus.cancelledTimeout',
      () {
        expect(
          OrderStatusExtension.parse('cancelledtimeout'),
          equals(OrderStatus.cancelledTimeout),
        );
      },
    );
  });

  // =========================================================================
  // OrderStatusExtension.parse — case insensitivity
  // =========================================================================
  group('OrderStatusExtension.parse — case insensitivity', () {
    test('"PENDING_PAYMENT" uppercase parses correctly', () {
      expect(
        OrderStatusExtension.parse('PENDING_PAYMENT'),
        equals(OrderStatus.pending),
      );
    });

    test('"Cancelled_Timeout" mixed case parses correctly', () {
      expect(
        OrderStatusExtension.parse('Cancelled_Timeout'),
        equals(OrderStatus.cancelledTimeout),
      );
    });
  });

  // =========================================================================
  // OrderStatusExtension.parse — null and unknown
  // =========================================================================
  group('OrderStatusExtension.parse — null and unknown', () {
    test('null returns null', () {
      expect(OrderStatusExtension.parse(null), isNull);
    });

    test('unknown string returns null', () {
      expect(OrderStatusExtension.parse('nonexistent_status'), isNull);
    });
  });

  // =========================================================================
  // OrderStatusExtension.value — round-trip
  // =========================================================================
  group('OrderStatusExtension.value — round-trip', () {
    test('cancelledTimeout serializes to "cancelled_timeout"', () {
      expect(OrderStatus.cancelledTimeout.value, equals('cancelled_timeout'));
    });

    test('pending serializes to "pending"', () {
      expect(OrderStatus.pending.value, equals('pending'));
    });

    test('all statuses round-trip through parse(value)', () {
      for (final status in OrderStatus.values) {
        final serialized = status.value;
        final parsed = OrderStatusExtension.parse(serialized);
        expect(
          parsed,
          equals(status),
          reason: 'Round-trip failed for $status (serialized: "$serialized")',
        );
      }
    });
  });

  // =========================================================================
  // OrderMapper._mapOrderStatus — fail-loud behavior
  // =========================================================================
  group('OrderMapper mapper — wire alignment', () {
    test('"pending_payment" maps via mapper toString', () {
      // mapOrderStatusToString uses the enum's canonical value
      expect(
        OrderMapper.mapOrderStatusToString(OrderStatus.pending),
        equals('pending'),
      );
    });

    test('"cancelled_timeout" maps via mapper toString', () {
      expect(
        OrderMapper.mapOrderStatusToString(OrderStatus.cancelledTimeout),
        equals('cancelled_timeout'),
      );
    });
  });

  // =========================================================================
  // Backend completeness — every backend status has a mobile mapping
  // =========================================================================
  group('Backend completeness — no unmapped wire values', () {
    // These are ALL the wire values from backend order_status.go
    final backendWireValues = [
      'pending_payment',
      'paid',
      'shipped',
      'delivered',
      'completed',
      'cancelled',
      'cancelled_timeout',
      'refunded',
      'partially_refunded',
      'dispute_open',
      'expired',
    ];

    for (final wire in backendWireValues) {
      test('backend wire value "$wire" parses to a non-null OrderStatus', () {
        final parsed = OrderStatusExtension.parse(wire);
        expect(
          parsed,
          isNotNull,
          reason:
              'Backend wire value "$wire" has no mobile mapping — this will '
              'cause silent data corruption (fallback to pending) or crash '
              '(FormatException in mapper)',
        );
      });
    }
  });

  // =========================================================================
  // Enum count — guard against silent additions
  // =========================================================================
  group('Enum count guard', () {
    test('OrderStatus has exactly 11 values (matching backend)', () {
      // Backend has 11 statuses. If this fails, a new status was added
      // to the enum without updating this test or the backend alignment.
      expect(OrderStatus.values.length, equals(11));
    });
  });
}
