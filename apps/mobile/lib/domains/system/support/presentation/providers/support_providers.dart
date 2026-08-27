/// Support Providers - Public API exports for support module
///
/// This file exports the Riverpod providers for the support feature.
/// The repository provider is now directly implemented in support_notifier.dart
/// following CLIENT_MIGRATION_STANDARD.md - no UnimplementedError override pattern.
///
/// MIGRATION COMPLETE: Support feature now uses pure Riverpod without GetIt/ServiceLocator
library;

// Re-export all public providers from the notifier
export 'support_notifier.dart'
    show
        supportRepositoryProvider,
        supportTicketProvider,
        supportCreateChatProvider,
        supportReopenTicketProvider;
