library;

/// Data Layer Barrel File for Support Module
/// Exports all data layer components

// DTOs
export 'dto/support_ticket_dto.dart';
export 'dto/support_message_dto.dart';

// Remote - API Datasource (Go Backend)
export 'datasources/support_api_datasource.dart';

// Repository Implementation - API version
export 'repositories/support_repository_api.dart';
