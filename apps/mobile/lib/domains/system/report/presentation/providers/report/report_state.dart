/// Report State
///
/// State management for report functionality.
library;

import 'package:labuda/domains/system/report/domain/entities/entities.dart';

/// Report Actions State
class ReportActionsState {
  final bool isLoading;
  final String? error;
  final Report? lastReport;

  const ReportActionsState({
    this.isLoading = false,
    this.error,
    this.lastReport,
  });

  ReportActionsState copyWith({
    bool? isLoading,
    String? error,
    Report? lastReport,
  }) {
    return ReportActionsState(
      isLoading: isLoading ?? this.isLoading,
      error: error,
      lastReport: lastReport ?? this.lastReport,
    );
  }
}

/// Report List State (for user reports)
class ReportListState {
  final List<Report> reports;
  final bool isLoading;
  final String? error;

  const ReportListState({
    this.reports = const [],
    this.isLoading = false,
    this.error,
  });

  ReportListState copyWith({
    List<Report>? reports,
    bool? isLoading,
    String? error,
  }) {
    return ReportListState(
      reports: reports ?? this.reports,
      isLoading: isLoading ?? this.isLoading,
      error: error,
    );
  }
}
