import 'package:flutter/material.dart';

/// Quick action item for extended actions panel
class QuickAction {
  final IconData icon;
  final String label;
  final Color color;
  final VoidCallback onTap;

  const QuickAction({
    required this.icon,
    required this.label,
    required this.color,
    required this.onTap,
  });
}

/// Text Input Actions Widget - Generic action buttons for text inputs
class TextInputActions extends StatelessWidget {
  final List<QuickAction> actions;
  final bool isDark;

  const TextInputActions({
    super.key,
    required this.actions,
    required this.isDark,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      height: 48, // Reduced from 60 to 48 for compact design
      alignment: Alignment.centerLeft, // Force left alignment
      child: ListView.separated(
        scrollDirection: Axis.horizontal,
        clipBehavior: Clip.none,
        padding: const EdgeInsets.only(left: 12, right: 12, top: 4, bottom: 4),
        shrinkWrap: true,
        itemCount: actions.length,
        separatorBuilder: (context, index) => const SizedBox(width: 12),
        itemBuilder: (context, index) {
          final action = actions[index];
          return _buildActionButton(
            icon: action.icon,
            label: action.label,
            color: action.color,
            onTap: action.onTap,
          );
        },
      ),
    );
  }

  Widget _buildActionButton({
    required IconData icon,
    required String label,
    required Color color,
    required VoidCallback onTap,
  }) {
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(8),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(icon, color: color, size: 20),
            const SizedBox(height: 2),
            Text(
              label,
              style: TextStyle(
                color: color,
                fontSize: 10,
                fontWeight: FontWeight.w500,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
