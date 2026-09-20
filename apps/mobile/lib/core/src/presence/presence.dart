import 'package:equatable/equatable.dart';

/// Canonical mobile Presence representation.
/// Mirrors backend GET /api/v1/users/presence and WS presence.changed data:
/// {user_id, is_online, last_seen_at, version}
class Presence extends Equatable {
  final String userId;
  final bool isOnline;
  final DateTime? lastSeenAt;
  final int version;

  const Presence({
    required this.userId,
    required this.isOnline,
    this.lastSeenAt,
    required this.version,
  });

  factory Presence.fromJson(Map<String, dynamic> json) {
    return Presence(
      userId: json['user_id'] as String,
      isOnline: json['is_online'] as bool,
      lastSeenAt: json['last_seen_at'] == null
          ? null
          : DateTime.parse(json['last_seen_at'] as String),
      version: (json['version'] as num).toInt(),
    );
  }

  Map<String, dynamic> toJson() => {
        'user_id': userId,
        'is_online': isOnline,
        'last_seen_at': lastSeenAt?.toIso8601String(),
        'version': version,
      };

  Presence copyWith({
    String? userId,
    bool? isOnline,
    DateTime? lastSeenAt,
    int? version,
    bool clearLastSeen = false,
  }) {
    return Presence(
      userId: userId ?? this.userId,
      isOnline: isOnline ?? this.isOnline,
      lastSeenAt: clearLastSeen ? null : (lastSeenAt ?? this.lastSeenAt),
      version: version ?? this.version,
    );
  }

  @override
  List<Object?> get props => [userId, isOnline, lastSeenAt, version];
}
