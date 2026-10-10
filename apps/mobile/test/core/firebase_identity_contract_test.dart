// H.4.6-E1 Firebase identity replacement contract.
//
// Verifies that the mobile app's native configuration is fully aligned to
// Firebase project "hishumi" (number 1079914053843) and application ID
// com.hishumi.app — without asserting any secret values.

import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  group('H.4.6-E1 Firebase identity contract', () {
    late String gradle;
    late String manifest;
    late String firebaseOptions;
    late String googleServices;
    late String plist;

    setUpAll(() {
      final root = Directory.current.path;
      gradle = File(
        '$root/android/app/build.gradle.kts',
      ).readAsStringSync();
      manifest = File(
        '$root/android/app/src/main/AndroidManifest.xml',
      ).readAsStringSync();
      firebaseOptions = File(
        '$root/lib/firebase_options.dart',
      ).readAsStringSync();
      googleServices = File(
        '$root/android/app/google-services.json',
      ).readAsStringSync();
      plist = File(
        '$root/ios/Runner/GoogleService-Info.plist',
      ).readAsStringSync();
    });

    test('gradle applicationId and namespace are com.hishumi.app', () {
      expect(gradle, contains('applicationId = "com.hishumi.app"'));
      expect(gradle, contains('namespace = "com.hishumi.app"'));
      expect(gradle, isNot(contains('com.labuda.app.labuda')));
    });

    test('firebase_options points to project hishumi', () {
      expect(firebaseOptions, contains("projectId: 'hishumi'"));
      expect(firebaseOptions, contains("messagingSenderId: '1079914053843'"));
      expect(firebaseOptions, contains("iosBundleId: 'com.hishumi.app'"));
      expect(firebaseOptions, isNot(contains('labuda-79de2')));
      expect(firebaseOptions, isNot(contains('com.labuda.app.labuda')));
    });

    test('google-services.json is for project hishumi', () {
      expect(googleServices, contains('"project_id": "hishumi"'));
      expect(googleServices, contains('"project_number": "1079914053843"'));
      expect(googleServices, contains('"package_name": "com.hishumi.app"'));
      expect(googleServices, isNot(contains('labuda-79de2')));
      expect(googleServices, isNot(contains('com.labuda.app.labuda')));
    });

    test('GoogleService-Info.plist is for project hishumi', () {
      expect(plist, contains('<string>hishumi</string>'));
      expect(plist, contains('<string>1079914053843</string>'));
      expect(plist, contains('<string>com.hishumi.app</string>'));
      expect(plist, isNot(contains('labuda-79de2')));
      expect(plist, isNot(contains('com.labuda.app.labuda')));
    });

    test('Android manifest has no old Firebase hostname', () {
      expect(manifest, isNot(contains('labuda-79de2.web.app')));
    });
  });
}
