part of '../screens/checkout_screen_impl.dart';

/// Notes Section
///
/// Notes travel to POST /orders (an order-creation input). They are NOT part of
/// the pricing contract, so editing them must not invalidate the applied
/// backend pricing preview.
class _NotesSection extends StatelessWidget {
  final TextEditingController notesController;

  const _NotesSection({required this.notesController});

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: colorScheme.surface,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: colorScheme.outlineVariant),
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
          ),
        ],
      ),
    );
  }
}
