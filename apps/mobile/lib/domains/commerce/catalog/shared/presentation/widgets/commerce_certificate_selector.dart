import 'package:flutter/material.dart';

const List<CommerceCertificateOption> commerceCertificateOptions = [
  CommerceCertificateOption('breeder', 'Breeder'),
  CommerceCertificateOption('contest', 'Kontes'),
  CommerceCertificateOption('ownership', 'Kepemilikan'),
  CommerceCertificateOption('health', 'Kesehatan'),
];

class CommerceCertificateSelector extends StatelessWidget {
  final List<String> selectedCertificates;
  final ValueChanged<List<String>> onChanged;
  final String? helperText;

  const CommerceCertificateSelector({
    super.key,
    required this.selectedCertificates,
    required this.onChanged,
    this.helperText,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final selected = selectedCertificates.toSet();

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'Sertifikat',
          style: theme.textTheme.titleSmall?.copyWith(
            fontWeight: FontWeight.w700,
          ),
        ),
        if (helperText != null) ...[
          const SizedBox(height: 4),
          Text(
            helperText!,
            style: theme.textTheme.bodySmall?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
            ),
          ),
        ],
        const SizedBox(height: 4),
        Text(
          'Berdasarkan pernyataan seller',
          style: theme.textTheme.bodySmall?.copyWith(
            color: theme.colorScheme.onSurfaceVariant,
            fontStyle: FontStyle.italic,
          ),
        ),
        const SizedBox(height: 12),
        Wrap(
          spacing: 8,
          runSpacing: 8,
          children: commerceCertificateOptions
              .map((option) {
                final isSelected = selected.contains(option.value);
                return FilterChip(
                  label: Text(option.label),
                  selected: isSelected,
                  onSelected: (isSelected) {
                    final next = <String>{...selected};
                    if (isSelected) {
                      next.add(option.value);
                    } else {
                      next.remove(option.value);
                    }
                    onChanged(next.toList(growable: false));
                  },
                );
              })
              .toList(growable: false),
        ),
      ],
    );
  }
}

class CommerceCertificateOption {
  final String value;
  final String label;

  const CommerceCertificateOption(this.value, this.label);
}
