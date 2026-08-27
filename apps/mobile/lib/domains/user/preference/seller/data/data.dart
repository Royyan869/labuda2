/// Data Layer Barrel File
///
/// Exports all data layer components.
///
/// ⚠️ ATURAN: Datasource & Repository Implementation TIDAK BOLEH di-export
library;

// DTOs
export 'dto/seller_dto.dart';

// Mappers
export 'mappers/seller_mapper.dart';

// Services
export 'services/store_photo_upload_service.dart';

// ========================================
// PROVIDERS (R4.3 PLACEMENT CONSISTENCY)
// ========================================
export 'seller_providers.dart'; // Data layer providers

// ❌ DILARANG: Export datasource/repository impl
// export 'remote/seller_remote_datasource.dart';
// export 'repositories/seller_repository_impl.dart';
