/// Unified Attachment Module
/// Single source of truth for all attachment entities, mappers, and widgets
///
/// Usage:
/// ```dart
/// import 'package:hishumi/domains/chat/attachment/attachment.dart';
/// ```
library;

// ===== ENTITIES =====
export 'package:hishumi/shared/attachment/entities/attachment.dart';
export 'package:hishumi/shared/attachment/entities/share_reference.dart';

// ===== MAPPERS =====
export 'mappers/attachment_mapper.dart';

// ===== TRUTH RESOLUTION =====
export 'package:hishumi/domains/commerce/catalog/shared/attachment_truth_resolver.dart';

// ===== LIVE STATUS PROVIDER ===== (purged: snapshot authority, zero consumers)
