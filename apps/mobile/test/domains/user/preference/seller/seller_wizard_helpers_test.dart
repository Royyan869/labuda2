import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/user/preference/seller/presentation/widgets/wizard/seller_wizard_helpers.dart';

void main() {
  group('SellerWizardHelpers', () {
    test('account step requires a structured sender address', () {
      // D2 HARD GATE: emailVerified is no longer a client-side wizard gate —
      // every authenticated user is already verified before the exchange.
      // OWNER DECISION: seller has no bio — bio belongs to the user profile
      // only, so the wizard gate takes no bio parameter.
      expect(
        SellerWizardHelpers.isAccountStepValid(
          username: 'seller01',
          phoneNumber: '+628123456789',
          senderAddress: 'Jl. Test No. 1, Dago, Bandung, Jawa Barat 40135',
        ),
        isTrue,
      );

      expect(
        SellerWizardHelpers.isAccountStepValid(
          username: 'seller01',
          phoneNumber: '+628123456789',
          senderAddress: '',
        ),
        isFalse,
      );
    });
  });
}
