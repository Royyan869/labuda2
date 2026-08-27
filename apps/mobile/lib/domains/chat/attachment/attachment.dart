/// Unified Attachment Module
/// Single source of truth for all attachment entities, mappers, and widgets
///
/// Usage:
/// ```dart
/// import 'package:labuda/domains/chat/attachment/attachment.dart';
/// ```
library;

// ===== ENTITIES =====
export 'package:labuda/shared/attachment/entities/attachment.dart';
export 'package:labuda/shared/attachment/entities/share_reference.dart';

// ===== MAPPERS =====
export 'mappers/attachment_mapper.dart';

// ===== TRUTH RESOLUTION =====
export 'package:labuda/domains/commerce/catalog/shared/attachment_truth_resolver.dart';

// ===== LIVE STATUS PROVIDER =====
export 'package:labuda/domains/commerce/catalog/shared/live_status_provider.dart';
