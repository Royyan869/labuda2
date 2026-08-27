// Shipping Feature Module
// Clean Architecture implementation

// Domain Layer
export 'domain/domain.dart';

// Data Layer - NOT EXPORTED (Implementation Detail)
// ❌ NOT exported: data/data.dart, data/dto/, data/mappers/, data/remote/
// These are internal to the module and accessed via repository interface only.

// Presentation Layer - Providers
export 'presentation/providers/providers.dart';
