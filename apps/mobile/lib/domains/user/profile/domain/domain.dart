// Profile Domain Layer
// SOURCE OF TRUTH for Profile module
//
// This layer contains:
// - Entities: Core business objects
// - Repository Interfaces: Contracts for data access
// - Value Objects: Immutable value types (when needed)
//
// 🚫 RULES:
// - NO Firebase imports
// - NO Riverpod imports
// - NO Flutter imports
// - Pure Dart + domain logic only

// ========================================
// ENTITIES
// ========================================
export 'entities/profile_entity.dart';
export 'entities/address_entity.dart';
export 'entities/bank_account_entity.dart';

// ========================================
// REPOSITORY INTERFACES
// ========================================
export 'repositories/i_profile_repository.dart';
export 'repositories/i_address_repository.dart';

// ========================================
// VALUE OBJECTS
// ========================================
// (to be added if needed)
