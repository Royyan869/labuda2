import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/src/router/modules/profile_module.dart';
import 'package:labuda/domains/system/notification/presentation/screens/notification_settings_screen.dart';

void main() {
  testWidgets('notification settings screen is read-only', (tester) async {
    await tester.pumpWidget(
      const MaterialApp(home: NotificationSettingsScreen()),
    );

    expect(find.text('Notification Settings'), findsOneWidget);
    expect(
      find.text('Notification settings are under development'),
      findsOneWidget,
    );
    expect(find.text('Save Changes'), findsNothing);
  });

  testWidgets('profile module registers /settings/notifications', (
    tester,
  ) async {
    final router = GoRouter(
      routes: ProfileModule().routes,
      initialLocation: '/settings/notifications',
    );

    await tester.pumpWidget(MaterialApp.router(routerConfig: router));
    await tester.pumpAndSettle();

    expect(find.text('Notification Settings'), findsOneWidget);
    expect(find.text('Save Changes'), findsNothing);
  });
}
