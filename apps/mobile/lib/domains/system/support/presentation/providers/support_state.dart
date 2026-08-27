library;

/// Application State for Support Module
/// Definisikan state untuk Riverpod Notifier
/// Pure Dart - bebas dari Firebase/Flutter

import 'package:labuda/domains/system/support/domain/domain.dart';

// ============================================
// BASE STATE
// ============================================

/// Base state untuk support operations
abstract class SupportState {
  const SupportState();
}

/// Initial state
class SupportInitial extends SupportState {
  const SupportInitial();
}

/// Loading state
class SupportLoading extends SupportState {
  const SupportLoading();
}

/// Success state with data
class SupportLoaded<T> extends SupportState {
  final T data;
  const SupportLoaded(this.data);
}

/// Error state
class SupportError extends SupportState {
  final String message;
  final SupportFailure? failure;

  const SupportError(this.message, {this.failure});
}

// ============================================
// TICKET LIST STATE
// ============================================

/// State untuk ticket list
class TicketListState extends SupportState {
  final List<SupportTicket> tickets;
  final bool isLoading;
  final String? errorMessage;

  const TicketListState({
    this.tickets = const [],
    this.isLoading = false,
    this.errorMessage,
  });

  TicketListState copyWith({
    List<SupportTicket>? tickets,
    bool? isLoading,
    String? errorMessage,
  }) {
    return TicketListState(
      tickets: tickets ?? this.tickets,
      isLoading: isLoading ?? this.isLoading,
      errorMessage: errorMessage,
    );
  }
}

// ============================================
// TICKET DETAIL STATE
// ============================================

/// State untuk single ticket
class TicketDetailState extends SupportState {
  final SupportTicket? ticket;
  final bool isLoading;
  final String? errorMessage;

  const TicketDetailState({
    this.ticket,
    this.isLoading = false,
    this.errorMessage,
  });

  TicketDetailState copyWith({
    SupportTicket? ticket,
    bool? isLoading,
    String? errorMessage,
  }) {
    return TicketDetailState(
      ticket: ticket ?? this.ticket,
      isLoading: isLoading ?? this.isLoading,
      errorMessage: errorMessage,
    );
  }
}

// ============================================
// CREATE TICKET STATE
// ============================================

/// State untuk create ticket operation
class CreateTicketState extends SupportState {
  final bool isLoading;
  final String? createdTicketId;
  final String? errorMessage;

  const CreateTicketState({
    this.isLoading = false,
    this.createdTicketId,
    this.errorMessage,
  });

  CreateTicketState copyWith({
    bool? isLoading,
    String? createdTicketId,
    String? errorMessage,
  }) {
    return CreateTicketState(
      isLoading: isLoading ?? this.isLoading,
      createdTicketId: createdTicketId ?? this.createdTicketId,
      errorMessage: errorMessage,
    );
  }
}

// ============================================
// CLAIM TICKET STATE
// ============================================

/// State untuk claim ticket operation
class ClaimTicketState extends SupportState {
  final bool isClaiming;
  final String? errorMessage;

  const ClaimTicketState({this.isClaiming = false, this.errorMessage});

  ClaimTicketState copyWith({bool? isClaiming, String? errorMessage}) {
    return ClaimTicketState(
      isClaiming: isClaiming ?? this.isClaiming,
      errorMessage: errorMessage,
    );
  }
}

// ============================================
// STATISTICS STATE
// ============================================

/// State untuk support statistics
class StatisticsState extends SupportState {
  final SupportTicketStatistics? statistics;
  final bool isLoading;
  final String? errorMessage;

  const StatisticsState({
    this.statistics,
    this.isLoading = false,
    this.errorMessage,
  });

  StatisticsState copyWith({
    SupportTicketStatistics? statistics,
    bool? isLoading,
    String? errorMessage,
  }) {
    return StatisticsState(
      statistics: statistics ?? this.statistics,
      isLoading: isLoading ?? this.isLoading,
      errorMessage: errorMessage,
    );
  }
}
