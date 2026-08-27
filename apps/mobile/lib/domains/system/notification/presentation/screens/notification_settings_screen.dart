library;

import 'package:flutter/material.dart';

/// Read-only notification settings placeholder.
///
/// This screen is intentionally limited to device-level guidance and a clear
/// under-development message. Backend preference APIs are not called here.
class NotificationSettingsScreen extends StatelessWidget {
  const NotificationSettingsScreen({super.key});

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final backgroundColor = isDark
        ? const Color(0xFF0F172A)
        : const Color(0xFFF8FAFC);
    final surfaceColor = isDark ? const Color(0xFF1E293B) : Colors.white;
    final headingColor = isDark ? Colors.white : const Color(0xFF0F172A);
    final bodyColor = isDark
        ? const Color(0xFFCBD5E1)
        : const Color(0xFF475569);

    return Scaffold(
      backgroundColor: backgroundColor,
      appBar: AppBar(
        elevation: 0,
        backgroundColor: surfaceColor,
        foregroundColor: headingColor,
        surfaceTintColor: Colors.transparent,
        title: const Text('Notification Settings'),
      ),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          _StatusBanner(
            surfaceColor: surfaceColor,
            headingColor: headingColor,
            bodyColor: bodyColor,
          ),
          const SizedBox(height: 16),
          _SectionCard(
            surfaceColor: surfaceColor,
            title: 'Current scope',
            children: const [
              _ReadOnlyRow(
                icon: Icons.phone_android_outlined,
                title: 'Device notifications',
                subtitle:
                    'Manage push permissions from your phone or tablet settings.',
              ),
              _ReadOnlyRow(
                icon: Icons.lock_outline,
                title: 'Backend preferences',
                subtitle:
                    'Not available yet in this build, so no save action is shown.',
              ),
              _ReadOnlyRow(
                icon: Icons.notifications_none,
                title: 'In-app delivery',
                subtitle:
                    'Notification list and read-state APIs remain available elsewhere in the app.',
              ),
            ],
          ),
          const SizedBox(height: 16),
          _SectionCard(
            surfaceColor: surfaceColor,
            title: 'What changed',
            children: [
              Text(
                'This screen is intentionally read-only for now. '
                'It is safe to open from Settings, but it does not submit '
                'any unsupported preference updates.',
                style: TextStyle(color: bodyColor, height: 1.45),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _StatusBanner extends StatelessWidget {
  final Color surfaceColor;
  final Color headingColor;
  final Color bodyColor;

  const _StatusBanner({
    required this.surfaceColor,
    required this.headingColor,
    required this.bodyColor,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: surfaceColor,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(
          color: const Color(0xFFF59E0B).withValues(alpha: 0.25),
        ),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Icon(Icons.info_outline, color: Color(0xFFF59E0B)),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Notification settings are under development',
                  style: TextStyle(
                    color: headingColor,
                    fontSize: 16,
                    fontWeight: FontWeight.w600,
                  ),
                ),
                const SizedBox(height: 6),
                Text(
                  'You can open this page safely, but preferences are not saved '
                  'from this screen yet.',
                  style: TextStyle(color: bodyColor, height: 1.45),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _SectionCard extends StatelessWidget {
  final Color surfaceColor;
  final String title;
  final List<Widget> children;

  const _SectionCard({
    required this.surfaceColor,
    required this.title,
    required this.children,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: BoxDecoration(
        color: surfaceColor,
        borderRadius: BorderRadius.circular(16),
      ),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              title,
              style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w600),
            ),
            const SizedBox(height: 12),
            ...children,
          ],
        ),
      ),
    );
  }
}

class _ReadOnlyRow extends StatelessWidget {
  final IconData icon;
  final String title;
  final String subtitle;

  const _ReadOnlyRow({
    required this.icon,
    required this.title,
    required this.subtitle,
  });

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;
    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(icon, size: 20, color: colorScheme.primary),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  title,
                  style: const TextStyle(
                    fontWeight: FontWeight.w600,
                    fontSize: 14,
                  ),
                ),
                const SizedBox(height: 4),
                Text(
                  subtitle,
                  style: TextStyle(
                    fontSize: 13,
                    color: colorScheme.onSurfaceVariant,
                    height: 1.4,
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
