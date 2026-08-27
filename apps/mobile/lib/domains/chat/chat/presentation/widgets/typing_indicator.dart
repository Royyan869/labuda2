import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_providers.dart';

/// Typing Indicator Widget
///
/// Shows when other users are typing in the chat.
class TypingIndicatorWidget extends ConsumerWidget {
  final String chatId;

  const TypingIndicatorWidget({super.key, required this.chatId});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final chatDetailState = ref.watch(chatDetailProvider(chatId));
    final typingUsers = chatDetailState.typingUsers;

    final typingUserList = typingUsers.entries.where((e) => e.value).toList();

    if (typingUserList.isEmpty) {
      return const SizedBox.shrink();
    }

    return _buildTypingIndicator(context, typingUserList);
  }

  Widget _buildTypingIndicator(
    BuildContext context,
    List<MapEntry<String, bool>> typingUsers,
  ) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
      child: Row(
        children: [
          SizedBox(width: 16, height: 16, child: _buildTypingDots(context)),
          const SizedBox(width: 8),
          Text(
            _getTypingText(typingUsers),
            style: TextStyle(
              fontSize: 12,
              color: Colors.grey[600],
              fontStyle: FontStyle.italic,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildTypingDots(BuildContext context) {
    return const Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        _TypingDot(delay: Duration(milliseconds: 0)),
        _TypingDot(delay: Duration(milliseconds: 200)),
        _TypingDot(delay: Duration(milliseconds: 400)),
      ],
    );
  }

  String _getTypingText(List<MapEntry<String, bool>> typingUsers) {
    if (typingUsers.isEmpty) return '';

    if (typingUsers.length == 1) {
      return 'Someone is typing...';
    } else if (typingUsers.length == 2) {
      return '2 people are typing...';
    } else {
      return '${typingUsers.length} people are typing...';
    }
  }
}

/// Typing Dot Animation
class _TypingDot extends StatefulWidget {
  final Duration delay;

  const _TypingDot({required this.delay});

  @override
  State<_TypingDot> createState() => _TypingDotState();
}

class _TypingDotState extends State<_TypingDot>
    with SingleTickerProviderStateMixin {
  late AnimationController _controller;
  late Animation<double> _animation;

  @override
  void initState() {
    super.initState();
    _controller = AnimationController(
      duration: const Duration(milliseconds: 600),
      vsync: this,
    );

    _animation = Tween<double>(
      begin: 0.3,
      end: 1.0,
    ).animate(CurvedAnimation(parent: _controller, curve: Curves.easeInOut));

    // Start animation after delay
    Future.delayed(widget.delay, () {
      if (mounted) {
        _controller.repeat(reverse: true);
      }
    });
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: _animation,
      builder: (context, child) {
        return Opacity(
          opacity: _animation.value,
          child: Container(
            width: 4,
            height: 4,
            decoration: BoxDecoration(
              color: Colors.grey[600],
              shape: BoxShape.circle,
            ),
          ),
        );
      },
    );
  }
}

/// Compact Typing Indicator
///
/// Smaller version for use in chat cards.
class CompactTypingIndicator extends StatelessWidget {
  final bool isTyping;

  const CompactTypingIndicator({super.key, required this.isTyping});

  @override
  Widget build(BuildContext context) {
    if (!isTyping) return const SizedBox.shrink();

    return SizedBox(width: 16, height: 16, child: _buildTypingDots(context));
  }

  Widget _buildTypingDots(BuildContext context) {
    return const Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        _CompactTypingDot(delay: Duration(milliseconds: 0)),
        _CompactTypingDot(delay: Duration(milliseconds: 150)),
        _CompactTypingDot(delay: Duration(milliseconds: 300)),
      ],
    );
  }
}

/// Compact Typing Dot
class _CompactTypingDot extends StatefulWidget {
  final Duration delay;

  const _CompactTypingDot({required this.delay});

  @override
  State<_CompactTypingDot> createState() => _CompactTypingDotState();
}

class _CompactTypingDotState extends State<_CompactTypingDot>
    with SingleTickerProviderStateMixin {
  late AnimationController _controller;
  late Animation<double> _animation;

  @override
  void initState() {
    super.initState();
    _controller = AnimationController(
      duration: const Duration(milliseconds: 500),
      vsync: this,
    );

    _animation = Tween<double>(
      begin: 0.3,
      end: 1.0,
    ).animate(CurvedAnimation(parent: _controller, curve: Curves.easeInOut));

    Future.delayed(widget.delay, () {
      if (mounted) {
        _controller.repeat(reverse: true);
      }
    });
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: _animation,
      builder: (context, child) {
        return Opacity(
          opacity: _animation.value,
          child: Container(
            width: 3,
            height: 3,
            margin: const EdgeInsets.symmetric(horizontal: 0.5),
            decoration: BoxDecoration(
              color: Colors.grey[600],
              shape: BoxShape.circle,
            ),
          ),
        );
      },
    );
  }
}
