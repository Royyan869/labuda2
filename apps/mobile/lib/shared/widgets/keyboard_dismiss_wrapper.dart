import 'package:flutter/material.dart';

/// Global wrapper to dismiss keyboard when tapping outside input fields
///
/// Uses Flutter's built-in GestureDetector with proper focus handling.
/// Simple and follows Flutter best practices.
///
/// Features:
/// - Dismisses keyboard on tap outside input fields
/// - Allows TextField/TextFormField to work normally
/// - Non-intrusive and performant
///
/// Usage:
/// ```dart
/// MaterialApp(
///   builder: (context, child) {
///     return KeyboardDismissWrapper(child: child!);
///   },
/// )
/// ```
class KeyboardDismissWrapper extends StatelessWidget {
  final Widget child;

  const KeyboardDismissWrapper({super.key, required this.child});

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: () {
        // Flutter best practice: unfocus when tapping outside
        FocusManager.instance.primaryFocus?.unfocus();
      },
      // Translucent allows child widgets to handle their own gestures
      behavior: HitTestBehavior.translucent,
      child: child,
    );
  }
}
