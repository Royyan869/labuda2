import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';

/// Custom AppBar for create post screen
class ContentAppBar extends StatelessWidget implements PreferredSizeWidget {
  final bool isSubmitting;
  final bool canSubmit;
  final VoidCallback onClose;
  final VoidCallback? onSubmit;

  const ContentAppBar({
    super.key,
    required this.isSubmitting,
    required this.canSubmit,
    required this.onClose,
    this.onSubmit,
  });

  @override
  Size get preferredSize => const Size.fromHeight(kToolbarHeight);

  @override
  Widget build(BuildContext context) {
    return AppBar(
      title: const Text('Create Content'),
      surfaceTintColor: Colors.transparent,
      scrolledUnderElevation: 0,
      leading: IconButton(onPressed: onClose, icon: const Icon(Icons.close)),
      actions: [
        TextButton(
          onPressed: canSubmit && !isSubmitting ? onSubmit : null,
          child: isSubmitting
              ? const SizedBox(
                  width: 16,
                  height: 16,
                  child: CircularProgressIndicator(
                    strokeWidth: 2,
                    valueColor: AlwaysStoppedAnimation(AppColors.primaryRed),
                  ),
                )
              : Text(
                  'Submit',
                  style: TextStyle(
                    fontWeight: FontWeight.w600,
                    color: canSubmit
                        ? AppColors.primaryRed
                        : AppColors.neutralGray400,
                  ),
                ),
        ),
      ],
    );
  }
}
