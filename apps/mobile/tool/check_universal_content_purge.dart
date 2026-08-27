/// Anti-Resurrection Contract — Universal Content Mobile Hard Purge
///
/// Run this script after every PR to ensure no Post/Request/fulfilled concepts
/// are reintroduced into the mobile codebase.
///
/// Usage: dart run tool/check_universal_content_purge.dart
///
/// Returns exit code 0 if clean, 1 if violations found.
///
/// Self-test: dart run tool/check_universal_content_purge.dart --self-test
/// Creates temporary violation fixtures, verifies detection, and cleans up.

import 'dart:io';

const _projectRoot = '.';

/// Patterns that MUST NOT appear in lib/ or test/ (case-insensitive regex).
/// False-positive exceptions are documented per-pattern.
const _forbidden = <_ForbiddenPattern>[
  // --- ContentType enum (the deleted domain enum) ---
  _ForbiddenPattern(
    r'\bContentType\.(post|request)\b',
    'ContentType.post or ContentType.request reference',
    allowedIn: ['c1b_create_content_type_and_comment_authority_test.dart'],
    note:
        'c1b test is a negative anti-resurrection contract test. '
        'The deleted domain ContentType enum (post/request) must not appear in production code.',
  ),

  // --- ContentStatus.fulfilled ---
  _ForbiddenPattern(
    r'\bContentStatus\.fulfilled\b',
    'ContentStatus.fulfilled reference',
    allowedIn: ['request_fulfill_contract_test.dart'],
    note: 'negative anti-resurrection contract test',
  ),
  _ForbiddenPattern(
    r"ContentStatus\.fromApiString\(.*'fulfilled'",
    'fulfilled string in fromApiString',
  ),

  // --- isRequest / isPost ---
  _ForbiddenPattern(r'\bisRequest\b', 'isRequest variable/field'),
  _ForbiddenPattern(r'\bisPost\b', 'isPost variable/field (not HTTP POST method)'),

  // --- createRequest / CreateRequest ---
  _ForbiddenPattern(
    r'\bcreateRequest\b',
    'createRequest reference',
    allowedIn: ['create_content_route_contract_test.dart'],
    note: 'negative anti-resurrection contract test',
  ),
  _ForbiddenPattern(
    r'\bCreateRequest\b',
    'CreateRequest class/type reference',
    allowedIn: ['create_content_route_contract_test.dart'],
    note: 'negative anti-resurrection contract test',
  ),

  // --- FulfillRequest / markAsFulfilled ---
  _ForbiddenPattern(
    r'\bfulfillRequest\b',
    'fulfillRequest method reference',
    allowedIn: ['request_fulfill_contract_test.dart'],
    note: 'negative anti-resurrection contract test',
  ),
  _ForbiddenPattern(
    r'\bFulfillRequest\b',
    'FulfillRequest type reference',
    allowedIn: ['request_fulfill_contract_test.dart'],
    note: 'negative anti-resurrection contract test',
  ),
  _ForbiddenPattern(r'\bmarkAsFulfilled\b', 'markAsFulfilled reference'),
  _ForbiddenPattern(r'\bcanFulfill\b', 'canFulfill reference'),
  _ForbiddenPattern(r'\bwasFulfilled\b', 'wasFulfilled reference'),

  // --- postId / requestId ---
  _ForbiddenPattern(
    r'\bpostId\b',
    'postId variable/field',
    allowedIn: ['comment.dart', 'attachment_widget.dart', 'content_notification_navigation_behavioral_test.dart'],
    note:
        'comment.dart postId is a generic comment-target reference; '
        'attachment_widget.dart postId is a generic content-target reference; '
        'behavioral test is a negative anti-resurrection contract test.',
  ),
  _ForbiddenPattern(
    r'\brequestId\b',
    'requestId variable/field',
    allowedIn: ['content_notification_navigation_behavioral_test.dart'],
    note: 'negative anti-resurrection contract test',
  ),

  // --- Old routes ---
  _ForbiddenPattern(
    r'/create/request',
    '/create/request route string',
    allowedIn: ['create_content_route_contract_test.dart'],
    note: 'negative anti-resurrection contract test',
  ),

  // --- type = post/request/content in JSON payloads ---
  _ForbiddenPattern(r'"type":\s*"post"', '"type": "post" JSON payload'),
  _ForbiddenPattern(r'"type":\s*"request"', '"type": "request" JSON payload'),
  _ForbiddenPattern(r'"type":\s*"content"', '"type": "content" JSON payload (positive fixture residue)'),
  _ForbiddenPattern(r"\x27type\x27:\s*\x27post\x27", "'type': 'post' JSON payload"),
  _ForbiddenPattern(r"\x27type\x27:\s*\x27request\x27", "'type': 'request' JSON payload"),
  _ForbiddenPattern(r"\x27type\x27:\s*\x27content\x27", "'type': 'content' JSON payload (positive fixture residue)"),

  // --- type=post/request query params ---
  _ForbiddenPattern(r'type=post', 'type=post query parameter'),
  _ForbiddenPattern(r'type=request', 'type=request query parameter'),

  // --- case 'post': / case 'request': switch literals ---
  _ForbiddenPattern(
    r"case .post.:",
    "case 'post': switch literal (content domain)",
    allowedIn: ['fcm_action_mapper.dart', 'content_notification_navigation_behavioral_test.dart'],
    note:
        'fcm_action_mapper.dart request case is social-graph follow-request alias; '
        'behavioral test is a negative anti-resurrection contract test.',
  ),
  _ForbiddenPattern(
    r"case .request.:",
    "case 'request': switch literal (content domain)",
    allowedIn: ['fcm_action_mapper.dart', 'content_notification_navigation_behavioral_test.dart'],
    note:
        'fcm_action_mapper.dart request case is social-graph follow-request alias; '
        'behavioral test is a negative anti-resurrection contract test.',
  ),

  // --- ExternalShareType.post / ExternalShareType.request ---
  _ForbiddenPattern(r'\bExternalShareType\.post\b', 'ExternalShareType.post reference'),
  _ForbiddenPattern(r'\bExternalShareType\.request\b', 'ExternalShareType.request reference'),

  // --- PopupMoreOptionsContentType.post ---
  _ForbiddenPattern(r'\bPopupMoreOptionsContentType\.post\b', 'PopupMoreOptionsContentType.post reference'),

  // --- Lihat Post / Buat Content copies ---
  _ForbiddenPattern(r'Lihat Post', '"Lihat Post" UI copy'),
  _ForbiddenPattern(r'\bBuat Content\b', '"Buat Content" UI copy (should be Buat Konten)'),

  // --- seller.response notification ---
  _ForbiddenPattern(r'\bisSellerResponse\b', 'isSellerResponse variable/field name'),
  _ForbiddenPattern(
    r'seller\.response',
    'seller.response notification type string',
    allowedIn: ['order_api_response_dtos.dart', 'content_notification_navigation_behavioral_test.dart'],
    note:
        'order_api_response_dtos.dart sellerResponse is a commerce order field; '
        'behavioral test is a negative anti-resurrection contract test.',
  ),
  _ForbiddenPattern(
    r'\bseller_response\b',
    'seller_response wire key',
    allowedIn: ['order_api_response_dtos.dart', 'content_notification_navigation_behavioral_test.dart'],
    note:
        'Commerce order seller_response field; '
        'behavioral test is a negative anti-resurrection contract test.',
  ),
  _ForbiddenPattern(r'\bTypeSellerResponse\b', 'TypeSellerResponse type reference'),
  _ForbiddenPattern(
    r'\bSellerResponse\b',
    'SellerResponse type/class reference',
    allowedIn: ['order_api_response_dtos.dart', 'content_notification_navigation_behavioral_test.dart'],
    note:
        'order_api_response_dtos.dart SellerResponse is a commerce order DTO; '
        'behavioral test is a negative anti-resurrection contract test.',
  ),
  _ForbiddenPattern(r'\brequest_creator_id\b', 'request_creator_id wire key'),

  // --- Indonesian UI copy ---
  _ForbiddenPattern(r'Minta Koi', '"Minta Koi" UI copy'),
  _ForbiddenPattern(r'Bagikan Post', '"Bagikan Post" UI copy'),
  _ForbiddenPattern(r'\bBuat Post\b', '"Buat Post" UI copy'),
  _ForbiddenPattern(r'Tandai Terpenuhi', '"Tandai Terpenuhi" UI copy'),
  _ForbiddenPattern(r'Mark as Fulfilled', '"Mark as Fulfilled" UI copy'),
  _ForbiddenPattern(r'Sudah Terpenuhi', '"Sudah Terpenuhi" UI copy'),
  _ForbiddenPattern(r'Respons untuk Request', '"Respons untuk Request" UI copy'),
  _ForbiddenPattern(r'Request telah terpenuhi', '"Request telah terpenuhi" UI copy'),

  // --- FeedItemType.post / FeedItemType.request ---
  _ForbiddenPattern(r'\bFeedItemType\.post\b', 'FeedItemType.post reference'),
  _ForbiddenPattern(r'\bFeedItemType\.request\b', 'FeedItemType.request reference'),

  // --- Post/Request notification routing ---
  _ForbiddenPattern(r"targetType\s*==\s*'post'", "notification targetType == 'post'"),
  _ForbiddenPattern(r"targetType\s*==\s*'request'", "notification targetType == 'request'"),

  // --- UploadTaskType.post / UploadTaskType.request ---
  _ForbiddenPattern(r'\bUploadTaskType\.post\b', 'UploadTaskType.post reference'),
  _ForbiddenPattern(r'\bUploadTaskType\.request\b', 'UploadTaskType.request reference'),

  // --- Old route paths /post/ /request/ ---
  _ForbiddenPattern(
    r"'/post/",
    "'/post/ route path string",
    allowedIn: ['content_notification_navigation_behavioral_test.dart'],
    note: 'negative anti-resurrection contract test',
  ),
  _ForbiddenPattern(
    r"'/request/",
    "'/request/ route path string",
    allowedIn: ['content_notification_navigation_behavioral_test.dart'],
    note: 'negative anti-resurrection contract test',
  ),

  // --- content type default fallback ---
  _ForbiddenPattern(r"json\['type'\]\s*\?\?\s*'post'", "json['type'] ?? 'post' fallback"),
  _ForbiddenPattern(r'\bContentType\.values\.byName\b', 'ContentType.values.byName parsing'),

  // --- request -> content mapping ---
  _ForbiddenPattern(
    r"'request'\s*->\s*'content'",
    "'request' -> 'content' compatibility mapping in Dart code",
    note: 'Narrow match — only catches the Dart map syntax specifically.',
  ),
];

