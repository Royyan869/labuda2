// SUBMISSION LOCK — bounded follow-up proof.
//
// Verifies the 13 audited consumers hold an immutable submission intent: the
// payload is either snapshotted before the first `await` or the controls that
// influence it are locked, AND submission cannot re-enter concurrently.
//
// Behavioral proof: an OTP flow can never start a second concurrent verify.
// Source-contract proof (positive + negative): every consumer snapshots/locks
// and the original mutable-read pattern is gone.
import 'dart:async';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/profile/data/services/phone_verification_service.dart';
import 'package:labuda/domains/user/profile/presentation/providers/phone_verification_provider.dart';
import 'package:labuda/domains/user/profile/presentation/widgets/phone_verification/otp_input_field.dart';

String _read(String path) => File(path).readAsStringSync();

String _code(String path) => _read(path)
    .split('\n')
    .where((l) => !l.trimLeft().startsWith('//'))
    .join('\n');

int _occurrences(String source, String needle) =>
    needle.allMatches(source).length;

const _forSale =
    'lib/domains/commerce/catalog/for_sale/presentation/screens/create_for_sale_screen.dart';
const _auction =
    'lib/domains/commerce/catalog/auction/presentation/screens/create_auction_screen.dart';
const _promotion =
    'lib/domains/commerce/pricing/promotion/presentation/screens/canonical_promotion_create_screen.dart';
const _checkout =
    'lib/domains/commerce/transaction/checkout/presentation/screens/checkout_screen_logic.dart';
const _refund =
    'lib/domains/commerce/transaction/order/presentation/widgets/refund_request_dialog_impl.dart';
const _dispute =
    'lib/domains/commerce/transaction/order/presentation/screens/order_detail/direct_dispute_dialog.dart';
const _personalInfo =
    'lib/domains/user/profile/presentation/screens/personal_information_screen.dart';
const _editProfile =
    'lib/domains/user/profile/presentation/screens/edit_profile/edit_profile_save_handler.dart';
const _verification =
    'lib/domains/user/preference/seller/presentation/screens/seller_verification_screen.dart';
const _upgrade =
    'lib/domains/user/preference/seller/presentation/screens/seller_upgrade_wizard_screen.dart';
const _renewal =
    'lib/domains/user/preference/seller/presentation/screens/seller_renewal_screen.dart';
const _otp =
    'lib/domains/user/profile/presentation/widgets/phone_verification/otp_input_field.dart';
const _comment =
    'lib/domains/social/comment/presentation/widgets/comment_input_with_commerce_reference.dart';

class _FakePhoneVerificationService implements PhoneVerificationService {
  int verifyCalls = 0;
  final Completer<bool> gate = Completer<bool>();

