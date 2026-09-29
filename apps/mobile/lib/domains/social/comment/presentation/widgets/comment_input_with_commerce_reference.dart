/// Comment Input with Commerce Reference capability.
///
/// Supports For Sale and Auction commerce references.
library;

import 'package:flutter/material.dart';
import 'package:labuda/shared/widgets/app_image.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/widgets/app_snackbar.dart';
import 'package:labuda/shared/widgets/composer_action_buttons.dart';
import 'package:labuda/shared/domain/entities/resource_projection.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/domain.dart';
import 'package:labuda/domains/social/comment/presentation/widgets/commerce_resource_picker.dart';
import 'package:labuda/domains/social/comment/presentation/widgets/resource_identity.dart';
import 'package:labuda/core/media/media_upload_config.dart';
import 'package:labuda/core/media/media_upload_orchestrator.dart';
import 'package:labuda/shared/widgets/media_grid_uploader.dart';
export 'resource_identity.dart';

/// Canonical comment input with commerce reference capability.
///
/// onSubmit callback receives [ResourceIdentity] for commerce references
/// (For Sale or Auction). Uses GoRouter for Create navigation.
class CommentInputWithCommerceReference extends ConsumerStatefulWidget {
  final Future<bool> Function(String body, ResourceIdentity? resource) onSubmit;
  /// Optional media-aware callback — if provided, foto+video URLs are forwarded.
  /// When null, media is still pickable/uploadable and previewed (FE support).
  final Future<bool> Function(String body, ResourceIdentity? resource, List<String> mediaUrls)? onSubmitWithMedia;
  final ResourceIdentity? initialResource;
  final String hintText;
  final bool isSeller;
  final String sellerId;

  const CommentInputWithCommerceReference({
    super.key,
    required this.onSubmit,
    this.onSubmitWithMedia,
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
  final List<String> _mediaUrls = [];
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
    final scheme = Theme.of(context).colorScheme;

    return Container(
      padding: const EdgeInsets.all(AppMetrics.p16),
      decoration: BoxDecoration(
        color: scheme.surface,
        border: Border(
          top: BorderSide(
            color: scheme.outlineVariant,
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
            // Media strip — foto+video (1 mesin, orchestrator)
            if (_mediaUrls.isNotEmpty) ...[
              CompactMediaStrip(
                mediaUrls: _mediaUrls,
                config: MediaUploadConfig.forComment,
                onMediaAdded: (url) => setState(() => _mediaUrls.add(url)),
                onMediaRemoved: (i) => setState(() => _mediaUrls.removeAt(i)),
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
                    decoration: AppTheme.composerDecoration(
                      scheme,
                      hintText: widget.hintText,
                    ),
                  ),
                ),
                const SizedBox(width: 8),
                // Canonical action row: ONE `+` entry for all attach flows
                // (foto/video + seller commerce menu). The two old right-side
                // icons cramped the pill — the sheet keeps both capabilities.
                ComposerAddButton(onPressed: _showAttachMenu),
                const SizedBox(width: 8),
                // Send always visible; disabled until the composer can submit.
                ComposerSendButton(
                  key: const ValueKey('comment-send-button'),
                  loading: _isSubmitting,
                  onPressed: (_canSubmit() && !_isSubmitting)
                      ? _handleSubmit
                      : null,
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  bool _canSubmit() =>
      _controller.text.trim().isNotEmpty || _selectedResource != null || _mediaUrls.isNotEmpty;

  Future<void> _handleSubmit() async {
    if (!_canSubmit() || _isSubmitting) return;
    final body = _controller.text.trim();
    final resource = _selectedResource;
    final mediaSnapshot = List<String>.from(_mediaUrls);
    setState(() => _isSubmitting = true);
    try {
      final success = widget.onSubmitWithMedia != null
          ? await widget.onSubmitWithMedia!(body, resource, mediaSnapshot)
          : await widget.onSubmit(body, resource);
      if (success && mounted) {
        _controller.clear();
        setState(() {
          _selectedResource = null;
          _selection = null;
          _mediaUrls.clear();
        });
      }
    } finally {
      if (mounted) setState(() => _isSubmitting = false);
    }
  }

  /// Single `+` entry for the comment composer — mirrors the chat
  /// attachment sheet (one concept, one presentation). The commerce entry
  /// only exists for sellers.
  void _showAttachMenu() {
    showModalBottomSheet(
      context: context,
      builder: (sheetContext) => SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            ListTile(
              leading: const Icon(Icons.photo_library),
              title: const Text('Foto'),
              subtitle: const Text('Kirim foto dari galeri'),
              onTap: () {
                Navigator.pop(sheetContext);
                _pickMedia();
              },
            ),
            ListTile(
              leading: const Icon(Icons.videocam),
              title: const Text('Video'),
              subtitle: const Text('Kirim video dari galeri'),
              onTap: () {
                Navigator.pop(sheetContext);
                _pickMedia();
              },
            ),
            if (widget.isSeller) ...[
              const Divider(),
              ListTile(
                leading: const Icon(Icons.storefront),
                title: const Text('Lampirkan Produk'),
                subtitle: const Text('For Sale atau Lelang'),
                onTap: () {
                  Navigator.pop(sheetContext);
                  _showCommercePicker();
                },
              ),
            ],
          ],
        ),
      ),
    );
  }

  void _pickMedia() {
    if (_mediaUrls.length >= MediaUploadConfig.forComment.maxTotal) {
      AppSnackBar.showError(
        context,
        'Maksimal ${MediaUploadConfig.forComment.maxTotal} foto/video',
      );
      return;
    }
    // canonical 1 mesin: foto+video via MediaUploadOrchestrator
    _showMediaPicker();
  }

  void _showMediaPicker() {
    MediaUploadOrchestrator.showPicker(
      context: context,
      config: MediaUploadConfig.forComment,
      currentCount: _mediaUrls.length,
      onUploaded: (urls) async {
        if (!mounted) return;
        setState(() => _mediaUrls.addAll(urls));
      },
    );
  }

  void _showCommercePicker() async {
    final result = await CommerceResourcePicker.show(
      context,
      sellerId: widget.sellerId,
      selectedResourceId: _selectedResource?.resourceId,
      onCreateNewForSale: () async {
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
    final scheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.all(AppMetrics.p10),
      decoration: BoxDecoration(
        color: scheme.surfaceContainerHigh,
        borderRadius: BorderRadius.circular(AppShape.r10),
        border: Border.all(
          color: scheme.primary.withValues(alpha: 0.3),
          width: 1,
        ),
      ),
      child: Row(
        children: [
          ClipRRect(
            borderRadius: BorderRadius.circular(AppShape.r6),
            child: selection.imageUrl != null
      ? AppImage(
          imageUrl: selection.imageUrl,
          width: 45,
          height: 45,
          fit: BoxFit.cover,
          errorWidget: _placeholder(context),
        )
                : _placeholder(context),
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
                    fontSize: AppType.s13,
                    color: scheme.onSurface,
                  ),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
                const SizedBox(height: 2),
                if (selection.price != null)
                  Text(
                    'Rp ${formatGroupedAmount(selection.price!)}',
                    style: TextStyle(
                      color: scheme.primary,
                      fontWeight: FontWeight.bold,
                      fontSize: AppType.s13,
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

  Widget _placeholder(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      width: 45,
      height: 45,
      decoration: BoxDecoration(
        color: scheme.surfaceContainerHighest,
        borderRadius: BorderRadius.circular(AppShape.r6),
      ),
      child: Icon(
        Icons.image_not_supported,
        size: 16,
        color: scheme.onSurfaceVariant,
      ),
    );
  }
}
