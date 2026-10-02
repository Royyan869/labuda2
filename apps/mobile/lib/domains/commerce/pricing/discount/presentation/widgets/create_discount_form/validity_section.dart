import 'package:flutter/material.dart';
import 'package:intl/intl.dart';
import 'package:labuda/core/core.dart' as core;
import 'package:labuda/core/src/theme/app_theme.dart';

/// Section untuk validity period discount
///
/// CANONICAL MODEL: Only validUntil (expiry-only).
/// Discount becomes active on creation and expires at validUntil.
class ValiditySection extends StatelessWidget {
  final DateTime validUntil;
  final ValueChanged<DateTime> onValidUntilChanged;

  const ValiditySection({
    super.key,
    required this.validUntil,
    required this.onValidUntilChanged,
  });

  String _formatDate(DateTime date) {
    return DateFormat('dd MMM yyyy', 'id_ID').format(date);
  }

  Future<void> _selectDate(
    BuildContext context,
    DateTime initialDate,
    ValueChanged<DateTime> onChanged,
  ) async {
    final picked = await showDatePicker(
      context: context,
      initialDate: initialDate,
      firstDate: DateTime.now(),
      lastDate: DateTime.now().add(const Duration(days: 365)),
    );

    if (picked != null) {
      onChanged(picked);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(core.AppMetrics.p16),
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.surface,
        borderRadius: BorderRadius.circular(core.AppShape.r12),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            'Expiry Date',
            style: TextStyle(
              fontSize: core.AppType.s16,
              fontWeight: FontWeight.bold,
              color: Theme.of(context).colorScheme.onSurface,
            ),
          ),
          const SizedBox(height: 16),

          // Valid Until
          Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                'Expires On *',
                style: TextStyle(
                  fontSize: core.AppType.s14,
                  fontWeight: FontWeight.w500,
                  color: Theme.of(context).colorScheme.onSurfaceVariant,
                ),
              ),
              const SizedBox(height: 8),
              InkWell(
                onTap: () {
                  _selectDate(context, validUntil, onValidUntilChanged);
                },
                borderRadius: BorderRadius.circular(core.AppShape.r12),
                child: Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: core.AppMetrics.p16,
                    vertical: core.AppMetrics.p16,
                  ),
                  decoration: BoxDecoration(
                    color: Theme.of(context).colorScheme.surfaceContainer,
                    borderRadius: BorderRadius.circular(core.AppShape.r12),
                    border: Border.all(
                      color: Theme.of(context).colorScheme.outlineVariant,
                    ),
                  ),
                  child: Row(
                    children: [
                      Icon(
                        Icons.event,
                        size: AppIconSize.action,
                        color: Theme.of(context).colorScheme.onSurfaceVariant,
                      ),
                      const SizedBox(width: 12),
                      Expanded(
                        child: Text(
                          _formatDate(validUntil),
                          style: TextStyle(
                            fontSize: core.AppType.s14,
                            color: Theme.of(context).colorScheme.onSurface,
                          ),
                        ),
                      ),
                      Icon(
                        Icons.arrow_drop_down,
                        color: Theme.of(context).colorScheme.onSurfaceVariant,
                      ),
                    ],
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: 12),

          // Info
          Container(
            padding: const EdgeInsets.all(core.AppMetrics.p12),
            decoration: BoxDecoration(
              color: context.statusColors.info.withValues(alpha: 0.1),
              borderRadius: BorderRadius.circular(core.AppShape.r8),
            ),
            child: Row(
              children: [
                Icon(
                  Icons.info_outline,
                  size: AppIconSize.inlineGlyph,
                  color: context.statusColors.info,
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    'Discount is active immediately and expires on the selected date.',
                    style: TextStyle(
                      fontSize: core.AppType.s12,
                      color: Theme.of(context).colorScheme.onSurfaceVariant,
                    ),
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
