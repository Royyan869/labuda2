import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'email_verify_bottom_sheet.dart';

/// Inline gate for actions the backend will reject with
/// `EMAIL_VERIFICATION_REQUIRED` (HTTP 403). Triggered from the call-site
/// when the user attempts a blocked action while their email is unverified.
///
/// API:
///   await showBlockedActionGate(context, actionDescription: "menulis komentar");
///
/// The user stays on the current screen; the gate appears as a modal bottom
/// sheet with two CTAs:
///   - "Verifikasi Sekarang" → opens [EmailVerifyBottomSheet]
///   - "Nanti" → dismiss
///
/// Recommended pattern at call sites:
/// ```dart
/// final result = await api.postComment(...);
/// if (result.isError && result.errorCode == 'EMAIL_VERIFICATION_REQUIRED') {
///   await showBlockedActionGate(context, actionDescription: 'menulis komentar');
///   return;
/// }
/// ```
Future<void> showBlockedActionGate(
  BuildContext context, {
  required String actionDescription,
}) {
  return showModalBottomSheet<void>(
    context: context,
    isScrollControlled: true,
    useRootNavigator: true,
    backgroundColor: Colors.transparent,
    builder: (sheetContext) {
      final isDark = Theme.of(sheetContext).brightness == Brightness.dark;
      final bgColor = isDark ? AppColors.darkGray800 : AppColors.neutralWhite;
      final fgColor = isDark
          ? AppColors.neutralWhite
          : AppColors.neutralGray800;
      final mutedColor = isDark
          ? AppColors.neutralGray300
          : AppColors.neutralGray600;

      return Padding(
        padding: MediaQuery.of(sheetContext).viewInsets,
        child: Container(
          decoration: BoxDecoration(
            color: bgColor,
            borderRadius: const BorderRadius.vertical(top: Radius.circular(20)),
          ),
          padding: const EdgeInsets.fromLTRB(20, 12, 20, 24),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Center(
                child: Container(
                  width: 44,
                  height: 4,
                  decoration: BoxDecoration(
                    color: AppColors.neutralGray300,
                    borderRadius: BorderRadius.circular(2),
                  ),
                ),
              ),
              const SizedBox(height: 20),
              Row(
                children: [
                  Container(
                    padding: const EdgeInsets.all(8),
                    decoration: BoxDecoration(
                      color: AppColors.statusWarning.withValues(alpha: 0.15),
                      shape: BoxShape.circle,
                    ),
                    child: const Icon(
                      Icons.mark_email_unread_outlined,
                      color: AppColors.statusWarning,
                      size: 20,
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Text(
                      'Verifikasi Email Diperlukan',
                      style: TextStyle(
                        fontSize: 17,
                        fontWeight: FontWeight.bold,
                        color: fgColor,
                      ),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 12),
              Text(
                'Untuk $actionDescription, verifikasi email kamu dulu.',
                style: TextStyle(fontSize: 14, color: mutedColor, height: 1.4),
              ),
              const SizedBox(height: 20),
              ElevatedButton(
                onPressed: () {
                  Navigator.of(sheetContext).pop();
                  EmailVerifyBottomSheet.show(context);
                },
                style: ElevatedButton.styleFrom(
                  backgroundColor: AppColors.primaryRed,
                  foregroundColor: AppColors.light,
                  padding: const EdgeInsets.symmetric(vertical: 14),
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(12),
                  ),
                ),
                child: const Text(
                  'Verifikasi Sekarang',
                  style: TextStyle(fontSize: 15, fontWeight: FontWeight.w600),
                ),
              ),
              const SizedBox(height: 10),
              TextButton(
                onPressed: () => Navigator.of(sheetContext).pop(),
                style: TextButton.styleFrom(
                  foregroundColor: mutedColor,
                  padding: const EdgeInsets.symmetric(vertical: 12),
                ),
                child: const Text(
                  'Nanti',
                  style: TextStyle(fontSize: 15, fontWeight: FontWeight.w600),
                ),
              ),
            ],
          ),
        ),
      );
    },
  );
}
