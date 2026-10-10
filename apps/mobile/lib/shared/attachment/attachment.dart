/// Unified Attachment Module
/// Single source of truth for all attachment entities, mappers, and widgets
///
/// Usage:
/// ```dart
/// import 'package:hishumi/shared/attachment/attachment.dart';
/// ```
library;

// ===== ENTITIES =====
export 'entities/attachment.dart';
export 'entities/share_reference.dart';

// ===== MAPPERS =====
// Canonical mapper lives in domains/chat/attachment/mappers/attachment_mapper.dart

// ===== TRUTH RESOLUTION =====
// Moved to domains/commerce/catalog/shared/
// Import from: package:hishumi/domains/commerce/catalog/shared/attachment_truth_resolver.dart