/// Paths excluded from ALL pattern checks.
/// Each entry must have an exact semantic reason.
const _falsePositivePaths = [
  // MIME content-type headers (not Content domain type)
  's3_service.dart',
  // MIME Content-Type in chat repo file upload
  'chat_repository_impl.dart',
  // HTTP POST method (not Content post)
  'api_client.dart',
  // Commerce listing status (not content status)
  'listing.dart',
  'listing_dto.dart',
  // Payment domain createRequest (NOT content domain createRequest method)
  'payment_initiation_notifier.dart',
  // Markdown/docs files
  'README.md',
  '.md',
];

class _ForbiddenPattern {
  final String pattern;
  final String description;
  final List<String> allowedIn;
  final String? note;

  const _ForbiddenPattern(
    this.pattern,
    this.description, {
    this.allowedIn = const [],
    this.note,
  });
}

class _Violation {
  final String file;
  final int line;
  final String pattern;
  final String description;

  const _Violation(this.file, this.line, this.pattern, this.description);

  @override
  String toString() => '  $file:$line — $description';
}

List<_Violation> _scan(List<String> scanDirs) {
  final violations = <_Violation>[];

  for (final basePath in scanDirs) {
    final dir = Directory('$_projectRoot/$basePath');
    if (!dir.existsSync()) continue;
    for (final entity in dir.listSync(recursive: true)) {
      if (entity is! File) continue;
      if (!entity.path.endsWith('.dart')) continue;

      final relPath = entity.path.replaceAll('\\', '/');
      if (_falsePositivePaths.any((fp) => relPath.contains(fp))) continue;

      try {
        final lines = entity.readAsLinesSync();
        for (var i = 0; i < lines.length; i++) {
          final line = lines[i];
          for (final fp in _forbidden) {
            if (fp.allowedIn.any((a) => relPath.contains(a))) continue;
            if (RegExp(fp.pattern, caseSensitive: false).hasMatch(line)) {
              violations.add(_Violation(relPath, i + 1, fp.pattern, fp.description));
            }
          }
        }
      } catch (_) {
        // skip unreadable files
      }
    }
  }

  return violations;
}

