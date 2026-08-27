import 'package:labuda/core/src/navigation/i_navigation_registry.dart';

/// Implementation of INavigationRegistry
///
/// Menyediakan registry untuk tabs tanpa coupling antar features
class NavigationRegistryImpl implements INavigationRegistry {
  final List<NavigationTab> _tabs = [];

  @override
  void registerTab(NavigationTab tab) {
    // Remove existing tab with same id
    _tabs.removeWhere((t) => t.id == tab.id);

    // Add new tab
    _tabs.add(tab);

    // Sort by order
    _tabs.sort((a, b) => a.order.compareTo(b.order));
  }

  @override
  List<NavigationTab> getRegisteredTabs() {
    return List.unmodifiable(_tabs);
  }

  @override
  void clearTabs() {
    _tabs.clear();
  }
}
