import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/shared/utils/mention_parser.dart';
import 'package:labuda/shared/widgets/mentions/mention_suggestion_overlay.dart';
import 'package:labuda/features/search/search/search.dart'; // **R2.2 MIGRATED**: Import from search domain

/// TextField dengan mention support (@username autocomplete)
///
/// **R2.2 MIGRATED**: Now uses mentionResolverProvider from search domain
/// instead of shared/providers.
///
/// Features:
/// - Auto-show suggestion saat ketik @
/// - Filter suggestions based on query
/// - Insert @username saat user select
/// - Track mentioned user IDs
/// - Support untuk group chat (filter by members)
///
/// Usage:
/// ```dart
/// MentionTextField(
///   controller: _controller,
///   hintText: 'Type a message...',
///   allowedUserIds: chat.participantIds, // For group chat
///   onMentionsChanged: (userIds) {
///     // Track mentioned users
///   },
/// )
/// ```
class MentionTextField extends ConsumerStatefulWidget {
  final TextEditingController controller;
  final FocusNode? focusNode;
  final String? hintText;
  final List<String>? allowedUserIds;
  final Function(List<String> mentionedUserIds)? onMentionsChanged;
  final InputDecoration? decoration;
  final int? maxLines;
  final int? minLines;
  final TextStyle? style;
  final bool showSpecialMentions;
  final VoidCallback? onChanged;

  const MentionTextField({
    super.key,
    required this.controller,
    this.focusNode,
    this.hintText,
    this.allowedUserIds,
    this.onMentionsChanged,
    this.decoration,
    this.maxLines = 1,
    this.minLines,
    this.style,
    this.showSpecialMentions = false,
    this.onChanged,
  });

  @override
  ConsumerState<MentionTextField> createState() => _MentionTextFieldState();
}

class _MentionTextFieldState extends ConsumerState<MentionTextField> {
  late FocusNode _focusNode;
  OverlayEntry? _overlayEntry;
  String _currentMentionQuery = '';
  int _mentionStartPos = -1;

  @override
  void initState() {
    super.initState();
    _focusNode = widget.focusNode ?? FocusNode();
    widget.controller.addListener(_onTextChanged);
    _focusNode.addListener(_onFocusChanged);
  }

  @override
  void dispose() {
    widget.controller.removeListener(_onTextChanged);
    _focusNode.removeListener(_onFocusChanged);
    if (widget.focusNode == null) {
      _focusNode.dispose();
    }
    _hideMentionSuggestions();
    super.dispose();
  }

  void _onFocusChanged() {
    if (!_focusNode.hasFocus) {
      _hideMentionSuggestions();
    }
  }

  void _onTextChanged() {
    final text = widget.controller.text;
    final cursorPos = widget.controller.selection.baseOffset;

    // Check if we're typing a mention
    if (cursorPos >= 0 && cursorPos <= text.length) {
      _checkForMention(text, cursorPos);
    }

    // Notify parent about mentions - resolve usernames to user IDs
    if (widget.onMentionsChanged != null) {
      _resolveMentionsAndNotify(text);
    }

    widget.onChanged?.call();
  }

  /// Resolve mentioned usernames to user IDs and notify parent
  Future<void> _resolveMentionsAndNotify(String text) async {
    final usernames = MentionParser.extractMentions(text);

    if (usernames.isEmpty) {
      widget.onMentionsChanged!([]);
      return;
    }

    // Resolve usernames to user IDs
    final resolver = ref.read(mentionResolverProvider);
    final usernameToId = await resolver.resolveUsernames(usernames);

    // Get only valid user IDs
    final userIds = usernameToId.values.toList();

    widget.onMentionsChanged!(userIds);
  }

  void _checkForMention(String text, int cursorPos) {
    // Find the last @ before cursor
    int atPos = -1;
    for (int i = cursorPos - 1; i >= 0; i--) {
      if (text[i] == '@') {
        atPos = i;
        break;
      } else if (text[i] == ' ' || text[i] == '\n') {
        // Space or newline breaks the mention
        break;
      }
    }

    if (atPos >= 0) {
      // We're in a mention
      final query = text.substring(atPos + 1, cursorPos);

      // Only show if query doesn't contain spaces
      if (!query.contains(' ') && !query.contains('\n')) {
        _mentionStartPos = atPos;
        _currentMentionQuery = query;
        _showMentionSuggestions();
        return;
      }
    }

    // Not in a mention, hide suggestions
    _hideMentionSuggestions();
  }

  void _showMentionSuggestions() {
    if (_overlayEntry != null) {
      // Already showing, just rebuild
      _overlayEntry!.markNeedsBuild();
      return;
    }

    _overlayEntry = _createOverlayEntry();
    Overlay.of(context).insert(_overlayEntry!);
  }

