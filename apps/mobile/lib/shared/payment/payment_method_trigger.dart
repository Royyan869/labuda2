import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

import 'payment_method_logo.dart';
import 'payment_method_visuals.dart';

/// Canonical payment-method selection TRIGGER/FIELD.
///
/// This is NOT a picker. It renders the current payment-method selection (or
/// the call-to-action to choose one) and delegates every action to the caller:
/// tapping the field invokes [onTap], which the consumer wires to the canonical
/// `PaymentMethodPickerSheet`. This widget owns presentation/interaction only —
/// it never loads methods, never touches a provider/controller/repository, and
/// never initiates or validates a payment. All business state stays with the
/// consumer.
///
/// Presentation authority is [PaymentMethodVisuals] via [PaymentMethodLogo]:
/// there is no method-code → icon/asset/label map here.
class PaymentMethodTrigger extends StatelessWidget {
  const PaymentMethodTrigger({
    super.key,
    required this.selectedMethodCode,
    required this.selectedMethodDisplayName,
    required this.isLoading,
    required this.hasMethods,
    this.errorMessage,
    this.onTap,
    this.onRetry,
    this.label = 'Payment method',
  });

  /// The selected canonical `method_code`, or null when none is selected. Used
  /// only to resolve the canonical visual through [PaymentMethodVisuals].
  final String? selectedMethodCode;

  /// The backend `display_name` of the selected method, or null when none is
  /// selected. No business label is invented when it is null.
  final String? selectedMethodDisplayName;

  /// Whether the consumer is currently loading the available methods.
  final bool isLoading;

  /// Whether the consumer has at least one method available to choose from.
  final bool hasMethods;

  /// Optional error. When no methods are available this drives the error state
  /// (with [onRetry]); when methods are available it is shown as a non-blocking
  /// note beneath the field so the consumer's flow (for example a failed
  /// initiation the seller must retry) is never hidden.
  final String? errorMessage;

  /// Invoked when the field is tapped (open the canonical picker). The consumer
  /// owns what that means.
  final VoidCallback? onTap;

  /// Invoked by the "Coba lagi" action on the empty/error state. The consumer
  /// owns what that means.
  final VoidCallback? onRetry;

  /// Field label.
  final String label;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          label,
          style: context.typeRoles.bodyDense.copyWith(
            fontWeight: FontWeight.w600,
            color: scheme.onSurfaceVariant,
          ),
        ),
        const SizedBox(height: 6),
        _buildField(context),
      ],
    );
  }

  Widget _buildField(BuildContext context) {
    if (isLoading) {
      return _shell(
        context,
        child: Row(
          children: [
            const SizedBox(
              height: 16,
              width: 16,
              child: CircularProgressIndicator(strokeWidth: 2),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Text(
                'Memuat metode pembayaran...',
                style: _textStyle(context),
              ),
            ),
          ],
        ),
      );
    }

    if (!hasMethods) {
      if (errorMessage != null) {
        return _errorRow(context);
      }
      return _shell(
        context,
        onTap: onRetry,
        child: Row(
          children: [
            Expanded(
              child: Text(
                'Tidak ada metode pembayaran tersedia',
                style: _textStyle(context, useErrorColor: true),
              ),
            ),
            if (onRetry != null) _chevron(context),
          ],
        ),
      );
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _shell(
          context,
          onTap: onTap,
          child: Row(
            children: [
              PaymentMethodLogo(
                visual: PaymentMethodVisuals.visual(selectedMethodCode),
                size: 24,
                maxWidth: 88,
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Text(
                  selectedMethodDisplayName ?? 'Pilih metode pembayaran',
                  style: _textStyle(context),
                ),
              ),
              _chevron(context),
            ],
          ),
        ),
        if (errorMessage != null) ...[
          const SizedBox(height: 8),
          Text(
            errorMessage!,
            style: _textStyle(context, useErrorColor: true),
          ),
        ],
      ],
    );
  }

  Widget _errorRow(BuildContext context) {
    return Row(
      children: [
        Icon(
          Icons.error_outline,
          color: context.statusColors.error,
          size: AppIconSize.action,
        ),
        const SizedBox(width: 12),
        Expanded(
          child: Text(
            errorMessage!,
            style: context.typeRoles.bodyDense.copyWith(
              color: Theme.of(context).colorScheme.onSurfaceVariant,
            ),
          ),
        ),
        if (onRetry != null)
          TextButton(onPressed: onRetry, child: const Text('Coba lagi')),
      ],
    );
  }

  TextStyle _textStyle(BuildContext context, {bool useErrorColor = false}) {
    return context.typeRoles.bodyDense.copyWith(
      color: useErrorColor
          ? context.statusColors.error
          : Theme.of(context).colorScheme.onSurface,
    );
  }

  Widget _chevron(BuildContext context) {
    return Icon(
      Icons.chevron_right,
      size: AppIconSize.action,
      color: Theme.of(context).colorScheme.onSurfaceVariant,
    );
  }

  Widget _shell(
    BuildContext context, {
    required Widget child,
    VoidCallback? onTap,
  }) {
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(AppShape.r8),
      child: Container(
        padding: const EdgeInsets.symmetric(
          horizontal: AppMetrics.p12,
          vertical: AppMetrics.p12,
        ),
        decoration: BoxDecoration(
          borderRadius: BorderRadius.circular(AppShape.r8),
          border: Border.all(
            color: Theme.of(context).colorScheme.outlineVariant,
          ),
        ),
        child: child,
      ),
    );
  }
}
