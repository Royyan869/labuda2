// Profile Data Layer
// Phase 4: Firebase isolated via HTTP API
//
// This layer contains:
// - Datasources: HTTP API calls to Go backend
// - Repositories: Implementation of domain interfaces
// - Mappers: DTO → Entity conversion
// - Models: API request/response models
// - Services: Lightweight services for user lookup & avatar caching
//
// 🚫 RULES:
// - Firebase SDK only via HTTP API (no direct Firestore)
// - Implements domain repository interfaces
// - Returns domain entities (exposes DTOs internally only)

// ========================================
// DATASOURCES (HTTP API)
// ========================================
export 'datasources/user_api_datasource.dart';
export 'datasources/address_api_datasource.dart';

// ========================================
// REPOSITORIES (API-BASED)
// ========================================
export 'repositories/profile_repository_api.dart';
export 'repositories/address_repository_api.dart';

// Deprecated: Firestore-based repositories (Phase 4 cleanup)
// These will be deleted in Phase 7 (Cutover)
// export 'repositories/profile_repository_impl.dart'; // Firebase - DEPRECATED
// export 'repositories/address_repository_impl.dart'; // Firebase - DEPRECATED
// bank_account_repository_impl.dart is REST-based; wired via bank_account_provider.dart (not exported here)

// ========================================
// MAPPERS
// ========================================
export 'mappers/user_api_mapper.dart';
export 'mappers/address_api_mapper.dart';

// ========================================
// API MODELS
// ========================================
export 'models/api/user_api_models.dart';
export 'models/api/address_api_models.dart';

// ========================================
// SERVICES (R2.3 PROFILE DOMAIN EXTRACTION)
// ========================================
export 'services/avatar_cache_service.dart'; // Avatar caching - replaces shared UserAvatarApiService
export 'services/user_lookup_service.dart'; // User lookup - replaces shared UserSearchApiService

// ========================================
// PROVIDERS (R4.3 PLACEMENT CONSISTENCY)
// ========================================
export 'profile_providers.dart'; // Data layer providers
