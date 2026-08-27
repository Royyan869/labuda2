import 'package:labuda/core/src/theme/app_colors.dart';
import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Reusable Date/DateTime Picker widget dengan styling konsisten sesuai LABUDA design
///
/// Features:
/// - Consistent styling dengan AppTextField
/// - Support date-only atau date+time
/// - Dark/light theme support
/// - Validation support
/// - Custom date range support
class AppDatePicker extends StatelessWidget {
  final String? labelText;
  final String? hintText;
  final IconData? prefixIcon;
  final DateTime? selectedDate;
  final void Function(DateTime?)? onChanged;
  final String? Function(DateTime?)? validator;
  final bool enabled;
  final bool showTime;
  final DateTime? firstDate;
  final DateTime? lastDate;

  const AppDatePicker({
    super.key,
    this.labelText,
    this.hintText,
    this.prefixIcon,
    this.selectedDate,
    this.onChanged,
    this.validator,
    this.enabled = true,
    this.showTime = true,
    this.firstDate,
    this.lastDate,
  });

  /// Factory untuk date-only picker
  const AppDatePicker.dateOnly({
    super.key,
    this.labelText,
    this.hintText,
    this.prefixIcon = Icons.calendar_today,
    this.selectedDate,
    this.onChanged,
    this.validator,
    this.enabled = true,
    this.firstDate,
    this.lastDate,
  }) : showTime = false;

