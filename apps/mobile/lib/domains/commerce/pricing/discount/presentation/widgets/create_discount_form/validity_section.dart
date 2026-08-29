import 'package:flutter/material.dart';
import 'package:intl/intl.dart';
import 'package:labuda/core/core.dart' as core;

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
      builder: (context, child) {
        return Theme(
          data: Theme.of(context).copyWith(
            colorScheme: ColorScheme.light(
              primary: core.AppColors.primaryRed,
              onPrimary: core.AppColors.neutralWhite,
              surface: core.AppColors.neutralWhite,
              onSurface: core.AppColors.neutralGray900,
            ),
          ),
          child: child!,
        );
      },
    );

    if (picked != null) {
      onChanged(picked);
    }
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: isDark
            ? core.AppColors.darkGray800
            : core.AppColors.neutralWhite,
        borderRadius: BorderRadius.circular(12),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            'Expiry Date',
            style: TextStyle(
              fontSize: 16,
              fontWeight: FontWeight.bold,
              color: isDark
                  ? core.AppColors.neutralWhite
                  : core.AppColors.neutralGray900,
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
                  fontSize: 14,
                  fontWeight: FontWeight.w500,
                  color: isDark
                      ? core.AppColors.neutralGray300
                      : core.AppColors.neutralGray700,
                ),
              ),
              const SizedBox(height: 8),
              InkWell(
                onTap: () {
                  _selectDate(context, validUntil, onValidUntilChanged);
                },
                borderRadius: BorderRadius.circular(12),
                child: Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: 16,
                    vertical: 14,
                  ),
                  decoration: BoxDecoration(
                    color: isDark
                        ? core.AppColors.darkGray700
                        : core.AppColors.neutralGray50,
                    borderRadius: BorderRadius.circular(12),
                    border: Border.all(
                      color: isDark
                          ? core.AppColors.neutralGray700
                          : core.AppColors.neutralGray200,
                    ),
                  ),
                  child: Row(
                    children: [
                      Icon(
                        Icons.event,
                        size: 20,
                        color: isDark
                            ? core.AppColors.neutralGray400
                            : core.AppColors.neutralGray600,
                      ),
                      const SizedBox(width: 12),
                      Expanded(
                        child: Text(
                          _formatDate(validUntil),
                          style: TextStyle(
                            fontSize: 14,
                            color: isDark
                                ? core.AppColors.neutralWhite
                                : core.AppColors.neutralGray900,
                          ),
                        ),
                      ),
                      Icon(
                        Icons.arrow_drop_down,
                        color: isDark
                            ? core.AppColors.neutralGray400
                            : core.AppColors.neutralGray600,
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
            padding: const EdgeInsets.all(10),
            decoration: BoxDecoration(
              color: core.AppColors.statusInfo.withValues(alpha: 0.1),
              borderRadius: BorderRadius.circular(8),
            ),
            child: Row(
              children: [
                Icon(
                  Icons.info_outline,
                  size: 16,
                  color: core.AppColors.statusInfo,
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    'Discount is active immediately and expires on the selected date.',
                    style: TextStyle(
                      fontSize: 12,
                      color: isDark
                          ? core.AppColors.neutralGray300
                          : core.AppColors.neutralGray700,
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
