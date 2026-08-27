import 'package:flutter/material.dart';
import 'package:intl/intl.dart';
import 'package:labuda/core/core.dart' as core;

/// Section untuk validity period discount
class ValiditySection extends StatelessWidget {
  final DateTime validFrom;
  final DateTime validUntil;
  final ValueChanged<DateTime> onValidFromChanged;
  final ValueChanged<DateTime> onValidUntilChanged;

  const ValiditySection({
    super.key,
    required this.validFrom,
    required this.validUntil,
    required this.onValidFromChanged,
    required this.onValidUntilChanged,
  });

  String _formatDate(DateTime date) {
    return DateFormat('dd MMM yyyy', 'id_ID').format(date);
  }

  Future<void> _selectDate(
    BuildContext context,
    DateTime initialDate,
    ValueChanged<DateTime> onChanged,
    DateTime? firstDate,
    DateTime? lastDate,
  ) async {
    final picked = await showDatePicker(
      context: context,
      initialDate: initialDate,
      firstDate: firstDate ?? DateTime.now(),
      lastDate: lastDate ?? DateTime.now().add(const Duration(days: 365)),
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
            'Validity Period',
            style: TextStyle(
              fontSize: 16,
              fontWeight: FontWeight.bold,
              color: isDark
                  ? core.AppColors.neutralWhite
                  : core.AppColors.neutralGray900,
            ),
          ),
          const SizedBox(height: 16),

          // Valid From
          _buildDateField(
            context,
            isDark,
            label: 'Valid From *',
            date: validFrom,
            icon: Icons.calendar_today,
            onTap: () {
              _selectDate(
                context,
                validFrom,
                onValidFromChanged,
                DateTime.now(),
                validUntil,
              );
            },
          ),
          const SizedBox(height: 16),

          // Valid Until
          _buildDateField(
            context,
            isDark,
            label: 'Valid Until *',
            date: validUntil,
            icon: Icons.event,
            onTap: () {
              _selectDate(
                context,
                validUntil,
                onValidUntilChanged,
                validFrom,
                DateTime.now().add(const Duration(days: 365)),
              );
            },
          ),
          const SizedBox(height: 12),

          // Duration info
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
                    'Duration: ${validUntil.difference(validFrom).inDays} days',
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

  Widget _buildDateField(
    BuildContext context,
    bool isDark, {
    required String label,
    required DateTime date,
    required IconData icon,
    required VoidCallback onTap,
  }) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          label,
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
          onTap: onTap,
          borderRadius: BorderRadius.circular(12),
          child: Container(
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 14),
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
                  icon,
                  size: 20,
                  color: isDark
                      ? core.AppColors.neutralGray400
                      : core.AppColors.neutralGray600,
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Text(
                    _formatDate(date),
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
    );
  }
}