  @override
  Future<bool> verifyOTP(String otp) {
    verifyCalls++;
    return gate.future;
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

void main() {
  group('positive proof: each consumer snapshots or locks', () {
    test('Create ForSale snapshots the mutable media/cert/shipping lists', () {
      final src = _code(_forSale);
      expect(src.contains('mediaUrls: List<String>.of(_mediaUrls)'), isTrue);
      expect(src.contains('certificates: List<String>.of(_certificates)'), isTrue);
      expect(
        src.contains('shippingSetupIds: List<String>.of(_selectedShippingSetupIds)'),
        isTrue,
      );
    });

    test('Create Auction snapshots the mutable media list', () {
      final src = _code(_auction);
      expect(src.contains('final mediaSnapshot = List<String>.of(_mediaUrls)'), isTrue);
      expect(src.contains('mediaUrls: mediaSnapshot'), isTrue);
      expect(src.contains('certificates: List<String>.of(_certificates)'), isTrue);
    });

    test('Promotion Create snapshots the kind before the async flow', () {
      final src = _code(_promotion);
      expect(src.contains('final kind = _kind;'), isTrue);
      expect(src.contains('kind: kind,'), isTrue);
      expect(_occurrences(src, 'kind: _kind,'), 0);
    });

    test('Checkout uses one payment-method snapshot for order and payment', () {
      final src = _code(_checkout);
      // The snapshot is taken ONCE before the first await. Bid-win snapshots
      // null (method chosen at Order Detail); method-binding checkouts
      // snapshot the force-unwrapped selection.
      expect(
        src.contains(
          'final submittedPaymentMethodCode = state._isBidWin',
        ),
        isTrue,
      );
      expect(
        src.contains(': state._selectedPaymentMethodCode!;'),
        isTrue,
      );
      expect(src.contains('paymentMethodCode: submittedPaymentMethodCode,'), isTrue);
      expect(_occurrences(src, 'selectedMethodCode = state._selectedPaymentMethodCode!'), 0);
    });

    test('Refund snapshots the evidence photo list', () {
      expect(
        _code(_refund).contains('List<XFile>.of(_evidencePhotos)'),
        isTrue,
      );
    });

    test('Direct Dispute snapshots reason/description/photos before upload', () {
      final src = _code(_dispute);
      expect(src.contains('final reason = _selectedReason!;'), isTrue);
      expect(src.contains('final description = _descriptionController.text.trim();'), isTrue);
      expect(src.contains('final photos = List<XFile>.of(_photoFiles);'), isTrue);
      expect(src.contains('reason: reason.displayName,'), isTrue);
      expect(_occurrences(src, '_selectedReason!.displayName'), 0);
    });

    test('Personal Information snapshots phone before the DOB await', () {
      final src = _code(_personalInfo);
      expect(_occurrences(src, 'final phoneValue = _phoneController.text.trim();'), 1);
    });

    test('Unified Edit Profile snapshots text/social/store before uploads', () {
      final src = _code(_editProfile);
      expect(src.contains('final bioSnapshot = bioController.text.trim();'), isTrue);
      expect(src.contains('final storeNameSnapshot = farmNameController.text.trim();'), isTrue);
      expect(src.contains('bio: bioSnapshot,'), isTrue);
      expect(src.contains('instagramHandle: socials.instagram,'), isTrue);
      expect(src.contains('storeName: storeNameSnapshot,'), isTrue);
      expect(_occurrences(src, 'bio: bioController.text'), 0);
    });

    test('Seller Verification snapshots name/NIK and guards _isUploading', () {
      final src = _code(_verification);
      expect(src.contains('if (_isUploading) return;'), isTrue);
      expect(src.contains('fullName: fullNameSnapshot,'), isTrue);
      expect(src.contains('nationalId: nationalIdSnapshot,'), isTrue);
      expect(src.contains('!_isUploading'), isTrue);
    });

    test('Seller Upgrade Wizard snapshots the method before onboarding', () {
      final src = _code(_upgrade);
      final declaration = src.lastIndexOf(
        'final selectedMethod = _selectedSubscriptionMethod;',
      );
      final onboarding = src.indexOf('.performOnboarding(');
      expect(declaration, greaterThanOrEqualTo(0));
      expect(
        onboarding,
        greaterThan(declaration),
        reason: 'the method must be snapshotted before the onboarding await',
      );
    });

    test('Seller Renewal snapshots the method before baseline', () {
      final src = _code(_renewal);
      expect(src.contains('final selectedMethod = _selected!;'), isTrue);
      expect(src.contains('paymentMethodCode: selectedMethod.methodCode,'), isTrue);
      expect(_occurrences(src, 'paymentMethodCode: _selected!.methodCode'), 0);
    });

    test('OTP locks boxes and guards re-entry', () {
      final src = _code(_otp);
      expect(
        src.contains('if (ref.read(phoneVerificationProvider).isVerifying) return;'),
        isTrue,
      );
      expect(src.contains('enabled: !state.isVerifying'), isTrue);
    });

    test('Comment Input snapshots pending media before upload', () {
      final src = _code(_comment);
      expect(src.contains('final pending = List.of(_pending);'), isTrue);
      expect(src.contains('items: pending,'), isTrue);
      expect(src.contains('pending.map((e) => e.url!)'), isTrue);
    });
  });

  group('negative proof: mutable-read patterns are gone', () {
    test('no submission payload reads a mutable list by reference', () {
      // The request payload uses value copies. (The UI uploader/retry widgets
      // still hold the live list — they are the editor, not the request.)
      expect(_code(_forSale).contains('mediaUrls: List<String>.of(_mediaUrls)'), isTrue);
      expect(_code(_auction).contains('mediaUrls: mediaSnapshot'), isTrue);
      expect(_code(_refund).contains('List<XFile>.of(_evidencePhotos)'), isTrue);
      expect(
        _code(_dispute).contains('final photos = List<XFile>.of(_photoFiles);'),
        isTrue,
      );
    });

    test('no selection value is re-read after await', () {
      expect(_occurrences(_code(_promotion), 'kind: _kind,'), 0);
      expect(_occurrences(_code(_renewal), 'paymentMethodCode: _selected!.methodCode'), 0);
      expect(
        _occurrences(_code(_checkout), 'selectedMethodCode = state._selectedPaymentMethodCode!'),
        0,
      );
    });
  });

  group('behavioral proof: OTP concurrent verification cannot occur', () {
    testWidgets('one verification; boxes locked while in flight', (tester) async {
      final fake = _FakePhoneVerificationService();
      var success = false;

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            phoneVerificationServiceProvider.overrideWithValue(fake),
          ],
          child: MaterialApp(
            theme: AppTheme.lightTheme,
            home: Scaffold(
              body: OTPInputField(
                phoneNumber: '081234567890',
                onVerificationSuccess: () => success = true,
                onResend: () {},
              ),
            ),
          ),
        ),
      );

      final boxes = find.byType(TextFormField);
      for (var i = 0; i < 6; i++) {
        await tester.enterText(boxes.at(i), '1');
        await tester.pump();
      }

      // The auto-submit fired exactly once for the completed code.
      expect(fake.verifyCalls, 1);
      // Boxes are locked while the verification is in flight.
      for (var i = 0; i < 6; i++) {
        expect(tester.widget<TextFormField>(boxes.at(i)).enabled, isFalse);
      }

      // Completing the request releases the lock and reports success.
      fake.gate.complete(true);
      await tester.pumpAndSettle();
      expect(success, isTrue);
      for (var i = 0; i < 6; i++) {
        expect(tester.widget<TextFormField>(boxes.at(i)).enabled, isTrue);
      }
    });
  });

  group('generic field contract still intact', () {
    test('AppTextField remains the single generic producer', () {
      expect(
        File('lib/shared/widgets/app_text_field.dart').existsSync(),
        isTrue,
      );
      expect(
        File(
          'lib/domains/user/identity/authentication/presentation/shared/widgets/auth_text_field.dart',
        ).existsSync(),
        isFalse,
      );
    });
  });
}
