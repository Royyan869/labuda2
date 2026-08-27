// Data barrel file - exports all public data items
//
// MIGRATED: Now uses Go Backend API (Firestore version removed)

// DTO
export 'dto/share_dto.dart';

// Remote datasources - API-based
export 'datasources/share_api_datasource.dart';
export 'remote/native_share_service.dart';

// Repository implementation - API-based
export 'repositories/share_repository_api.dart';
