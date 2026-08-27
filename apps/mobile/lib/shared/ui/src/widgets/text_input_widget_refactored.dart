import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';

// Import all extracted components
import 'text_input_reply_preview.dart';
import 'text_input_media_preview.dart';
import 'text_input_area.dart';

// Import custom pickers

/// Refactored Text Input Widget using extracted components
///
/// Under 200 lines, uses all 4 extracted components:
/// 1. TextInputReplyPreview - for reply preview
/// 2. TextInputMediaPreview - for media preview
/// 3. TextInputArea - for main input area
/// 4. TextInputActions - for action buttons
class TextInputWidgetRefactored extends ConsumerStatefulWidget {
  final TextEditingController messageController;
  final FocusNode focusNode;
  final Function(String content, List<String> mediaUrls) onSendMessage;
  final Function(List<String> mediaUrls)? onMediaSelected;
  final List<String> selectedMediaUrls;
  final Function(int index)? onRemoveMedia;
  final VoidCallback? onMediaCleared;
  final ReplyData? replyingTo;
  final VoidCallback? onCancelReply;
  final TextInputConfig config;
  final List<QuickAction>? customQuickActions;

  const TextInputWidgetRefactored({
    super.key,
    required this.messageController,
    required this.focusNode,
    required this.onSendMessage,
    this.onMediaSelected,
    this.selectedMediaUrls = const [],
    this.onRemoveMedia,
    this.onMediaCleared,
    this.replyingTo,
    this.onCancelReply,
    this.config = const TextInputConfig(),
    this.customQuickActions,
  });

  @override
  ConsumerState<TextInputWidgetRefactored> createState() =>
      _TextInputWidgetRefactoredState();
}

class _TextInputWidgetRefactoredState
    extends ConsumerState<TextInputWidgetRefactored> {
  bool _showQuickActions = false;
  bool _hasTextContent = false;

  @override
  void initState() {
    super.initState();
    widget.messageController.addListener(_onTextChanged);
  }

  @override
  void dispose() {
    widget.messageController.removeListener(_onTextChanged);
    super.dispose();
  }

  void _onTextChanged() {
    setState(() {
      _hasTextContent = widget.messageController.text.trim().isNotEmpty;
    });
  }

  void _toggleQuickActions() {
    setState(() {
      _showQuickActions = !_showQuickActions;
    });
  }

  void _sendMessage() {
    final content = widget.messageController.text.trim();
    if (content.isNotEmpty || widget.selectedMediaUrls.isNotEmpty) {
      widget.onSendMessage(content, widget.selectedMediaUrls);
      widget.messageController.clear();
      setState(() {
        _hasTextContent = false;
        _showQuickActions = false;
      });
    }
  }

  List<QuickAction> _getDefaultQuickActions() {
    return [
      QuickAction(
        icon: Icons.photo_library,
        label: 'Gallery',
        color: AppColors.primaryGreen,
        onTap: _openGallery,
      ),
      QuickAction(
        icon: Icons.camera_alt,
        label: 'Camera',
        color: AppColors.primaryBlue,
        onTap: _openCamera,
      ),
      QuickAction(
        icon: Icons.location_on,
        label: 'Location',
        color: AppColors.primaryYellow,
        onTap: _shareLocation,
      ),
      QuickAction(
        icon: Icons.link,
        label: 'Link',
        color: AppColors.primaryBlue,
        onTap: _showLinkPicker,
      ),
    ];
  }

  Future<void> _openGallery() async {
    try {
      final mediaUrls = await MediaPickerHelper.pickMedia(
        context: context,
        maxAssets: 10,
      );

      if (mediaUrls != null && mediaUrls.isNotEmpty) {
        widget.onMediaSelected?.call(mediaUrls);
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: const Text('Gagal memilih media. Coba lagi.'),
            duration: const Duration(seconds: 4),
          ),
        );
      }
    }
  }

  Future<void> _openCamera() async {
    try {
      final mediaUrls = await CustomCameraScreen.show(context);

      if (mediaUrls != null && mediaUrls.isNotEmpty) {
        widget.onMediaSelected?.call(mediaUrls);
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: const Text('Gagal membuka kamera. Coba lagi.'),
            duration: const Duration(seconds: 4),
          ),
        );
      }
    }
  }

  void _shareLocation() {
    // NOTE: Location sharing is not available
    ScaffoldMessenger.of(context).showSnackBar(
      const SnackBar(
        content: Text('Location sharing not available'),
        backgroundColor: AppColors.neutralGray600,
        duration: Duration(seconds: 3),
      ),
    );
  }

  void _showLinkPicker() {
    LinkPickerModal.show(
      context: context,
      onLinkSelected: (shareRef) {
        // TODO: Handle link attachment - save to comment or show in preview
        if (mounted) {
          AppSnackBar.showSuccess(
            context,
            'Link selected: ${shareRef.targetType.wireValue} (${shareRef.targetId})',
          );
        }
      },
    );
  }

  void _openFullScreenViewer(int index) {
    // TODO: Implement full screen viewer
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text('Opening media ${index + 1}'),
        duration: const Duration(seconds: 3),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final actions = widget.customQuickActions ?? _getDefaultQuickActions();

    return Container(
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
        border: Border(
          top: BorderSide(
            color: isDark ? AppColors.darkGray700 : AppColors.neutralGray200,
            width: 1,
          ),
        ),
      ),
      child: Column(
        children: [
          // Reply Preview - Component usage
          if (widget.replyingTo != null)
            TextInputReplyPreview(
              replyingTo: widget.replyingTo!,
              onCancelReply: widget.onCancelReply,
              isDark: isDark,
            ),

          // Media Preview - Component usage
          if (widget.selectedMediaUrls.isNotEmpty)
            TextInputMediaPreview(
              selectedMediaUrls: widget.selectedMediaUrls,
              onRemoveMedia: widget.onRemoveMedia,
              onMediaTap: _openFullScreenViewer,
              isDark: isDark,
            ),

          // Extended Actions - Component usage (MOVED TO TOP)
          if (_showQuickActions && widget.config.enableQuickActions)
            TextInputActions(actions: actions, isDark: isDark),

          // Main Input Area - Component usage (MOVED TO BOTTOM)
          TextInputArea(
            messageController: widget.messageController,
            focusNode: widget.focusNode,
            hasTextContent: _hasTextContent,
            selectedMediaUrls: widget.selectedMediaUrls,
            config: widget.config,
            showQuickActions: _showQuickActions,
            onToggleQuickActions: widget.config.enableQuickActions
                ? _toggleQuickActions
                : null,
            onSendMessage: _sendMessage,
            isDark: isDark,
          ),
        ],
      ),
    );
  }
}
