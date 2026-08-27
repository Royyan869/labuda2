part of '../screens/checkout_screen_impl.dart';

/// Notes Section
class _NotesSection extends StatelessWidget {
  final TextEditingController notesController;
  final VoidCallback? onNotesChanged;

  const _NotesSection({required this.notesController, this.onNotesChanged});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: AppColors.neutralWhite,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: AppColors.neutralGray200),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text(
            'Catatan (Opsional)',
            style: TextStyle(fontSize: 16, fontWeight: FontWeight.w600),
          ),
          const SizedBox(height: 12),
          AppTextField(
            controller: notesController,
            hintText: 'Tambahkan catatan untuk seller...',
            maxLines: 3,
            onChanged: onNotesChanged != null ? (_) => onNotesChanged!() : null,
          ),
        ],
      ),
    );
  }
}
