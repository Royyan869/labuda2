import 'package:flutter_riverpod/flutter_riverpod.dart';

/// Upload progress state untuk modular management
class UploadProgressState {
  final Map<String, UploadTaskProgress> activeUploads;

  const UploadProgressState({this.activeUploads = const {}});

  UploadProgressState copyWith({
    Map<String, UploadTaskProgress>? activeUploads,
  }) {
    return UploadProgressState(
      activeUploads: activeUploads ?? this.activeUploads,
    );
  }
}

/// Individual upload task progress
class UploadTaskProgress {
  final String taskId;
  final UploadTaskType type;
  final String description;
  final double progress;
  final UploadTaskStatus status;
  final String? errorMessage;
  final int currentStep;
  final int totalSteps;

  const UploadTaskProgress({
    required this.taskId,
    required this.type,
    required this.description,
    this.progress = 0.0,
    this.status = UploadTaskStatus.pending,
    this.errorMessage,
    this.currentStep = 0,
    this.totalSteps = 0,
  });

  UploadTaskProgress copyWith({
    String? taskId,
    UploadTaskType? type,
    String? description,
    double? progress,
    UploadTaskStatus? status,
    String? errorMessage,
    int? currentStep,
    int? totalSteps,
  }) {
    return UploadTaskProgress(
      taskId: taskId ?? this.taskId,
      type: type ?? this.type,
      description: description ?? this.description,
      progress: progress ?? this.progress,
      status: status ?? this.status,
      errorMessage: errorMessage ?? this.errorMessage,
      currentStep: currentStep ?? this.currentStep,
      totalSteps: totalSteps ?? this.totalSteps,
    );
  }
}

enum UploadTaskType { post, request, listing, auction }

enum UploadTaskStatus { pending, uploading, processing, completed, failed }

/// Provider untuk upload progress management
class UploadProgressNotifier extends Notifier<UploadProgressState> {
  @override
  UploadProgressState build() {
    return const UploadProgressState();
  }

  /// Start new upload task
  void startUpload({
    required String taskId,
    required UploadTaskType type,
    required String description,
    required int totalSteps,
  }) {
    final task = UploadTaskProgress(
      taskId: taskId,
      type: type,
      description: description,
      totalSteps: totalSteps,
      status: UploadTaskStatus.pending,
    );

    state = state.copyWith(
      activeUploads: {...state.activeUploads, taskId: task},
    );
  }

  /// Update upload progress
  void updateProgress({
    required String taskId,
    required double progress,
    required int currentStep,
    UploadTaskStatus? status,
    String? stepDescription,
  }) {
    final currentTask = state.activeUploads[taskId];
    if (currentTask == null) return;

    final updatedTask = currentTask.copyWith(
      progress: progress,
      currentStep: currentStep,
      status: status ?? currentTask.status,
      description: stepDescription ?? currentTask.description,
    );

    state = state.copyWith(
      activeUploads: {...state.activeUploads, taskId: updatedTask},
    );
  }

  /// Mark upload as completed
  void completeUpload(String taskId) {
    final currentTask = state.activeUploads[taskId];
    if (currentTask == null) return;

    final completedTask = currentTask.copyWith(
      progress: 1.0,
      status: UploadTaskStatus.completed,
      currentStep: currentTask.totalSteps,
    );

    state = state.copyWith(
      activeUploads: {...state.activeUploads, taskId: completedTask},
    );

    // Remove completed task after delay
    Future.delayed(const Duration(seconds: 3), () {
      removeUpload(taskId);
    });
  }

  /// Mark upload as failed
  void failUpload(String taskId, String errorMessage) {
    final currentTask = state.activeUploads[taskId];
    if (currentTask == null) return;

    final failedTask = currentTask.copyWith(
      status: UploadTaskStatus.failed,
      errorMessage: errorMessage,
    );

    state = state.copyWith(
      activeUploads: {...state.activeUploads, taskId: failedTask},
    );
  }

  /// Remove upload task
  void removeUpload(String taskId) {
    final updatedUploads = Map<String, UploadTaskProgress>.from(
      state.activeUploads,
    );
    updatedUploads.remove(taskId);

    state = state.copyWith(activeUploads: updatedUploads);
  }

  /// Clear all uploads
  void clearAll() {
    state = const UploadProgressState();
  }
}

/// Provider instance
final uploadProgressProvider =
    NotifierProvider<UploadProgressNotifier, UploadProgressState>(
      () => UploadProgressNotifier(),
    );
