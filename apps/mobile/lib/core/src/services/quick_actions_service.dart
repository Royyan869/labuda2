import 'package:quick_actions/quick_actions.dart';

/// Quick Actions Service
///
/// Manages app shortcuts that appear when user long-presses the app icon.
///
/// Platform support:
/// - Android: App shortcuts (long-press app icon)
/// - iOS: 3D Touch shortcuts (force press app icon)
class QuickActionsService {
  final QuickActions _quickActions = const QuickActions();

  QuickActionsService();

  /// Initialize quick actions
  ///
  /// This should be called once during app initialization.
  /// Sets up the shortcuts and listens for shortcut taps.
  Future<void> initialize() async {
    try {
      await _setupQuickActions();
      _setupQuickActionsHandler();
    } catch (e) {
      // Error handling without debug print
    }
  }

  /// Setup the quick actions shortcuts
  Future<void> _setupQuickActions() async {
    await _quickActions.setShortcutItems(<ShortcutItem>[
      // Quick create post
      const ShortcutItem(
        type: 'action_create_post',
        localizedTitle: 'Buat Postingan',
        icon: 'ic_create',
      ),
    ]);
  }

  /// Setup handler for when user taps a quick action
  void _setupQuickActionsHandler() {
    _quickActions.initialize((String shortcutType) {
      _handleQuickAction(shortcutType);
    });
  }

  /// Handle quick action tap
  void _handleQuickAction(String type) {
    switch (type) {
      case 'action_create_post':
        // TODO: Implement create post navigation
        break;

      default:
        // Unknown shortcut type - ignore
        break;
    }
  }

  /// Dispose resources
  void dispose() {
    // Quick actions don't need disposal
  }
}
