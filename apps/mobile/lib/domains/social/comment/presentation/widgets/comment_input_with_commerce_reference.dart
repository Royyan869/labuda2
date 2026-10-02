/// Comment Input with Commerce Reference capability.
///
/// Supports For Sale and Auction commerce references.
library;

import 'package:flutter/material.dart';
import 'package:labuda/shared/widgets/pending_commerce_chip.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/widgets/app_snackbar.dart';
import 'package:labuda/shared/widgets/composer_action_buttons.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/domain.dart';
import 'package:labuda/domains/social/comment/presentation/widgets/commerce_resource_picker.dart';
import 'package:labuda/domains/social/comment/presentation/widgets/resource_identity.dart';
import 'package:labuda/core/media/media_upload_config.dart';
import 'package:labuda/core/media/media_upload_orchestrator.dart';
import 'package:labuda/shared/widgets/pending_media_strip.dart';
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
  /// Local, not-yet-uploaded media. Uploaded at Send (canonical deferred
  /// upload), so a cancelled comment leaves nothing behind in S3.
  final List<MediaPendingItem> _pending = [];
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
            // Selected commerce resource preview — canonical pre-send chip.
            if (_selection != null) ...[
              PendingCommerceChip(
                title: _selection!.title,
                imageUrl: _selection!.imageUrl,
                price: _selection!.price,
                onRemove: () => setState(() {
                  _selectedResource = null;
                  _selection = null;
                }),
              ),
              const SizedBox(height: 12),
            ],
            // Media strip — foto+video (1 mesin, orchestrator)
            if (_pending.isNotEmpty) ...[
              PendingMediaStrip(
                items: _pending,
                onRemove: (i) => setState(() => _pending.removeAt(i)),
                onRetry: _retryUploads,
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
                ComposerAddButton(onPressed: _showAttachSheet),
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
      _controller.text.trim().isNotEmpty ||
      _selectedResource != null ||
      _pending.isNotEmpty;

  Future<void> _handleSubmit() async {
    if (!_canSubmit() || _isSubmitting) return;
    final body = _controller.text.trim();
    final resource = _selectedResource;
    setState(() => _isSubmitting = true);
    try {
      // Upload-at-Send (canonical): picking never uploads, so an abandoned
      // comment costs nothing. A failure stops BEFORE submit and keeps the
      // composer intact — the next Send resumes from the file that failed
      // instead of re-uploading the ones that already succeeded.
      final uploaded = await MediaUploadOrchestrator.uploadPending(
        context: context,
        config: MediaUploadConfig.forComment,
        items: _pending,
        onChanged: () {
          if (mounted) setState(() {});
        },
      );
      // Nothing was sent: the strip still shows exactly which file failed, and
      // retrying re-runs ONLY that file.
      if (!uploaded) return;
      final mediaUrls = _pending.map((e) => e.url!).toList();
      final success = widget.onSubmitWithMedia != null
          ? await widget.onSubmitWithMedia!(body, resource, mediaUrls)
          : await widget.onSubmit(body, resource);
      if (success && mounted) {
        _controller.clear();
        setState(() {
          _selectedResource = null;
          _selection = null;
          _pending.clear();
        });
      }
    } finally {
      if (mounted) setState(() => _isSubmitting = false);
    }
  }  /// Re-runs the uploads the strip marks as failed. Only those files go again —
  /// anything already holding a URL is skipped by the loop.
  Future<void> _retryUploads() async {
    if (_pending.isEmpty) return;
    await MediaUploadOrchestrator.uploadPending(
      context: context,
      config: MediaUploadConfig.forComment,
      items: _pending,
      onChanged: () {
        if (mounted) setState(() {});
      },
    );
  }

  /// The canonical attachment sheet — no local copy. Entries: Galeri (foto &
  /// video, ONE system picker), Kamera, plus `Lampirkan Produk`, which is
  /// a SELLER-only capability (a non-seller must never see it). Picking only
  /// parks local files in the composer; upload happens at Send.
  void _showAttachSheet() {
    final config = MediaUploadConfig.forComment;
    final current = MediaUploadOrchestrator.countsOfFiles(
      _pending.map((e) => e.file),
    );
    if (config.remainingTotalFor(current) <= 0) {
      AppSnackBar.showError(
        context,
        'Maksimal ${config.maxTotal} foto/video',
      );
      return;
    }
    MediaUploadOrchestrator.showAttachSheet(
      context: context,
      config: config,
      current: current,
      extraActions: widget.isSeller
          ? [
              MediaSheetAction(
                icon: Icons.storefront,
                label: 'Lampirkan Produk',
                subtitle: 'For Sale atau Lelang',
                onTap: _showCommercePicker,
              ),
            ]
          : const [],
      onPicked: (files) async {
        if (!mounted) return;
        setState(() => _pending.addAll(files.map(MediaPendingItem.new)));
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
