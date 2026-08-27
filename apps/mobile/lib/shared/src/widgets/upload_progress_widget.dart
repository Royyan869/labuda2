// Dart
// Flutter
import 'package:flutter/material.dart';

// External
import 'package:flutter_riverpod/flutter_riverpod.dart';

// Internal
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/src/providers/upload_progress_provider.dart';
import 'package:labuda/shared/src/widgets/upload_task_utils.dart';

/// Widget untuk menampilkan upload progress di home screen
class UploadProgressWidget extends ConsumerWidget {
  const UploadProgressWidget({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final uploadState = ref.watch(uploadProgressProvider);
    final isDark = Theme.of(context).brightness == Brightness.dark;

    if (uploadState.activeUploads.isEmpty) {
      return const SizedBox.shrink();
    }

    return Container(
      margin: const EdgeInsets.fromLTRB(12, 8, 12, 0),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray700 : AppColors.neutralGray50,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: isDark ? AppColors.neutralGray600 : AppColors.neutralGray200,
          width: 1,
        ),
      ),
      child: Column(
        children: uploadState.activeUploads.values
            .map((task) => _buildUploadCard(context, ref, task, isDark))
            .toList(),
      ),
    );
  }

  Widget _buildUploadCard(
    BuildContext context,
    WidgetRef ref,
    UploadTaskProgress task,
    bool isDark,
  ) {
    return Container(
      padding: const EdgeInsets.all(12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Header dengan icon dan tipe
          Row(
            children: [
              UploadTaskUtils.buildTaskIcon(task.type, task.status),
              const SizedBox(width: 8),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      UploadTaskUtils.getTaskTitle(task.type),
                      style: const TextStyle(
                        fontWeight: FontWeight.w600,
                        fontSize: 14,
                      ),
                    ),
                    const SizedBox(height: 2),
                    Text(
                      task.description,
                      style: TextStyle(
                        fontSize: 12,
                        color: isDark
                            ? AppColors.neutralGray400
                            : AppColors.neutralGray600,
                      ),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                  ],
                ),
              ),
              if (task.status == UploadTaskStatus.completed)
                Icon(
                  Icons.check_circle,
                  color: AppColors.statusSuccess,
                  size: 20,
                )
              else if (task.status == UploadTaskStatus.failed)
                GestureDetector(
                  onTap: () => ref
                      .read(uploadProgressProvider.notifier)
                      .removeUpload(task.taskId),
                  child: const Icon(
                    Icons.close,
                    color: AppColors.statusError,
                    size: 20,
                  ),
                ),
            ],
          ),

          const SizedBox(height: 8),

          // Progress bar
          if (task.status != UploadTaskStatus.completed) ...[
            Row(
              children: [
                Expanded(
                  child: LinearProgressIndicator(
                    value: task.progress,
                    backgroundColor: isDark
                        ? AppColors.neutralGray600
                        : AppColors.neutralGray200,
                    valueColor: AlwaysStoppedAnimation<Color>(
                      task.status == UploadTaskStatus.failed
                          ? AppColors.statusError
                          : AppColors.primaryBlue,
                    ),
                  ),
                ),
                const SizedBox(width: 8),
                Text(
                  '${(task.progress * 100).toInt()}%',
                  style: TextStyle(
                    fontSize: 12,
                    fontWeight: FontWeight.w500,
                    color: isDark
                        ? AppColors.neutralGray300
                        : AppColors.neutralGray700,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 4),

            // Step indicator
            if (task.totalSteps > 0)
              Text(
                'Langkah ${task.currentStep} dari ${task.totalSteps}',
                style: TextStyle(
                  fontSize: 11,
                  color: isDark
                      ? AppColors.neutralGray400
                      : AppColors.neutralGray500,
                ),
              ),
          ],

          // Error message
          if (task.status == UploadTaskStatus.failed &&
              task.errorMessage != null) ...[
            const SizedBox(height: 4),
            Text(
              task.errorMessage!,
              style: const TextStyle(
                fontSize: 11,
                color: AppColors.statusError,
              ),
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
            ),
          ],
        ],
      ),
    );
  }
}
