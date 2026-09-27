// Dart
// Flutter
import 'package:flutter/material.dart';

// External
import 'package:flutter_riverpod/flutter_riverpod.dart';

// Internal
import 'package:labuda/shared/src/providers/upload_progress_provider.dart';
import 'package:labuda/shared/src/widgets/upload_task_utils.dart';

/// Widget untuk menampilkan upload progress di home screen
class UploadProgressWidget extends ConsumerWidget {
  const UploadProgressWidget({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final uploadState = ref.watch(uploadProgressProvider);

    if (uploadState.activeUploads.isEmpty) {
      return const SizedBox.shrink();
    }

    final scheme = Theme.of(context).colorScheme;
    return Container(
      margin: const EdgeInsets.fromLTRB(12, 8, 12, 0),
      decoration: BoxDecoration(
        color: scheme.surfaceContainerLow,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: scheme.outlineVariant,
          width: 1,
        ),
      ),
      child: Column(
        children: uploadState.activeUploads.values
            .map((task) => _buildUploadCard(context, ref, task))
            .toList(),
      ),
    );
  }

  Widget _buildUploadCard(
    BuildContext context,
    WidgetRef ref,
    UploadTaskProgress task,
  ) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.all(12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Header dengan icon dan tipe
          Row(
            children: [
              UploadTaskUtils.buildTaskIcon(context, task.type, task.status),
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
                        color: scheme.onSurfaceVariant,
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
                  color: scheme.primary,
                  size: 20,
                )
              else if (task.status == UploadTaskStatus.failed)
                GestureDetector(
                  onTap: () => ref
                      .read(uploadProgressProvider.notifier)
                      .removeUpload(task.taskId),
                  child: Icon(
                    Icons.close,
                    color: scheme.error,
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
                    backgroundColor: scheme.surfaceContainerHighest,
                    valueColor: AlwaysStoppedAnimation<Color>(
                      task.status == UploadTaskStatus.failed
                          ? scheme.error
                          : scheme.secondary,
                    ),
                  ),
                ),
                const SizedBox(width: 8),
                Text(
                  '${(task.progress * 100).toInt()}%',
                  style: TextStyle(
                    fontSize: 12,
                    fontWeight: FontWeight.w500,
                    color: scheme.onSurface,
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
                  color: scheme.onSurfaceVariant,
                ),
              ),
          ],

          // Error message
          if (task.status == UploadTaskStatus.failed &&
              task.errorMessage != null) ...[
            const SizedBox(height: 4),
            Text(
              task.errorMessage!,
              style: TextStyle(
                fontSize: 11,
                color: scheme.error,
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
