import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

/// REGRESSION GUARD — canonical store photo upload authority.
///
/// The wizard's store photo upload must use the Labuda user ID from the
/// canonical auth state, never the Firebase UID. Backend fixed-key validation
/// (`images/stores/{user_id}.jpg`) checks ownership against the JWT user ID;
/// a Firebase UID is always rejected with INVALID_STORAGE_KEY
/// ("storage_key must match images/avatars/{user_id}.jpg ...").
///
/// Contract-tested at source level following the repo's
/// source-contract convention (see e.g.
/// test/domains/user/profile/edit_profile_canonical_identity_contract_test.dart).
String _readSource(String relativePath) =>
    File(relativePath).readAsStringSync();

void main() {
  group('Seller wizard store photo upload authority', () {
    final source = _readSource(
      'lib/domains/user/preference/seller/presentation/screens/seller_upgrade_wizard_screen.dart',
    );

    test('wizard must NOT read FirebaseAuth directly', () {
      expect(source, isNot(contains('FirebaseAuth.instance')));
      expect(
        source,
        isNot(contains("import 'package:firebase_auth/firebase_auth.dart';")),
      );
    });

    test('wizard must derive the upload owner from canonical auth state', () {
      expect(
        source.contains('_currentAuthenticatedUserId()'),
        isTrue,
        reason: 'upload must use the Labuda user ID from auth state',
      );
      expect(
        source.contains('uploadStorePhoto(userId: userId'),
        isTrue,
        reason: 'upload must pass the Labuda user ID, not firebaseUser.uid',
      );
    });

    test('wizard persists the storage key, not a read URL, to onboarding', () {
      expect(
        source.contains('storeImageUrl: _farmPhotoStorageKey'),
        isTrue,
        reason:
            'POST /seller/onboarding store_image_url must be the canonical '
            'images/stores/{user_id}.jpg key, never a resolved read URL',
      );
    });

    test('wizard separates storage key from display URL', () {
      expect(source.contains('_farmPhotoStorageKey'), isTrue);
      expect(source.contains('_farmPhotoDisplayUrl'), isTrue);
      expect(
        source.contains('_farmPhotoUrl'),
        isFalse,
        reason:
            'legacy conflated field must stay removed so key/url cannot be '
            'mixed up again',
      );
    });

    test(
      'store photo upload service returns key + display url, never a bare string',
      () {
        final serviceSource = _readSource(
          'lib/domains/user/preference/seller/data/services/store_photo_upload_service.dart',
        );
        expect(serviceSource.contains('StorePhotoUploadOutcome'), isTrue);
        expect(
          serviceSource.contains('Result.success(storageKey)'),
          isFalse,
          reason:
              'a bare-string success conflated storage key and display URL; '
              'the outcome object keeps them separate',
        );
      },
    );

    test(
      'edit profile save handler persists the storage key for PATCH /seller/profile',
      () {
        final saveSource = _readSource(
          'lib/domains/user/profile/presentation/screens/edit_profile/edit_profile_save_handler.dart',
        );
        expect(
          saveSource.contains('result.data!.storageKey'),
          isTrue,
          reason:
              'PATCH /seller/profile validates the canonical key form '
              '(images/stores/{user_id}.jpg); a read URL would be rejected',
        );
      },
    );
  });

  group('Store name single authority', () {
    test(
      'wizard step2 and edit profile both render the shared StoreNameFormField',
      () {
        for (final path in <String>[
          'lib/domains/user/preference/seller/presentation/widgets/wizard/seller_wizard_step2_widget.dart',
          'lib/domains/user/profile/presentation/screens/edit_profile/edit_profile_farm_section.dart',
          'lib/domains/user/profile/presentation/screens/edit_profile/edit_profile_store_section.dart',
        ]) {
          final source = _readSource(path);
          expect(
            source.contains('StoreNameFormField('),
            isTrue,
            reason:
                '$path must use the shared store name field instead of its '
                'own inline label/validator',
          );
        }
      },
    );

    test('no surface keeps its own diverging store-name copy', () {
      final step2 = _readSource(
        'lib/domains/user/preference/seller/presentation/widgets/wizard/seller_wizard_step2_widget.dart',
      );
      final farmSection = _readSource(
        'lib/domains/user/profile/presentation/screens/edit_profile/edit_profile_farm_section.dart',
      );
      final storeSection = _readSource(
        'lib/domains/user/profile/presentation/screens/edit_profile/edit_profile_store_section.dart',
      );

      // The canonical label lives ONLY in StoreNameFormField.
      const canonicalLabel = 'Nama Toko/Farm *';
      for (final (name, source) in [
        ('step2', step2),
        ('farm_section', farmSection),
        ('store_section', storeSection),
      ]) {
        expect(
          source.contains(canonicalLabel),
          isFalse,
          reason:
              '$name must not carry its own label copy; single authority is '
              'StoreNameFormField.canonicalLabel',
        );
      }

      expect(
        farmSection.contains("'Farm Name'"),
        isFalse,
        reason: 'farm section legacy label must be gone',
      );
      expect(
        storeSection.contains("'Store Name'"),
        isFalse,
        reason: 'store section legacy label must be gone',
      );
    });

    test(
      'edit profile validators delegate farm-name validation to the canonical one',
      () {
        final validators = _readSource(
          'lib/domains/user/profile/presentation/screens/edit_profile/edit_profile_validators.dart',
        );
        // Single authority: the canonical validator lives ONLY in the seller
        // domain widget; edit profile must not redefine it.
        expect(
          validators.contains(
            "import 'package:labuda/domains/user/preference/seller/presentation/widgets/wizard/store_name_form_field.dart';",
          ),
          isTrue,
        );
        expect(
          validators.contains('validateFarmName'),
          isFalse,
          reason:
              'dead validator removed — StoreNameFormField.validate is the '
              'single store-name validation authority',
        );
        expect(
          validators.contains("'Farm name is required'"),
          isFalse,
          reason: 'diverging message removed in favor of the canonical one',
        );
      },
    );

    test('wizard helpers carry no dead store-change detection', () {
      final helpers = _readSource(
        'lib/domains/user/preference/seller/presentation/widgets/wizard/seller_wizard_helpers.dart',
      );
      expect(
        helpers.contains('static bool hasAnyChanges('),
        isFalse,
        reason:
            'SellerWizardHelpers.hasAnyChanges was an uncalled duplicate of '
            'the screen-level _hasAnyChanges getter',
      );
    });

    test('store photo service exposes no client-side delete authority', () {
      final service = _readSource(
        'lib/domains/user/preference/seller/data/services/store_photo_upload_service.dart',
      );
      expect(
        service.contains('deleteStorePhoto('),
        isFalse,
        reason:
            'canonical removal is PATCH /seller/profile with an empty '
            'store_image_url (backend write) — a client delete stub would be '
            'a parallel removal authority',
      );
    });
  });
}
