library;

/// Pre-Chat Form Sheet Widget (Refactored)
/// UI-only widget for collecting support ticket information
/// Presentation layer - delegates business logic to providers

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/shared/widgets/app_snackbar.dart';
import 'package:labuda/domains/system/support/domain/domain.dart';
import 'package:labuda/domains/system/support/presentation/providers/support_providers.dart';
import 'package:labuda/domains/system/support/presentation/screens/support_ticket_thread_screen.dart';

// ============================================
// WIDGET
// ============================================

/// Pre-Chat Form Sheet
/// Shown before user starts support chat
/// Collects: category, description, priority, linked order
class PreChatFormSheetRefactored extends ConsumerStatefulWidget {
  final String userId;
  final String userName;
  final String? userAvatar;
  final String? linkedOrderId;
  final VoidCallback? onChatCreated;

  const PreChatFormSheetRefactored({
    super.key,
    required this.userId,
    required this.userName,
    this.userAvatar,
    this.linkedOrderId,
    this.onChatCreated,
  });

  @override
  ConsumerState<PreChatFormSheetRefactored> createState() =>
      _PreChatFormSheetRefactoredState();
}

class _PreChatFormSheetRefactoredState
    extends ConsumerState<PreChatFormSheetRefactored> {
  final _formKey = GlobalKey<FormState>();
  final _descriptionController = TextEditingController();

  SupportCategory? _selectedCategory;
  SupportPriority _selectedPriority = SupportPriority.medium;
  String? _linkedOrderId;
  bool _isLoading = false;

  @override
  void initState() {
    super.initState();
    _linkedOrderId = widget.linkedOrderId;
  }

  /// PHASE 3 HARDENING: Auto-priority mapping based on category
  /// Maps categories to appropriate priority levels
  SupportPriority _getPriorityForCategory(SupportCategory category) {
    return switch (category) {
      // Money-at-risk categories are high priority.
      SupportCategory.paymentIssue => SupportPriority.high,
      SupportCategory.refundRequest => SupportPriority.high,
      SupportCategory.dispute => SupportPriority.high,
      // Order and shipping issues affect fulfilment.
      SupportCategory.orderIssue => SupportPriority.medium,
      SupportCategory.shippingIssue => SupportPriority.medium,
      SupportCategory.accountIssue => SupportPriority.medium,
      // Listing, technical and general inquiries are lower priority.
      SupportCategory.listingIssue => SupportPriority.low,
      SupportCategory.technicalIssue => SupportPriority.low,
      SupportCategory.other => SupportPriority.low,
    };
  }

  /// Update priority when category changes
  void _onCategorySelected(SupportCategory? category) {
    setState(() {
      _selectedCategory = category;
      if (category != null) {
        _selectedPriority = _getPriorityForCategory(category);
      }
    });
  }

  @override
  void dispose() {
    _descriptionController.dispose();
    super.dispose();
  }

  Future<void> _submitForm() async {
    if (_selectedCategory == null) return;

    setState(() => _isLoading = true);

    try {
      final repository = ref.read(supportRepositoryProvider);

      final result = await repository.createTicket(
        userId: widget.userId,
        userName: widget.userName,
        userAvatar: widget.userAvatar,
        category: _selectedCategory!,
        priority: _selectedPriority,
        description: _descriptionController.text.trim().isEmpty
            ? null
            : _descriptionController.text.trim(),
        linkedOrderId: _linkedOrderId,
      );

      if (!mounted) return;

      if (result.isSuccess) {
        final ticketId = result.dataOrThrow;

        // Close bottom sheet
        Navigator.pop(context);

        // Callback
        widget.onChatCreated?.call();

        // Navigate to ticket thread screen (email-like, not chat)
        Navigator.of(context).push(
          MaterialPageRoute(
            builder: (context) => SupportTicketThreadScreen(ticketId: ticketId),
          ),
        );

        // Show success message
        AppSnackBar.showSuccess(
          context,
          'Support ticket created. We\'ll respond shortly!',
          duration: const Duration(seconds: 3),
        );
      } else {
        setState(() => _isLoading = false);

        AppSnackBar.showError(
          context,
          result.failure?.message ?? 'Failed to create support chat',
          duration: const Duration(seconds: 4),
        );
      }
    } catch (e) {
      if (!mounted) return;

      setState(() => _isLoading = false);

      AppSnackBar.showError(
        context,
        'Gagal membuka chat. Coba lagi.',
        duration: const Duration(seconds: 4),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Container(
      decoration: BoxDecoration(
        color: theme.scaffoldBackgroundColor,
        borderRadius: const BorderRadius.vertical(top: Radius.circular(20)),
      ),
      child: SafeArea(
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(24),
          child: Form(
            key: _formKey,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              mainAxisSize: MainAxisSize.min,
              children: [
                // Handle bar
                Center(
                  child: Container(
                    width: 40,
                    height: 4,
                    margin: const EdgeInsets.only(bottom: 20),
                    decoration: BoxDecoration(
                      color: Theme.of(context).colorScheme.outlineVariant,
                      borderRadius: BorderRadius.circular(2),
                    ),
                  ),
                ),

                // Header
                Text(
                  'Create Support Ticket',
                  style: theme.textTheme.headlineSmall?.copyWith(
                    fontWeight: FontWeight.bold,
                  ),
                ),
                const SizedBox(height: 8),
                Text(
                  'Describe your issue and we\'ll get back to you via email.',
                  style: theme.textTheme.bodyMedium?.copyWith(
                    color: Theme.of(context).colorScheme.onSurfaceVariant,
                  ),
                ),
                const SizedBox(height: 24),

                // Category Selection
                Text(
                  'Category *',
                  style: theme.textTheme.titleSmall?.copyWith(
                    fontWeight: FontWeight.bold,
                  ),
                ),
                const SizedBox(height: 12),
                _buildCategorySelection(),
                const SizedBox(height: 24),

                // Description (optional)
                Text(
                  'Description (optional)',
                  style: theme.textTheme.titleSmall?.copyWith(
                    fontWeight: FontWeight.bold,
                  ),
                ),
                const SizedBox(height: 8),
                TextField(
                  controller: _descriptionController,
                  maxLines: 3,
                  maxLength: 500,
                  decoration: InputDecoration(
                    hintText: 'Briefly describe your issue...',
                    border: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(12),
                    ),
                    filled: true,
                    fillColor: Theme.of(context).colorScheme.surfaceContainer,
                  ),
                ),
                const SizedBox(height: 24),

                // Linked Order (if any)
                if (_linkedOrderId != null) ...[
                  Container(
                    padding: const EdgeInsets.all(12),
                    decoration: BoxDecoration(
                      color: Theme.of(context).colorScheme.secondary.withValues(alpha: 0.08),
                      borderRadius: BorderRadius.circular(12),
                      border: Border.all(color: Theme.of(context).colorScheme.secondary.withValues(alpha: 0.35)),
                    ),
                    child: Row(
                      children: [
                        Icon(Icons.link, color: Theme.of(context).colorScheme.secondary),
                        const SizedBox(width: 8),
                        Expanded(
                          child: Text(
                            'Linked to Order #${_linkedOrderId!.substring(0, 8)}...',
                            style: TextStyle(
                              color: Theme.of(context).colorScheme.secondary,
                              fontWeight: FontWeight.w500,
                            ),
                          ),
                        ),
                        IconButton(
                          icon: const Icon(Icons.close, size: 20),
                          onPressed: () {
                            setState(() => _linkedOrderId = null);
                          },
                          color: Theme.of(context).colorScheme.secondary,
                        ),
                      ],
                    ),
                  ),
                  const SizedBox(height: 24),
                ],

                // Submit Button
                ElevatedButton(
                  onPressed: (_isLoading || _selectedCategory == null)
                      ? null
                      : _submitForm,
                  style: ElevatedButton.styleFrom(
                    padding: const EdgeInsets.symmetric(vertical: 16),
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(12),
                    ),
                    backgroundColor: Theme.of(context).colorScheme.primary,
                    foregroundColor: Theme.of(context).colorScheme.onPrimary,
                    disabledBackgroundColor: Theme.of(context).colorScheme.onSurfaceVariant,
                    disabledForegroundColor: Theme.of(context).colorScheme.onSurfaceVariant,
                  ),
                  child: _isLoading
                      ? SizedBox(
                          height: 20,
                          width: 20,
                          child: CircularProgressIndicator(
                            strokeWidth: 2,
                            valueColor: AlwaysStoppedAnimation<Color>(
                              Theme.of(context).colorScheme.onPrimary,
                            ),
                          ),
                        )
                      : const Text(
                          'Create Ticket',
                          style: TextStyle(
                            fontSize: 16,
                            fontWeight: FontWeight.bold,
                          ),
                        ),
                ),
                const SizedBox(height: 12),

                // Cancel Button
                TextButton(
                  onPressed: _isLoading ? null : () => Navigator.pop(context),
                  child: const Text('Cancel'),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildCategorySelection() {
    return Wrap(
      spacing: 8,
      runSpacing: 8,
      children: SupportCategory.values.map((category) {
        final config = CategoryConfig.get(category);
        final isSelected = _selectedCategory == category;

        return ChoiceChip(
          label: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(config.icon),
              const SizedBox(width: 6),
              Text(config.nameId),
            ],
          ),
          selected: isSelected,
          onSelected: (selected) {
            // PHASE 3 HARDENING: Use priority mapping when category changes
            _onCategorySelected(selected ? category : null);
          },
          selectedColor: Color(config.colorValue).withValues(alpha: 0.2),
          backgroundColor: Theme.of(context).colorScheme.surfaceContainer,
          labelStyle: TextStyle(
            color: isSelected
                ? Color(config.colorValue)
                : Theme.of(context).textTheme.bodyMedium?.color,
            fontWeight: isSelected ? FontWeight.bold : FontWeight.normal,
          ),
          side: BorderSide(
            color: isSelected ? Color(config.colorValue) : Colors.transparent,
            width: 2,
          ),
        );
      }).toList(),
    );
  }
}

// ============================================
// HELPER FUNCTIONS
// ============================================

/// Helper to show the pre-chat form sheet
void showPreChatFormRefactored(
  BuildContext context, {
  required String userId,
  required String userName,
  String? userAvatar,
  String? linkedOrderId,
  VoidCallback? onChatCreated,
}) {
  showModalBottomSheet(
    context: context,
    isScrollControlled: true,
    backgroundColor: Colors.transparent,
    builder: (context) => PreChatFormSheetRefactored(
      userId: userId,
      userName: userName,
      userAvatar: userAvatar,
      linkedOrderId: linkedOrderId,
      onChatCreated: onChatCreated,
    ),
  );
}
