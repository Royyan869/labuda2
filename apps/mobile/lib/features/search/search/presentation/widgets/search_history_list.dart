import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/features/search/search/domain/entities/search_history.dart';

/// Widget to display search history
class SearchHistoryList extends StatelessWidget {
  final List<SearchHistory> history;
  final ValueChanged<String> onHistoryTap;
  final ValueChanged<String> onDeleteTap;
  final VoidCallback onClearAll;

  const SearchHistoryList({
    super.key,
    required this.history,
    required this.onHistoryTap,
    required this.onDeleteTap,
    required this.onClearAll,
  });

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    if (history.isEmpty) {
      return const SizedBox.shrink();
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
          child: Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text(
                'Recent Searches',
                style: TextStyle(
                  fontSize: 16,
                  fontWeight: FontWeight.w600,
                  color: isDark
                      ? AppColors.neutralGray100
                      : AppColors.neutralGray900,
                ),
              ),
              TextButton(
                onPressed: onClearAll,
                child: Text(
                  'Clear All',
                  style: TextStyle(color: AppColors.primary, fontSize: 14),
                ),
              ),
            ],
          ),
        ),
        ListView.separated(
          shrinkWrap: true,
          physics: const NeverScrollableScrollPhysics(),
          itemCount: history.length,
          separatorBuilder: (_, _) => Divider(
            height: 1,
            color: isDark ? AppColors.darkGray600 : AppColors.neutralGray200,
          ),
          itemBuilder: (context, index) {
            final item = history[index];
            return ListTile(
              leading: Icon(
                Icons.history,
                color: isDark
                    ? AppColors.neutralGray500
                    : AppColors.neutralGray400,
                size: 20,
              ),
              title: Text(
                item.query,
                style: TextStyle(
                  color: isDark
                      ? AppColors.neutralGray100
                      : AppColors.neutralGray900,
                ),
              ),
              trailing: IconButton(
                icon: Icon(
                  Icons.close,
                  size: 18,
                  color: isDark
                      ? AppColors.neutralGray500
                      : AppColors.neutralGray400,
                ),
                onPressed: () => onDeleteTap(item.id),
              ),
              onTap: () => onHistoryTap(item.query),
            );
          },
        ),
      ],
    );
  }
}