  void _hideMentionSuggestions() {
    _overlayEntry?.remove();
    _overlayEntry = null;
    _mentionStartPos = -1;
    _currentMentionQuery = '';
  }

  OverlayEntry _createOverlayEntry() {
    return OverlayEntry(
      builder: (context) {
        // Smart positioning: detect if TextField is near keyboard or not
        final screenHeight = MediaQuery.of(context).size.height;
        final keyboardHeight = MediaQuery.of(context).viewInsets.bottom;

        // Get TextField position
        final RenderBox? renderBox = context.findRenderObject() as RenderBox?;
        if (renderBox == null) {
          // Fallback: show above keyboard
          return Positioned(
            left: 0,
            right: 0,
            bottom: keyboardHeight + 8,
            child: Container(
              height: 250,
              padding: const EdgeInsets.symmetric(horizontal: 16),
              child: Material(
                color: Colors.transparent,
                child: MentionSuggestionOverlay(
                  query: _currentMentionQuery,
                  allowedUserIds: widget.allowedUserIds,
                  showSpecialMentions: widget.showSpecialMentions,
                  onUserSelected: _onUserSelected,
                  onDismiss: _hideMentionSuggestions,
                ),
              ),
            ),
          );
        }

        final textFieldOffset = renderBox.localToGlobal(Offset.zero);
        final textFieldHeight = renderBox.size.height;
        final textFieldBottom = textFieldOffset.dy + textFieldHeight;

        // Calculate distance from TextField bottom to keyboard top
        final keyboardTop = screenHeight - keyboardHeight;
        final distanceToKeyboard = keyboardTop - textFieldBottom;

        const overlayHeight = 250.0;

        // If TextField is close to keyboard (< 100px away) → Comment case
        // Show overlay ABOVE TextField to avoid covering it
        if (distanceToKeyboard < 100) {
          // Calculate bottom position to place overlay above TextField
          final overlayBottom = screenHeight - textFieldOffset.dy + 8;

          return Positioned(
            left: 0,
            right: 0,
            bottom: overlayBottom,
            child: Container(
              height: overlayHeight,
              padding: const EdgeInsets.symmetric(horizontal: 16),
              child: Material(
                color: Colors.transparent,
                child: MentionSuggestionOverlay(
                  query: _currentMentionQuery,
                  allowedUserIds: widget.allowedUserIds,
                  showSpecialMentions: widget.showSpecialMentions,
                  onUserSelected: _onUserSelected,
                  onDismiss: _hideMentionSuggestions,
                ),
              ),
            ),
          );
        } else {
          // TextField far from keyboard → Caption/content case
          // Show overlay above keyboard (doesn't cover TextField)
          return Positioned(
            left: 0,
            right: 0,
            bottom: keyboardHeight + 8,
            child: Container(
              height: overlayHeight,
              padding: const EdgeInsets.symmetric(horizontal: 16),
              child: Material(
                color: Colors.transparent,
                child: MentionSuggestionOverlay(
                  query: _currentMentionQuery,
                  allowedUserIds: widget.allowedUserIds,
                  showSpecialMentions: widget.showSpecialMentions,
                  onUserSelected: _onUserSelected,
                  onDismiss: _hideMentionSuggestions,
                ),
              ),
            ),
          );
        }
      },
    );
  }

  void _onUserSelected(UserSearch user) {
    final text = widget.controller.text;
    final cursorPos = widget.controller.selection.baseOffset;

    if (_mentionStartPos < 0) return;

    // Replace from @ to cursor with @username
    final newText =
        '${text.substring(0, _mentionStartPos)}@${user.username} ${text.substring(cursorPos)}';

    final newCursorPos = _mentionStartPos + user.username.length + 2;

    widget.controller.value = TextEditingValue(
      text: newText,
      selection: TextSelection.collapsed(offset: newCursorPos),
    );

    _hideMentionSuggestions();

    // Notify parent with resolved user IDs
    if (widget.onMentionsChanged != null) {
      _resolveMentionsAndNotify(newText);
    }
  }

  @override
  Widget build(BuildContext context) {
    return TextField(
      controller: widget.controller,
      focusNode: _focusNode,
      decoration:
          widget.decoration ??
          InputDecoration(
            hintText: widget.hintText,
            border: OutlineInputBorder(borderRadius: BorderRadius.circular(24)),
            contentPadding: const EdgeInsets.symmetric(
              horizontal: 16,
              vertical: 12,
            ),
          ),
      maxLines: widget.maxLines,
      minLines: widget.minLines,
      style: widget.style,
    );
  }
}
