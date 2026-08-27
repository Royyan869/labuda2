/// Domain Entities for Support Module
/// Pure Dart - bebas dari Firebase, Flutter, dan external dependencies
library;

import 'package:equatable/equatable.dart';

// ============================================
// ENUMS
// ============================================

/// Support Category - kategori support ticket
enum SupportCategory {
  payment, // Masalah pembayaran
  order, // Masalah pesanan
  technical, // Masalah teknis app
  account, // Masalah akun
  general, // Pertanyaan umum
}

/// Support Priority - prioritas support ticket
enum SupportPriority {
  low, // Prioritas rendah
  medium, // Prioritas menengah (default)
  high, // Prioritas tinggi
  urgent, // Urgent - butuh respon cepat
}

/// Support Status - status support ticket
enum SupportStatus {
  open, // Baru dibuat, belum ada admin
  inProgress, // Admin sedang handle
  waitingUser, // Menunggu response user
  resolved, // Sudah diselesaikan
  closed, // Ditutup
}

/// Ticket Filter untuk admin queue
enum TicketFilter {
  all, // Semua tickets
  myTickets, // Tickets assigned ke admin
  unassigned, // Tickets belum di-assign
}

/// Sort Option untuk queue
enum SortOption {
  newest, // Terbaru di atas
  priority, // Berdasarkan priority
  waitingLongest, // Yang paling lama menunggu
}

// ============================================
// ENTITIES
// ============================================

/// Support Ticket - representasi support ticket
/// Entity ini bebas dari Firebase/Flutter - pure Dart class
class SupportTicket extends Equatable {
  final String id;
  final String userId; // User yang create ticket
  final String userName;
  final String? userAvatar;
  final SupportCategory category;
  final SupportPriority priority;
  final SupportStatus status;
  final String? linkedOrderId; // Order terkait jika ada
  final String? assignedToAdmin; // Admin ID yang handle
  final String? assignedAdminName; // Nama admin untuk display
  final DateTime createdAt;
  final DateTime? updatedAt;
  final DateTime? assignedAt;
  final DateTime? resolvedAt;
  final String? resolvedBy;
  final DateTime? firstResponseAt; // Untuk SLA tracking
  final String? lastMessage; // Preview pesan terakhir
  final DateTime? lastMessageAt;
  final bool isActive;

  const SupportTicket({
    required this.id,
    required this.userId,
    required this.userName,
    this.userAvatar,
    required this.category,
    required this.priority,
    required this.status,
    this.linkedOrderId,
    this.assignedToAdmin,
    this.assignedAdminName,
    required this.createdAt,
    this.updatedAt,
    this.assignedAt,
    this.resolvedAt,
    this.resolvedBy,
    this.firstResponseAt,
    this.lastMessage,
    this.lastMessageAt,
    this.isActive = true,
  });

  /// Helper: check apakah ticket assigned ke admin tertentu
  bool isAssignedTo(String? adminId) {
    return adminId != null && assignedToAdmin == adminId;
  }

  /// Helper: check apakah ticket unassigned
  bool get isUnassigned => assignedToAdmin == null;

  /// Helper: check apakah ticket resolved
  bool get isResolved => status == SupportStatus.resolved;

  /// Helper: check apakah ticket open (belum di-assign)
  bool get isOpen => status == SupportStatus.open;

  /// Helper: check apakah ticket sedang in progress
  bool get isInProgress => status == SupportStatus.inProgress;

  /// Helper: check apakah menunggu response user
  bool get isWaitingUser => status == SupportStatus.waitingUser;

  /// Helper: check apakah urgent
  bool get isUrgent => priority == SupportPriority.urgent;

  /// Helper: get waktu tunggu (duration since created)
  Duration get waitingDuration => DateTime.now().difference(createdAt);

  /// Helper: check apakah first response SLA terpenuhi (30 menit)
  bool get isFirstResponseSLAMet {
    if (firstResponseAt == null) return false;
    final responseTime = firstResponseAt!.difference(createdAt);
    return responseTime.inMinutes <= 30;
  }

  /// Helper: check apakah resolution SLA terpenuhi (24 jam)
  bool get isResolutionSLAMet {
    if (resolvedAt == null) return false;
    final resolutionTime = resolvedAt!.difference(createdAt);
    return resolutionTime.inHours <= 24;
  }

  @override
  List<Object?> get props => [
    id,
    userId,
    userName,
    userAvatar,
    category,
    priority,
    status,
    linkedOrderId,
    assignedToAdmin,
    assignedAdminName,
    createdAt,
    updatedAt,
    assignedAt,
    resolvedAt,
    resolvedBy,
    firstResponseAt,
    lastMessage,
    lastMessageAt,
    isActive,
  ];