void _runSelfTest() {
  stdout.writeln('=== SELF-TEST: Creating temporary violation fixtures ===');
  final tmpDir = Directory('$_projectRoot/test/.purge_self_test_tmp');
  if (tmpDir.existsSync()) {
    tmpDir.deleteSync(recursive: true);
  }
  tmpDir.createSync(recursive: true);

  // Test 1: ContentType.post reference should be detected
  File('${tmpDir.path}/test_content_type_post.dart').writeAsStringSync('''
// This file intentionally contains ContentType.post for self-test
final x = ContentType.post;
''');

  // Test 2: "type":"post" JSON payload should be detected
  File('${tmpDir.path}/test_type_post_json.dart').writeAsStringSync('''
const payload = \'{"type":"post","caption":"test"}\';
''');

  // Test 3: "type":"content" JSON payload should be detected
  File('${tmpDir.path}/test_type_content_json.dart').writeAsStringSync('''
final json = <String, dynamic>{"type": "content", "caption": "test"};
''');

  // Test 4: 'type': 'content' single-quoted JSON should be detected
  File('${tmpDir.path}/test_type_content_single_quote.dart').writeAsStringSync('''
final json = <String, dynamic>{'type': 'content', 'caption': 'test'};
''');

  // Test 5: Buat Content (wrong Indonesian label) should be detected
  File('${tmpDir.path}/test_buat_content.dart').writeAsStringSync('''
final label = "Buat Content";
''');

  // Test 6: Clean file should NOT produce violations
  File('${tmpDir.path}/test_clean.dart').writeAsStringSync('''
// This file is clean and should produce no violations
final caption = "Hello world";
final visibility = "public";
''');

  final violations = _scan(['test/.purge_self_test_tmp']);

  var failures = 0;

  // Test 1-5 should each produce at least one violation
  final expectedViolationFiles = [
    'test_content_type_post.dart',
    'test_type_post_json.dart',
    'test_type_content_json.dart',
    'test_type_content_single_quote.dart',
    'test_buat_content.dart',
  ];

  for (final expectedFile in expectedViolationFiles) {
    final hasViolation = violations.any((v) => v.file.contains(expectedFile));
    if (!hasViolation) {
      stderr.writeln('SELF-TEST FAIL: $expectedFile should have produced a violation but did not');
      failures++;
    } else {
      stdout.writeln('  ✓ $expectedFile detected');
    }
  }

  // Test 6 should NOT produce violations
  final cleanViolation = violations.any((v) => v.file.contains('test_clean.dart'));
  if (cleanViolation) {
    stderr.writeln('SELF-TEST FAIL: test_clean.dart should NOT produce violations');
    failures++;
  } else {
    stdout.writeln('  ✓ test_clean.dart correctly passed');
  }

  // Cleanup
  tmpDir.deleteSync(recursive: true);

  if (failures > 0) {
    stderr.writeln('\nSELF-TEST FAILED: $failures failure(s)');
    exit(1);
  }
  stdout.writeln('\n✅ SELF-TEST PASSED: All violation patterns detected correctly');
}

void main(List<String> args) {
  if (args.contains('--self-test')) {
    _runSelfTest();
    return;
  }

  final violations = _scan(['lib', 'test']);

  if (violations.isEmpty) {
    stdout.writeln('✅ ANTI-RESURRECTION CHECK PASSED');
    stdout.writeln('   Zero forbidden Post/Request/fulfilled patterns found in lib/ and test/.');
    exit(0);
  }

  stderr.writeln('❌ ANTI-RESURRECTION CHECK FAILED');
  stderr.writeln('   ${violations.length} violation(s) found:\n');
  for (final v in violations) {
    stderr.writeln(v.toString());
  }
  exit(1);
}
