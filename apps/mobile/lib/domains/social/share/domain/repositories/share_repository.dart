// Share Repository Interface
//
// HONESTY CLEANUP v1:
// - Share tracking methods (incrementShareCount, getShareStats) removed
// - Backend does not implement /social/shares endpoints
// - Only canonical share operations are preserved

import '../entities/share_target.dart';
import '../entities/share_destination.dart';
import '../entities/share_result.dart';
import '../entities/share_failure.dart';
import 'package:dartz/dartz.dart';

/// Repository interface for share operations
/// Following Clean Architecture principles
/// Domain layer - pure interface, no implementation details
abstract class ShareRepository {
  /// Share content via external platforms (WhatsApp, Instagram, etc.)
  /// Uses native OS share dialog or platform-specific APIs
  Future<Either<ShareFailure, ShareResult>> shareViaExternal({
    required ShareTarget target,
    required ShareDestinationType destination,
  });

  /// Share content as new Post in feed (Repost)
  /// Creates a new Post with shared content embedded
  /// Returns the created Post ID or failure
  Future<Either<ShareFailure, String>> shareAsPost({
    required ShareTarget target,
    required String authorId,
    String? caption,
  });

  /// Send content via LABUDA internal chat
  Future<Either<ShareFailure, ShareResult>> sendToChat({
    required ShareTarget target,
    required String recipientUserId,
    String? message,
  });
}
