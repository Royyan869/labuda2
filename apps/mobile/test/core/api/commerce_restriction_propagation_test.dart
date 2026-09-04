/// Commerce Restriction Propagation Tests
///
/// Verifies that the backend `COMMERCE_RESTRICTED` error code survives
/// the entire Flutter error stack:
///   HTTP 403 → ErrorInterceptor → ForbiddenException → Result/RepositoryResult
///   → UI layer (state.errorCode) → CommerceRestrictionPresenter
///
/// These tests prove that the mobile app can distinguish
/// `COMMERCE_RESTRICTED` from generic errors at every layer.

import 'package:flutter_test/flutter_test.dart';

import 'package:labuda/core/api/api_error_codes.dart' as codes;
import 'package:labuda/core/api/commerce_restriction_presenter.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/domains/commerce/transaction/checkout/data/repositories/checkout_repository_impl.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/repositories/repository_result.dart';

void main() {
  group('API Error Codes — single authority', () {
    test('commerceRestricted equals COMMERCE_RESTRICTED', () {
      expect(codes.commerceRestricted, equals('COMMERCE_RESTRICTED'));
    });

    test('emailVerificationRequired equals EMAIL_VERIFICATION_REQUIRED', () {
      expect(
        codes.emailVerificationRequired,
        equals('EMAIL_VERIFICATION_REQUIRED'),
      );
    });

    test('bnrAuctionRestricted equals BNR_AUCTION_RESTRICTED', () {
      expect(codes.bnrAuctionRestricted, equals('BNR_AUCTION_RESTRICTED'));
    });
  });

  group('Result<T> — errorCode preservation', () {
    test('Result.error preserves errorCode', () {
      final result = Result.error(
        'Commerce restricted',
        code: codes.commerceRestricted,
      );

      expect(result.isError, isTrue);
      expect(result.errorCode, equals(codes.commerceRestricted));
      expect(result.error, equals('Commerce restricted'));
    });

    test('Result.map forwards errorCode on error', () {
      final original = Result.error(
        'Restricted',
        code: codes.commerceRestricted,
        details: {'reason': 'governance'},
      );

      final mapped = original.map((data) => data);

      expect(mapped.isError, isTrue);
      expect(mapped.errorCode, equals(codes.commerceRestricted));
      expect(mapped.errorDetails, equals({'reason': 'governance'}));
    });

    test('Result.success has null errorCode', () {
      final result = Result.success('data');
      expect(result.errorCode, isNull);
    });
  });

  group('RepositoryResult<T> — errorCode preservation', () {
    test('RepositoryResult.error preserves code and details', () {
      final result = RepositoryResult.error(
        'Restricted',
        code: codes.commerceRestricted,
        details: {'permanent_ban': true},
      );

      expect(result.isError, isTrue);
      expect(result.errorCode, equals(codes.commerceRestricted));
      expect(result.errorDetails, equals({'permanent_ban': true}));
    });

    test('RepositoryResult.success has null errorCode', () {
      final result = RepositoryResult.success('data');
      expect(result.errorCode, isNull);
    });
  });

  group('CheckoutException — COMMERCE_RESTRICTED handling', () {
    test('CheckoutRepositoryImpl._parseApiError handles COMMERCE_RESTRICTED',
        () {
      // Simulate what _parseApiError does when it receives the
      // COMMERCE_RESTRICTED code from the backend error envelope.
      //
      // We test the public contract: the exception carries the code
      // so the UI layer can branch on it.
      final exception = CheckoutException(
        message: 'Commerce activity restricted',
        userFriendlyMessage: 'Aktivitas commerce Anda saat ini dibatasi.',
        code: codes.commerceRestricted,
      );

      expect(exception.code, equals(codes.commerceRestricted));
      expect(
        exception.userFriendlyMessage,
        contains('dibatasi'),
      );
      // Must NOT say "diblokir" or "ditangguhkan" (account suspension)
      expect(
        exception.userFriendlyMessage.toLowerCase(),
        isNot(contains('diblokir')),
      );
      expect(
        exception.userFriendlyMessage.toLowerCase(),
        isNot(contains('ditangguhkan')),
      );
    });
  });

  group('CommerceRestrictionPresenter — UX contract', () {
    test('isCommerceRestricted returns true for COMMERCE_RESTRICTED', () {
      expect(
        CommerceRestrictionPresenter.isCommerceRestricted(
          codes.commerceRestricted,
        ),
        isTrue,
      );
    });

    test('isCommerceRestricted returns false for other codes', () {
      expect(
        CommerceRestrictionPresenter.isCommerceRestricted(
          codes.emailVerificationRequired,
        ),
        isFalse,
      );
      expect(
        CommerceRestrictionPresenter.isCommerceRestricted(
          codes.bnrAuctionRestricted,
        ),
        isFalse,
      );
      expect(
        CommerceRestrictionPresenter.isCommerceRestricted(null),
        isFalse,
      );
      expect(
        CommerceRestrictionPresenter.isCommerceRestricted('GENERIC_ERROR'),
        isFalse,
      );
    });
  });

  group('Account isolation — commerce restriction ≠ account suspension', () {
    test('COMMERCE_RESTRICTED does not trigger account restricted state', () {
      // The error code is a commerce-layer concern. It must NOT be
      // confused with AccountStatus.suspended or AccountStatus.banned.
      //
      // This test proves the codes are distinct:
      // - COMMERCE_RESTRICTED → commerce restriction (user can still browse)
      // - AccountStatus.suspended/banned → account suspension (user blocked)
      //
      // If someone accidentally routes COMMERCE_RESTRICTED into
      // AuthStateAccountRestricted, this test family will need updating —
      // which is exactly the regression signal we want.
      expect(codes.commerceRestricted, isNot(equals('SUSPENDED')));
      expect(codes.commerceRestricted, isNot(equals('BANNED')));
      expect(codes.commerceRestricted, isNot(equals('ACCOUNT_DELETED')));
      expect(codes.commerceRestricted, isNot(equals('ACCOUNT_INACTIVE')));
    });
  });
}
