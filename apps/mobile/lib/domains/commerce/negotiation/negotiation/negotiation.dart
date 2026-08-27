/// Negotiation Module (Backend-First)
///
/// Backend-First Architecture - Domain Strong, Presentation Thin
/// - No Firebase in domain/presentation
/// - No get_it - Riverpod only
/// - No UseCase classes - logic in Notifiers
library;

// Domain - Entities
export 'domain/entities/negotiation.dart';

// Domain - Repositories
export 'domain/repositories/negotiation_repository.dart';

// Presentation - State & Notifiers
export 'presentation/providers/negotiation_state.dart';
export 'presentation/providers/negotiation_notifier.dart';

// Presentation - Providers
export 'presentation/providers/negotiation_providers.dart';

// DI Helper
export 'negotiation_di.dart';
