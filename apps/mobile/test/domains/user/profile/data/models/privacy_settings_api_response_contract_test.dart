import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/user/profile/data/models/api/user_api_models.dart';

void main() {
  group('PrivacySettingsApiResponse', () {
    test('defaults show_activity_status to true when omitted', () {
      final response = PrivacySettingsApiResponse.fromJson(
        const <String, dynamic>{},
      );

      expect(response.showActivityStatus, isTrue);
    });

    test('parses explicit true and false values', () {
      final enabled = PrivacySettingsApiResponse.fromJson(
        const <String, dynamic>{'show_activity_status': true},
      );
      final disabled = PrivacySettingsApiResponse.fromJson(
        const <String, dynamic>{'show_activity_status': false},
      );

      expect(enabled.showActivityStatus, isTrue);
      expect(disabled.showActivityStatus, isFalse);
    });

    test('non-boolean payload fails closed', () {
      expect(
        () => PrivacySettingsApiResponse.fromJson(const <String, dynamic>{
          'show_activity_status': 'yes',
        }),
        throwsA(isA<TypeError>()),
      );
    });
  });

  group('UpdateProfileApiRequest privacy serialization', () {
    test('omitted show_activity_status is not serialized', () {
      final request = UpdateProfileApiRequest(bio: 'bio', showEmail: true);

      final json = request.toJson();

      expect(json.containsKey('show_activity_status'), isFalse);
      expect(json['bio'], 'bio');
      expect(json['show_email'], isTrue);
    });

    test('explicit true and false serialize canonically', () {
      final enabled = UpdateProfileApiRequest(showActivityStatus: true);
      final disabled = UpdateProfileApiRequest(showActivityStatus: false);

      expect(enabled.toJson()['show_activity_status'], isTrue);
      expect(disabled.toJson()['show_activity_status'], isFalse);
    });
  });
}
