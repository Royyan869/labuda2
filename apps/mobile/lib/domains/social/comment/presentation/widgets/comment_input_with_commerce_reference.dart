/// Comment Input with Commerce Reference capability.
///
/// Supports For Sale and Auction commerce references.
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/domain.dart';
import 'package:labuda/domains/social/comment/presentation/widgets/commerce_resource_picker.dart';
import 'package:labuda/domains/social/comment/presentation/widgets/resource_identity.dart';
export 'resource_identity.dart';

/// Canonical comment input with commerce reference capability.
///
/// onSubmit callback receives [ResourceIdentity] for commerce references
/// (For Sale or Auction). Uses GoRouter for Create navigation.
class CommentInputWithCommerceReference extends ConsumerStatefulWidget {
  final Future<bool> Function(String body, ResourceIdentity? resource) onSubmit;
  final ResourceIdentity? initialResource;
  final String hintText;
  final bool isSeller;
  final String sellerId;

  const CommentInputWithCommerceReference({
    super.key,
    required this.onSubmit,
    this.initialResource,
    this.hintText = 'Tulis komentar...',
    this.isSeller = false,
    this.sellerId = '',
  });

  @override
  ConsumerState<CommentInputWithCommerceReference> createState() =>
      _CommentInputWithCommerceReferenceState();
}

