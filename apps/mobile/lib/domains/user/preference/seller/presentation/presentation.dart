/// Presentation Layer Barrel File
///
/// Exports all presentation layer components (screens & widgets).
library;

// Providers
export 'providers/current_seller_provider.dart';

// Screens (to be migrated from old module)
// export 'screens/seller_dashboard_screen.dart';
// export 'screens/seller_analytics_screen.dart';
// export 'screens/seller_earnings_screen.dart';
// export 'screens/seller_subscription_screen.dart';

// Widgets (to be migrated from old module)
// export 'widgets/seller_stats_card.dart';
// export 'widgets/seller_activity_item.dart';
// export 'widgets/seller_revenue_card.dart';

// Stub widgets for profile module compatibility
export 'widgets/profile_store_tab.dart';
export 'widgets/settings_upgrade_card.dart' show SettingsUpgradeCard;
export 'screens/seller_upgrade_wizard_screen.dart'
    show SellerUpgradeWizardScreen;
export 'screens/seller_dashboard_screen.dart' show SellerDashboardScreen;

// Verification
export 'screens/seller_verification_screen.dart' show SellerVerificationScreen;

// For now, existing screens remain in the old seller module
// and can be imported directly from there when needed
