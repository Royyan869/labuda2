import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/common/result.dart';
import 'package:hishumi/domains/social/comment/domain/entities/comment.dart';
import 'package:hishumi/domains/social/comment/domain/repositories/comment_repository.dart';
import 'package:hishumi/domains/social/comment/presentation/providers/comment_notifier.dart';
import 'package:hishumi/domains/social/comment/presentation/providers/comment_providers.dart';

// ============================================================================
// COMMENT PENDING ROW CONTRACT
//
// Tapping Send must be visible. The composer's row appears BEFORE the request
// and is not a `Comment`: the domain list stays server truth, so a failed send
// can never masquerade as a persisted comment. A failure stays on screen with
// its own retry instead of a snackbar the user has already missed.
// ============================================================================

const _contentId = 'content_1';

class _CommentRepo implements CommentRepository {
  bool failCreate = false;
  int createCalls = 0;

  @override
  Future<Result<Comment>> createComment({
    required String targetId,
    required CommentTargetType targetType,
    required String content,
    String? parentId,
    List<String> mentionedUserIds = const [],
    List<String> mediaUrls = const [],
  }) async {
    createCalls++;
    if (failCreate) return Result.error('boom');

    return Result.success(
      Comment(
        id: 'c_$createCalls',
        authorId: 'me',
        contentId: targetId,
        authorUsername: 'me',
        type: 'normal',
        createdAt: DateTime.utc(2026, 6, 1),
      ),
    );
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

void main() {
  late _CommentRepo repo;
  late ProviderContainer container;
  late ProviderSubscription<dynamic> sub;

  setUp(() {
    repo = _CommentRepo();
    container = ProviderContainer(
      overrides: [commentRepositoryProvider.overrideWithValue(repo)],
    );
    sub = container.listen(commentProvider, (_, _) {});
    addTearDown(() {
      sub.close();
      container.dispose();
    });
  });

  test('a comment is on screen before the server confirms it', () async {
    final notifier = container.read(commentProvider.notifier);

    final inFlight = notifier.createComment(
      targetId: _contentId,
      targetType: CommentTargetType.content,
      content: 'halo',
    );

    // The row exists while the request is still in flight — and it is NOT in the
    // domain list yet.
    expect(container.read(commentProvider).pendingComments.length, 1);
    expect(
      container.read(commentProvider).pendingComments.single.content,
      'halo',
    );
    expect(container.read(commentProvider).comments, isEmpty);

    await inFlight;

    final state = container.read(commentProvider);
    expect(state.pendingComments, isEmpty);
    expect(state.comments.map((c) => c.id), ['c_1']);
  });

  test('a failed comment stays on screen, marked, and retries', () async {
    repo.failCreate = true;
    final notifier = container.read(commentProvider.notifier);

    await notifier.createComment(
      targetId: _contentId,
      targetType: CommentTargetType.content,
      content: 'gagal dulu',
    );

    var state = container.read(commentProvider);
    // A failure never becomes a persisted comment.
    expect(state.comments, isEmpty);
    expect(state.pendingComments.single.failed, isTrue);
    expect(state.pendingComments.single.content, 'gagal dulu');

    repo.failCreate = false;
    await notifier.retryPendingComment(state.pendingComments.single.id);

    state = container.read(commentProvider);
    expect(state.pendingComments, isEmpty);
    expect(state.comments.map((c) => c.id), ['c_2']);
  });

  test('a retry never leaves the pending row behind as a duplicate', () async {
    repo.failCreate = true;
    final notifier = container.read(commentProvider.notifier);

    await notifier.createComment(
      targetId: _contentId,
      targetType: CommentTargetType.content,
      content: 'x',
    );
    final pendingId = container.read(commentProvider).pendingComments.single.id;

    repo.failCreate = false;
    await notifier.retryPendingComment(pendingId);

    final state = container.read(commentProvider);
    expect(state.pendingComments, isEmpty);
    expect(state.comments.length, 1);
  });
}
