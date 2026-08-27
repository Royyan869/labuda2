import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/helpers/canonical_password_strength.dart';

/// Canonical password strength meter.
///
/// Consumes the single canonical strength authority
/// ([CanonicalPasswordStrength]) and renders the three-state classification
/// (Weak / Medium / Strong) with a progress bar.
///
/// - Empty password → neutral state (no bar fill, no label).
/// - Strength updates purely from [password]; the widget rebuilds whenever
///   the parent rebuilds with a new value (realtime on typing is the parent
///   screen's responsibility via controller listener / onChanged).
///
/// This is UX feedback only.  Password validity remains the authority of
/// [CanonicalPasswordPolicy]; this widget never gates submission.
class PasswordStrengthIndicator extends StatelessWidget {
  final String password;
  final bool isDark;

  const PasswordStrengthIndicator({
    super.key,
    required this.password,
    required this.isDark,
  });

  @override
  Widget build(BuildContext context) {
    final level = CanonicalPasswordStrength.evaluate(password);

    // Neutral state for empty input: no classification, no "Weak" warning.
    if (level == null) {
      return const SizedBox.shrink();
    }

    final color = _colorFor(level);
    final progress = CanonicalPasswordStrength.progress(password);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        const SizedBox(height: 8),
        Row(
          children: [
            Expanded(
              child: LinearProgressIndicator(
                value: progress,
                backgroundColor: isDark
                    ? AppColors.darkGray600
                    : AppColors.neutralGray200,
                valueColor: AlwaysStoppedAnimation<Color>(color),
                minHeight: 4,
              ),
            ),
            const SizedBox(width: 8),
            Text(
              level.label,
              style: TextStyle(
                fontSize: 12,
                fontWeight: FontWeight.w500,
                color: color,
              ),
            ),
          ],
        ),
      ],
    );
  }

  Color _colorFor(PasswordStrengthLevel level) {
    switch (level) {
      case PasswordStrengthLevel.weak:
        return AppColors.error;
      case PasswordStrengthLevel.medium:
        return AppColors.warning;
      case PasswordStrengthLevel.strong:
        return AppColors.success;
    }
  }
}
