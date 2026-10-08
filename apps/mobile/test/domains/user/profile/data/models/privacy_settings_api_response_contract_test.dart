// Canonical edit-profile API contract: social media has NO visibility toggle.
//
// Presence of a handle is the visibility authority. UpdateProfileApiRequest
// serializes only the canonical field set the backend persists; the obsolete
// privacy/website/social-toggle fields no longer exist.
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/user/profile/data/models/api/user_api_models.dart';

void main() {
  group('SocialMediaApiResponse', () {
    test('parses the four canonical handles', () {
      final response = SocialMediaApiResponse.fromJson(
        const <String, dynamic>{
          'instagram_handle': 'ig',
          'facebook_handle': 'fb',
          'twitter_handle': 'tw',
          'tiktok_handle': 'tt',
        },
      );

      expect(response.instagramHandle, 'ig');
      expect(response.facebookHandle, 'fb');
      expect(response.twitterHandle, 'tw');
      expect(response.tiktokHandle, 'tt');
    });

    test('absent handles parse as null (not displayed)', () {
      final response = SocialMediaApiResponse.fromJson(
        const <String, dynamic>{},
      );

      expect(response.instagramHandle, isNull);
      expect(response.facebookHandle, isNull);
      expect(response.twitterHandle, isNull);
      expect(response.tiktokHandle, isNull);
    });
  });

  group('UpdateProfileApiRequest social serialization', () {
    test('omitted social handle is not serialized', () {
      final request = const UpdateProfileApiRequest(bio: 'bio');

      final json = request.toJson();

      expect(json.containsKey('instagram_handle'), isFalse);
      expect(json['bio'], 'bio');
    });

    test('explicit handles serialize canonically', () {
      final request = const UpdateProfileApiRequest(
        instagramHandle: 'ig',
        facebookHandle: 'fb',
        twitterHandle: 'tw',
        tiktokHandle: 'tt',
      );

      expect(request.toJson(), {
        'instagram_handle': 'ig',
        'facebook_handle': 'fb',
        'twitter_handle': 'tw',
        'tiktok_handle': 'tt',
      });
    });

    test('empty string is the canonical clear signal', () {
      final request = const UpdateProfileApiRequest(instagramHandle: '');
      expect(request.toJson()['instagram_handle'], '');
    });

    test('obsolete website/privacy fields never serialize', () {
      final request = const UpdateProfileApiRequest(bio: 'bio');
      final json = request.toJson();

      for (final obsolete in [
        'website_url',
        'youtube_handle',
        'visibility',
        'show_phone_number',
        'show_email',
        'show_location',
        'allow_messages_from',
        'allow_tagging',
        'show_activity_status',
        'show_transaction_count',
      ]) {
        expect(json.containsKey(obsolete), isFalse, reason: obsolete);
      }
    });
  });
}