  /// Factory untuk date+time picker
  const AppDatePicker.dateTime({
    super.key,
    this.labelText,
    this.hintText,
    this.prefixIcon = Icons.calendar_today,
    this.selectedDate,
    this.onChanged,
    this.validator,
    this.enabled = true,
    this.firstDate,
    this.lastDate,
  }) : showTime = true;

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return GestureDetector(
      onTap: enabled ? () => _selectDateTime(context) : null,
      child: AbsorbPointer(
        child: TextFormField(
          controller: TextEditingController(
            text: selectedDate != null ? _formatDate(selectedDate!) : '',
          ),
          decoration: InputDecoration(
            labelText: labelText,
            hintText:
                hintText ?? (showTime ? 'Select date & time' : 'Select date'),
            floatingLabelBehavior: FloatingLabelBehavior.always,
            prefixIcon: prefixIcon != null
                ? Icon(
                    prefixIcon,
                    color: isDark
                        ? AppColors.neutralGray400
                        : AppColors.neutralGray500,
                  )
                : null,
            border: OutlineInputBorder(
              borderRadius: BorderRadius.circular(12),
              borderSide: BorderSide(
                color: isDark
                    ? AppColors.darkGray600
                    : AppColors.neutralGray300,
              ),
            ),
            enabledBorder: OutlineInputBorder(
              borderRadius: BorderRadius.circular(12),
              borderSide: BorderSide(
                color: isDark
                    ? AppColors.darkGray600
                    : AppColors.neutralGray300,
              ),
            ),
            focusedBorder: OutlineInputBorder(
              borderRadius: BorderRadius.circular(12),
              borderSide: const BorderSide(
                color: AppColors.primaryRed,
                width: 2,
              ),
            ),
            errorBorder: OutlineInputBorder(
              borderRadius: BorderRadius.circular(12),
              borderSide: const BorderSide(color: AppColors.error),
            ),
            focusedErrorBorder: OutlineInputBorder(
              borderRadius: BorderRadius.circular(12),
              borderSide: const BorderSide(color: AppColors.error, width: 2),
            ),
          ),
          validator: validator != null
              ? (value) => validator!(selectedDate)
              : null,
        ),
      ),
    );
  }

  Future<void> _selectDateTime(BuildContext context) async {
    // Step 1: Pick date
    final DateTime? pickedDate = await showDatePicker(
      context: context,
      initialDate: selectedDate ?? DateTime.now(),
      firstDate: firstDate ?? DateTime.now(),
      lastDate: lastDate ?? DateTime.now().add(const Duration(days: 365)),
    );

    if (pickedDate != null && context.mounted) {
      if (showTime) {
        // Step 2: Pick time if showTime is true
        final TimeOfDay? pickedTime = await _showCustomTimePicker(context);
        if (pickedTime != null) {
          final DateTime dateTime = DateTime(
            pickedDate.year,
            pickedDate.month,
            pickedDate.day,
            pickedTime.hour,
            pickedTime.minute,
          );
          onChanged?.call(dateTime);
        }
      } else {
        // Date only - set time to 00:00
        final DateTime dateTime = DateTime(
          pickedDate.year,
          pickedDate.month,
          pickedDate.day,
        );
        onChanged?.call(dateTime);
      }
    }
  }

  Future<TimeOfDay?> _showCustomTimePicker(BuildContext context) async {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    TimeOfDay selectedTime = TimeOfDay.now();

    return showDialog<TimeOfDay>(
      context: context,
      builder: (BuildContext context) {
        return StatefulBuilder(
          builder: (context, setState) {
            return AlertDialog(
              backgroundColor: isDark ? AppColors.darkGray800 : AppColors.light,
              title: Text(
                'Select Time',
                style: TextStyle(
                  color: isDark
                      ? AppColors.neutralWhite
                      : AppColors.neutralGray900,
                ),
              ),
              content: Row(
                mainAxisAlignment: MainAxisAlignment.spaceEvenly,
                children: [
                  // Hour picker
                  Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Text(
                        'Hour',
                        style: TextStyle(
                          color: isDark
                              ? AppColors.neutralGray300
                              : AppColors.neutralGray700,
                        ),
                      ),
                      const SizedBox(height: 8),
                      SizedBox(
                        height: 120,
                        width: 60,
                        child: ListWheelScrollView.useDelegate(
                          itemExtent: 40,
                          physics: const FixedExtentScrollPhysics(),
                          onSelectedItemChanged: (index) {
                            setState(() {
                              selectedTime = TimeOfDay(
                                hour: index,
                                minute: selectedTime.minute,
                              );
                            });
                          },
                          childDelegate: ListWheelChildBuilderDelegate(
                            builder: (context, index) {
                              if (index >= 24) return null;
                              return Center(
                                child: Text(
                                  index.toString().padLeft(2, '0'),
                                  style: TextStyle(
                                    fontSize: 18,
                                    color: isDark
                                        ? AppColors.neutralWhite
                                        : AppColors.neutralGray900,
                                  ),
                                ),
                              );
                            },
                          ),
                        ),
                      ),
                    ],
                  ),
                  Text(
                    ':',
                    style: TextStyle(
                      fontSize: 24,
                      color: isDark
                          ? AppColors.neutralWhite
                          : AppColors.neutralGray900,
                    ),
                  ),
                  // Minute picker
                  Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Text(
                        'Minute',
                        style: TextStyle(
                          color: isDark
                              ? AppColors.neutralGray300
                              : AppColors.neutralGray700,
                        ),
                      ),
                      const SizedBox(height: 8),
                      SizedBox(
                        height: 120,
                        width: 60,
                        child: ListWheelScrollView.useDelegate(
                          itemExtent: 40,
                          physics: const FixedExtentScrollPhysics(),
                          onSelectedItemChanged: (index) {
                            setState(() {
                              selectedTime = TimeOfDay(
                                hour: selectedTime.hour,
                                minute: index * 5,
                              );
                            });
                          },
                          childDelegate: ListWheelChildBuilderDelegate(
                            builder: (context, index) {
                              if (index >= 12) return null; // 0, 5, 10, ..., 55
                              final minute = index * 5;
                              return Center(
                                child: Text(
                                  minute.toString().padLeft(2, '0'),
                                  style: TextStyle(
                                    fontSize: 18,
                                    color: isDark
                                        ? AppColors.neutralWhite
                                        : AppColors.neutralGray900,
                                  ),
                                ),
                              );
                            },
                          ),
                        ),
                      ),
                    ],
                  ),
                ],
              ),
              actions: [
                TextButton(
                  onPressed: () => Navigator.of(context).pop(),
                  child: Text(
                    'Cancel',
                    style: TextStyle(
                      color: isDark
                          ? AppColors.neutralGray400
                          : AppColors.neutralGray600,
                    ),
                  ),
                ),
                ElevatedButton(
                  onPressed: () => Navigator.of(context).pop(selectedTime),
                  style: ElevatedButton.styleFrom(
                    backgroundColor: AppColors.primaryRed,
                    foregroundColor: AppColors.light,
                  ),
                  child: const Text('OK'),
                ),
              ],
            );
          },
        );
      },
    );
  }

  String _formatDate(DateTime dateTime) {
    if (showTime) {
      return '${dateTime.day.toString().padLeft(2, '0')}/${dateTime.month.toString().padLeft(2, '0')}/${dateTime.year} ${dateTime.hour.toString().padLeft(2, '0')}:${dateTime.minute.toString().padLeft(2, '0')}';
    } else {
      return '${dateTime.day.toString().padLeft(2, '0')}/${dateTime.month.toString().padLeft(2, '0')}/${dateTime.year}';
    }
  }
}