class _CommentInputWithCommerceReferenceState
    extends ConsumerState<CommentInputWithCommerceReference> {
  late TextEditingController _controller;
  ResourceIdentity? _selectedResource;
  CommerceResourceSelection? _selection;
  bool _isSubmitting = false;

  void _handleComposerChanged() {
    if (mounted) setState(() {});
  }

  @override
  void initState() {
    super.initState();
    _controller = TextEditingController();
    _selectedResource = widget.initialResource;
    _controller.addListener(_handleComposerChanged);
  }

  @override
  void dispose() {
    _controller.removeListener(_handleComposerChanged);
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
        border: Border(
          top: BorderSide(
            color: isDark ? AppColors.darkGray700 : AppColors.neutralGray200,
          ),
        ),
      ),
      child: SafeArea(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // Selected commerce resource preview
            if (_selection != null) ...[
              _SelectedResourceCard(
                selection: _selection!,
                onRemove: () => setState(() {
                  _selectedResource = null;
                  _selection = null;
                }),
              ),
              const SizedBox(height: 12),
            ],
            // Input row
            Row(
              crossAxisAlignment: CrossAxisAlignment.end,
              children: [
                Expanded(
                  child: TextField(
                    controller: _controller,
                    maxLines: null,
                    minLines: 1,
                    maxLength: 500,
                    decoration: InputDecoration(
                      hintText: widget.hintText,
                      hintStyle: TextStyle(color: AppColors.neutralGray600),
                      filled: true,
                      fillColor: isDark
                          ? AppColors.darkGray700
                          : AppColors.neutralGray100,
                      border: OutlineInputBorder(
                        borderRadius: BorderRadius.circular(24),
                        borderSide: BorderSide.none,
                      ),
                      contentPadding: const EdgeInsets.symmetric(
                        horizontal: 16,
                        vertical: 12,
                      ),
                      counterText: '',
                    ),
                  ),
                ),
                const SizedBox(width: 8),
                // Attach commerce resource button (seller only)
                if (widget.isSeller)
                  IconButton(
                    onPressed: _showCommercePicker,
                    icon: Icon(
                      Icons.add_circle_outline,
                      color: _selectedResource != null
                          ? AppColors.primaryRed
                          : AppColors.neutralGray600,
                      size: 28,
                    ),
                    tooltip: 'Lampirkan Produk',
                  ),
                // Send button
                Container(
                  decoration: BoxDecoration(
                    color: _canSubmit() && !_isSubmitting
                        ? AppColors.primaryRed
                        : AppColors.neutralGray400,
                    shape: BoxShape.circle,
                  ),
                  child: IconButton(
                    key: const ValueKey('comment-send-button'),
                    onPressed: (_canSubmit() && !_isSubmitting)
                        ? _handleSubmit
                        : null,
                    icon: _isSubmitting
                        ? const SizedBox(
                            width: 20,
                            height: 20,
                            child: CircularProgressIndicator(
                              strokeWidth: 2,
                              valueColor: AlwaysStoppedAnimation<Color>(
                                AppColors.neutralWhite,
                              ),
                            ),
                          )
                        : const Icon(Icons.send, color: AppColors.neutralWhite),
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  bool _canSubmit() =>
      _controller.text.trim().isNotEmpty || _selectedResource != null;

  Future<void> _handleSubmit() async {
    if (!_canSubmit() || _isSubmitting) return;
    final body = _controller.text.trim();
    final resource = _selectedResource;
    setState(() => _isSubmitting = true);
    try {
      final success = await widget.onSubmit(body, resource);
      if (success && mounted) {
        _controller.clear();
        setState(() {
          _selectedResource = null;
          _selection = null;
        });
      }
    } finally {
      if (mounted) setState(() => _isSubmitting = false);
    }
  }

  void _showCommercePicker() async {
    final result = await CommerceResourcePicker.show(
      context,
      sellerId: widget.sellerId,
      selectedResourceId: _selectedResource?.resourceId,
      onCreateNewListing: () async {
        Navigator.of(context).pop(); // close picker
        // Use GoRouter for canonical navigation
        final result = await context.pushNamed(RoutePaths.createForSale);
        if (!mounted) return;
        if (result is ForSale) {
          // Create For Sale route returned a ForSale — set as selected resource
          setState(() {
            _selectedResource = ResourceIdentity(
              resourceType: ResourceType.forSale,
              resourceId: result.forSaleId,
            );
            _selection = CommerceResourceSelection(
              resource: _selectedResource!,
              title: result.title,
              price: result.price.toInt(),
              imageUrl: result.media.isNotEmpty
                  ? result.media.first.originalUrl
                  : null,
            );
          });
        }
      },
    );
    if (result != null && mounted) {
      setState(() {
        _selectedResource = result.resource;
        _selection = result;
      });
    }
  }
}

/// Selected commerce resource preview.
class _SelectedResourceCard extends StatelessWidget {
  final CommerceResourceSelection selection;
  final VoidCallback onRemove;

  const _SelectedResourceCard({
    required this.selection,
    required this.onRemove,
  });

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    return Container(
      padding: const EdgeInsets.all(10),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray700 : AppColors.neutralGray50,
        borderRadius: BorderRadius.circular(10),
        border: Border.all(
          color: AppColors.primaryRed.withValues(alpha: 0.3),
          width: 1,
        ),
      ),
      child: Row(
        children: [
          ClipRRect(
            borderRadius: BorderRadius.circular(6),
            child: selection.imageUrl != null
                ? Image.network(
                    selection.imageUrl!,
                    width: 45,
                    height: 45,
                    fit: BoxFit.cover,
                    errorBuilder: (_, _, _) => _placeholder(isDark),
                  )
                : _placeholder(isDark),
          ),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  selection.title,
                  style: TextStyle(
                    fontWeight: FontWeight.w600,
                    fontSize: 13,
                    color: isDark
                        ? AppColors.neutralWhite
                        : AppColors.neutralGray900,
                  ),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
                const SizedBox(height: 2),
                if (selection.price != null)
                  Text(
                    'Rp ${selection.price}',
                    style: const TextStyle(
                      color: AppColors.primaryRed,
                      fontWeight: FontWeight.bold,
                      fontSize: 13,
                    ),
                  ),
              ],
            ),
          ),
          IconButton(
            onPressed: onRemove,
            icon: const Icon(Icons.close, size: 18),
            constraints: const BoxConstraints(),
            padding: EdgeInsets.zero,
          ),
        ],
      ),
    );
  }

  Widget _placeholder(bool isDark) => Container(
    width: 45,
    height: 45,
    decoration: BoxDecoration(
      color: AppColors.neutralGray200,
      borderRadius: BorderRadius.circular(6),
    ),
    child: Icon(
      Icons.image_not_supported,
      size: 16,
      color: AppColors.neutralGray400,
    ),
  );
}
