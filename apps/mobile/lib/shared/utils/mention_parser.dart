import 'package:equatable/equatable.dart';

/// Parser untuk extract dan parse @mentions dari text
///
/// Supports:
/// - @username mentions
/// - Special mentions (@everyone, @admins)
/// - Multiple mentions dalam satu text
class MentionParser {
  /// Regex untuk match @username (alphanumeric + underscore)
  static final RegExp mentionRegex = RegExp(r'@(\w+)');

  /// Special mention keywords
  static const String everyoneMention = '@everyone';
  static const String adminsMention = '@admins';
  static const String onlineMention = '@online';

  /// Extract all usernames yang di-mention dari text
  ///
  /// Example:
  /// ```dart
  /// extractMentions("Hi @john and @jane!")
  /// // Returns: ["john", "jane"]
  /// ```
  static List<String> extractMentions(String text) {
    if (text.isEmpty) return [];

    return mentionRegex
        .allMatches(text)
        .map((match) => match.group(1)!)
        .where((username) => !_isSpecialMention('@$username'))
        .toList();
  }

  /// Extract special mentions dari text
  ///
  /// Returns list of special mention types found
  static List<String> extractSpecialMentions(String text) {
    if (text.isEmpty) return [];

    final mentions = <String>[];
    if (text.contains(everyoneMention)) mentions.add('everyone');
    if (text.contains(adminsMention)) mentions.add('admins');
    if (text.contains(onlineMention)) mentions.add('online');

    return mentions;
  }

  /// Check if a mention is a special mention
  static bool _isSpecialMention(String mention) {
    return mention == everyoneMention ||
        mention == adminsMention ||
        mention == onlineMention;
  }

  /// Parse text into segments dengan mention information
  ///
  /// Example:
  /// ```dart
  /// parseText("Hi @john and @jane!")
  /// // Returns:
  /// // [
  /// //   MentionSegment(text: "Hi ", isMention: false),
  /// //   MentionSegment(text: "@john", isMention: true, username: "john"),
  /// //   MentionSegment(text: " and ", isMention: false),
  /// //   MentionSegment(text: "@jane", isMention: true, username: "jane"),
  /// //   MentionSegment(text: "!", isMention: false),
  /// // ]
  /// ```
  static List<MentionSegment> parseText(String text) {
    if (text.isEmpty) return [];

    final segments = <MentionSegment>[];
    var lastEnd = 0;

    for (final match in mentionRegex.allMatches(text)) {
      // Add text before mention
      if (match.start > lastEnd) {
        segments.add(
          MentionSegment(
            text: text.substring(lastEnd, match.start),
            isMention: false,
          ),
        );
      }

      // Add mention
      final username = match.group(1)!;
      final mentionText = match.group(0)!; // Includes @

      segments.add(
        MentionSegment(
          text: mentionText,
          isMention: true,
          username: username,
          isSpecialMention: _isSpecialMention(mentionText),
        ),
      );

      lastEnd = match.end;
    }

    // Add remaining text
    if (lastEnd < text.length) {
      segments.add(
        MentionSegment(text: text.substring(lastEnd), isMention: false),
      );
    }

    return segments;
  }

  /// Check if text contains any mentions
  static bool hasMentions(String text) {
    return mentionRegex.hasMatch(text);
  }

  /// Count total mentions dalam text
  static int countMentions(String text) {
    return mentionRegex.allMatches(text).length;
  }

  /// Replace mentions dengan display names
  ///
  /// Example:
  /// ```dart
  /// replaceMentions(
  ///   "Hi @john!",
  ///   {"john": "John Doe"}
  /// )
  /// // Returns: "Hi John Doe!"
  /// ```
  static String replaceMentions(
    String text,
    Map<String, String> usernameToDisplayName,
  ) {
    return text.replaceAllMapped(mentionRegex, (match) {
      final username = match.group(1)!;
      final displayName = usernameToDisplayName[username];
      return displayName ?? match.group(0)!;
    });
  }
}

/// Segment dari parsed text dengan mention information
class MentionSegment extends Equatable {
  final String text;
  final bool isMention;
  final String? username;
  final String? userId; // Resolved later via provider
  final bool isSpecialMention;

  const MentionSegment({
    required this.text,
    required this.isMention,
    this.username,
    this.userId,
    this.isSpecialMention = false,
  });

  MentionSegment copyWith({
    String? text,
    bool? isMention,
    String? username,
    String? userId,
    bool? isSpecialMention,
  }) {
    return MentionSegment(
      text: text ?? this.text,
      isMention: isMention ?? this.isMention,
      username: username ?? this.username,
      userId: userId ?? this.userId,
      isSpecialMention: isSpecialMention ?? this.isSpecialMention,
    );
  }

  @override
  List<Object?> get props => [
    text,
    isMention,
    username,
    userId,
    isSpecialMention,
  ];

  @override
  String toString() {
    if (isMention) {
      return 'MentionSegment(text: $text, username: $username, special: $isSpecialMention)';
    }
    return 'MentionSegment(text: $text)';
  }
}

/// Mention data untuk caching
class MentionData extends Equatable {
  final String userId;
  final String username;
  final String? avatarUrl;

  const MentionData({
    required this.userId,
    required this.username,
    this.avatarUrl,
  });

  Map<String, dynamic> toMap() {
    return {'userId': userId, 'username': username, 'avatarUrl': avatarUrl};
  }

  factory MentionData.fromMap(Map<String, dynamic> map) {
    return MentionData(
      userId: map['userId'] as String,
      username: map['username'] as String,
      avatarUrl: map['avatarUrl'] as String?,
    );
  }

  @override
  List<Object?> get props => [userId, username, avatarUrl];
}
