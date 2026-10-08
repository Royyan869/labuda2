import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Wizard Progress Indicator - Reusable Component
///
/// Compact & responsive progress indicator untuk multi-step wizards
/// Dipakai di: Seller Upgrade, Product, Auction
///
/// Features:
/// - Adaptive horizontal spacing berdasarkan screen width
/// - Dynamic connector width untuk maximize label visibility
/// - Connector positioned di tengah (sejajar dengan circle)
/// - Minimal vertical & horizontal space
/// The wizard's label-slot bounds — COMPONENT policy, named ONCE. They used
/// to be spelled twice (`minLabelWidth = 35.0` in the layout builder and a
/// raw `clamp(35.0, 65.0)` under every label), so retuning one end left the
/// other behind. Deliberately NOT on the app ladder: these bound one
/// indicator's connectors, they are not an app-wide extent.
const double _minLabelWidth = 35.0;
const double _maxLabelWidth = 65.0;

class WizardProgressIndicator extends StatelessWidget {
  final int currentStep;
  final int totalSteps;
  final List<String> stepLabels;

  const WizardProgressIndicator({
    super.key,
    required this.currentStep,
    required this.totalSteps,
    required this.stepLabels,
  }) : assert(
         stepLabels.length == totalSteps,
         'stepLabels must match totalSteps',
       );

  @override
  Widget build(BuildContext context) {
    final screenWidth = MediaQuery.of(context).size.width;

    // Calculate responsive sizes - minimal padding
    final horizontalPadding = screenWidth < 360
        ? 2.0
        : (screenWidth < 400 ? 4.0 : 8.0);
    final stepSize = screenWidth < 360 ? 28.0 : 30.0;

    return Padding(
      padding: EdgeInsets.symmetric(
        horizontal: horizontalPadding,
        vertical: AppMetrics.p8,
      ),
      child: LayoutBuilder(
        builder: (context, constraints) {
          final availableWidth = constraints.maxWidth;

          // Calculate adaptive connector width based on available space
          // Reserve space for: circles + labels, then distribute remaining space to connectors
          final totalStepWidth = stepSize * totalSteps;
          final minLabelWidth = _minLabelWidth;
          final totalMinLabelWidth = minLabelWidth * totalSteps;
          final remainingSpace =
              availableWidth - totalStepWidth - totalMinLabelWidth;

          // Connector width: divide remaining space by number of connectors, with min/max limits
          final connectorWidth = (remainingSpace / (totalSteps - 1)).clamp(
            8.0,
            24.0,
          );

          // Recalculate label width with actual connector width
          final totalConnectorWidth = connectorWidth * (totalSteps - 1);
          final labelWidth =
              (availableWidth - totalConnectorWidth - totalStepWidth) /
              totalSteps;

          return Row(
            mainAxisAlignment: MainAxisAlignment.center,
            children: List.generate(totalSteps * 2 - 1, (index) {
              if (index.isOdd) {
                // Connector line
                final stepIndex = index ~/ 2;
                final isCompleted = stepIndex < currentStep;
                return _buildConnectorLine(
                  context,
                  isCompleted,
                  connectorWidth,
                  stepSize,
                );
              } else {
                // Step circle with label
                final stepIndex = index ~/ 2;
                final isActive = stepIndex == currentStep;
                final isCompleted = stepIndex < currentStep;
                return _buildStepWithLabel(
                  context,
                  stepIndex,
                  isActive,
                  isCompleted,
                  stepSize,
                  labelWidth,
                );
              }
            }),
          );
        },
      ),
    );
  }

  Widget _buildStepWithLabel(
    BuildContext context,
    int index,
    bool isActive,
    bool isCompleted,
    double stepSize,
    double labelWidth,
  ) {
    final scheme = Theme.of(context).colorScheme;
    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        // Step number circle
        Container(
          width: stepSize,
          height: stepSize,
          decoration: BoxDecoration(
            color: isActive
                ? scheme.primary
                : isCompleted
                ? context.statusColors.success
                : Colors.transparent,
            shape: BoxShape.circle,
            border: Border.all(
              color: isActive
                  ? scheme.primary
                  : isCompleted
                  ? context.statusColors.success
                  : scheme.outline,
              width: 1.5,
            ),
          ),
          child: Center(
            child: isCompleted
                ? Icon(
                    Icons.check,
                    color: scheme.onPrimary,
                    size: stepSize * 0.5,
                  )
                : Text(
                    '${index + 1}',
                    style: TextStyle(
                      color: isActive
                          ? scheme.onPrimary
                          : scheme.onSurfaceVariant,
                      fontWeight: FontWeight.bold,
                      fontSize: stepSize * 0.42,
                    ),
                  ),
          ),
        ),
        const SizedBox(height: 3),
        // Step label - compact
        SizedBox(
          width: labelWidth.clamp(_minLabelWidth, _maxLabelWidth),
          child: Text(
            stepLabels[index],
            style: context.typeRoles.labelMicro.copyWith(
              color: isActive ? scheme.onSurface : scheme.onSurfaceVariant,
              fontWeight: isActive ? FontWeight.w600 : FontWeight.normal,
              height: 1.1,
            ),
            textAlign: TextAlign.center,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
          ),
        ),
      ],
    );
  }

  Widget _buildConnectorLine(
    BuildContext context,
    bool isCompleted,
    double width,
    double stepSize,
  ) {
    final scheme = Theme.of(context).colorScheme;
    // Position connector in the middle (vertically aligned with circle center)
    final verticalOffset =
        stepSize / 2 - 0.75; // Center of circle minus half of line height

    return Container(
      width: width,
      height: 1.5,
      margin: EdgeInsets.only(
        bottom: verticalOffset + AppMetrics.p4,
      ), // +3 for label spacing
      decoration: BoxDecoration(
        color: isCompleted
            ? context.statusColors.success
            : scheme.outlineVariant,
      ),
    );
  }
}