  /// CopyWith untuk immutable updates
  SupportTicket copyWith({
    String? id,
    String? userId,
    String? userName,
    String? userAvatar,
    SupportCategory? category,
    SupportPriority? priority,
    SupportStatus? status,
    String? linkedOrderId,
    String? assignedToAdmin,
    String? assignedAdminName,
    DateTime? createdAt,
    DateTime? updatedAt,
    DateTime? assignedAt,
    DateTime? resolvedAt,
    String? resolvedBy,
    DateTime? firstResponseAt,
    String? lastMessage,
    DateTime? lastMessageAt,
    bool? isActive,
  }) {
    return SupportTicket(
      id: id ?? this.id,
      userId: userId ?? this.userId,
      userName: userName ?? this.userName,
      userAvatar: userAvatar ?? this.userAvatar,
      category: category ?? this.category,
      priority: priority ?? this.priority,
      status: status ?? this.status,
      linkedOrderId: linkedOrderId ?? this.linkedOrderId,
      assignedToAdmin: assignedToAdmin ?? this.assignedToAdmin,
      assignedAdminName: assignedAdminName ?? this.assignedAdminName,
      createdAt: createdAt ?? this.createdAt,
      updatedAt: updatedAt ?? this.updatedAt,
      assignedAt: assignedAt ?? this.assignedAt,
      resolvedAt: resolvedAt ?? this.resolvedAt,
      resolvedBy: resolvedBy ?? this.resolvedBy,
      firstResponseAt: firstResponseAt ?? this.firstResponseAt,
      lastMessage: lastMessage ?? this.lastMessage,
      lastMessageAt: lastMessageAt ?? this.lastMessageAt,
      isActive: isActive ?? this.isActive,
    );
  }
}

/// Create Support Ticket Request
class CreateSupportTicketRequest {
  final String userId;
  final String userName;
  final String? userAvatar;
  final SupportCategory category;
  final SupportPriority priority;
  final String? description; // Initial message/description
  final String? linkedOrderId;

  const CreateSupportTicketRequest({
    required this.userId,
    required this.userName,
    this.userAvatar,
    required this.category,
    this.priority = SupportPriority.medium,
    this.description,
    this.linkedOrderId,
  });

  /// Validate request
  bool get isValid => userId.isNotEmpty && userName.isNotEmpty;

  /// Check if description is provided
  bool get hasDescription => description != null && description!.isNotEmpty;
}

/// Claim Ticket Request
class ClaimTicketRequest {
  final String ticketId;
  final String adminId;
  final String adminDisplayName;
  final String? adminFirstName;
  final bool sendGreeting;
  final String greetingStyle; // 'friendly', 'professional', 'casual'

  const ClaimTicketRequest({
    required this.ticketId,
    required this.adminId,
    required this.adminDisplayName,
    this.adminFirstName,
    this.sendGreeting = true,
    this.greetingStyle = 'friendly',
  });

  /// Validate request
  bool get isValid =>
      ticketId.isNotEmpty && adminId.isNotEmpty && adminDisplayName.isNotEmpty;

  /// Extract first name from display name if not provided
  String get firstName => adminFirstName ?? adminDisplayName.split(' ').first;
}

/// Resolve Ticket Request
class ResolveTicketRequest {
  final String ticketId;
  final String adminId;
  final String? resolutionNotes;
  final bool sendSystemMessage;

  const ResolveTicketRequest({
    required this.ticketId,
    required this.adminId,
    this.resolutionNotes,
    this.sendSystemMessage = true,
  });

  /// Validate request
  bool get isValid => ticketId.isNotEmpty && adminId.isNotEmpty;
}

/// Reopen Ticket Request
class ReopenTicketRequest {
  final String ticketId;
  final String userId; // User yang reopen

  const ReopenTicketRequest({required this.ticketId, required this.userId});

  /// Validate request
  bool get isValid => ticketId.isNotEmpty && userId.isNotEmpty;
}

/// Close Ticket Request
class CloseTicketRequest {
  final String ticketId;
  final String adminId;

  const CloseTicketRequest({required this.ticketId, required this.adminId});

  /// Validate request
  bool get isValid => ticketId.isNotEmpty && adminId.isNotEmpty;
}

/// Support Ticket Statistics untuk admin dashboard
class SupportTicketStatistics extends Equatable {
  final int unassignedCount;
  final int myTicketsCount;
  final int openTicketsCount;
  final int resolvedTodayCount;
  final int resolvedThisWeekCount;
  final int averageResponseTimeMinutes; // Dalam hitungan menit

  const SupportTicketStatistics({
    this.unassignedCount = 0,
    this.myTicketsCount = 0,
    this.openTicketsCount = 0,
    this.resolvedTodayCount = 0,
    this.resolvedThisWeekCount = 0,
    this.averageResponseTimeMinutes = 0,
  });

  @override
  List<Object?> get props => [
    unassignedCount,
    myTicketsCount,
    openTicketsCount,
    resolvedTodayCount,
    resolvedThisWeekCount,
    averageResponseTimeMinutes,
  ];

  /// CopyWith
  SupportTicketStatistics copyWith({
    int? unassignedCount,
    int? myTicketsCount,
    int? openTicketsCount,
    int? resolvedTodayCount,
    int? resolvedThisWeekCount,
    int? averageResponseTimeMinutes,
  }) {
    return SupportTicketStatistics(
      unassignedCount: unassignedCount ?? this.unassignedCount,
      myTicketsCount: myTicketsCount ?? this.myTicketsCount,
      openTicketsCount: openTicketsCount ?? this.openTicketsCount,
      resolvedTodayCount: resolvedTodayCount ?? this.resolvedTodayCount,
      resolvedThisWeekCount:
          resolvedThisWeekCount ?? this.resolvedThisWeekCount,
      averageResponseTimeMinutes:
          averageResponseTimeMinutes ?? this.averageResponseTimeMinutes,
    );
  }
}
