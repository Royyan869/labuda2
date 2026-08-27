/// Village Search Dropdown Widget
///
/// Searchable dropdown untuk desa/kelurahan dengan autocomplete functionality
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';

class VillageSearchDropdown extends ConsumerStatefulWidget {
  final Village? selectedVillage;
  final District? selectedDistrict;
  final ValueChanged<Village?> onChanged;
  final String? labelText;
  final String? hintText;
  final IconData? prefixIcon;
  final String? Function(Village?)? validator;

  const VillageSearchDropdown({
    super.key,
    required this.selectedVillage,
    required this.selectedDistrict,
    required this.onChanged,
    this.labelText,
    this.hintText,
    this.prefixIcon,
    this.validator,
  });

  @override
  ConsumerState<VillageSearchDropdown> createState() =>
      _VillageSearchDropdownState();
}

class _VillageSearchDropdownState extends ConsumerState<VillageSearchDropdown> {
  final TextEditingController _searchController = TextEditingController();
  bool _isDropdownOpen = false;
  List<Village> _filteredVillages = [];

  @override
  void initState() {
    super.initState();
    if (widget.selectedVillage != null) {
      _searchController.text = widget.selectedVillage!.name;
    }
  }

  @override
  void didUpdateWidget(VillageSearchDropdown oldWidget) {
    super.didUpdateWidget(oldWidget);

    // Reset ketika district berubah
    if (oldWidget.selectedDistrict?.id != widget.selectedDistrict?.id) {
      _searchController.clear();
      setState(() {
        _isDropdownOpen = false;
        _filteredVillages = [];
      });
      widget.onChanged(null);
    }

    // Update text ketika selectedVillage berubah dari luar
    if (oldWidget.selectedVillage?.id != widget.selectedVillage?.id) {
      _searchController.text = widget.selectedVillage?.name ?? '';
    }
  }

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final villagesAsync = ref.watch(
      villagesProvider(widget.selectedDistrict?.id),
    );

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (widget.labelText != null) ...[
          Text(
            widget.labelText!,
            style: TextStyle(
              fontSize: 14,
              fontWeight: FontWeight.w500,
              color: isDark
                  ? AppColors.neutralGray300
                  : AppColors.neutralGray700,
            ),
          ),
          const SizedBox(height: 8),
        ],
        widget.selectedDistrict == null
            ? _buildDisabledField(context, isDark)
            : villagesAsync.when(
                data: (villages) =>
                    _buildSearchableDropdown(context, isDark, villages),
                loading: () => _buildLoadingField(context, isDark),
                error: (error, stack) => _buildErrorField(context, isDark),
              ),
      ],
    );
  }

  Widget _buildDisabledField(BuildContext context, bool isDark) {
    return Container(
      height: 50,
      padding: const EdgeInsets.symmetric(horizontal: 16),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: isDark ? AppColors.darkGray600 : AppColors.neutralGray300,
        ),
        color: isDark ? AppColors.darkGray800 : AppColors.neutralGray100,
      ),
      child: Row(
        children: [
          if (widget.prefixIcon != null) ...[
            Icon(
              widget.prefixIcon,
              color: isDark
                  ? AppColors.neutralGray500
                  : AppColors.neutralGray400,
            ),
            const SizedBox(width: 12),
          ],
          Text(
            'Pilih kecamatan dulu',
            style: TextStyle(
              color: isDark
                  ? AppColors.neutralGray500
                  : AppColors.neutralGray400,
              fontSize: 16,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildLoadingField(BuildContext context, bool isDark) {
    return Container(
      height: 50,
      padding: const EdgeInsets.symmetric(horizontal: 16),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: isDark ? AppColors.darkGray600 : AppColors.neutralGray300,
        ),
        color: isDark ? AppColors.darkGray700 : AppColors.neutralWhite,
      ),
      child: Row(
        children: [
          if (widget.prefixIcon != null) ...[
            Icon(
              widget.prefixIcon,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray600,
            ),
            const SizedBox(width: 12),
          ],
          const SizedBox(
            width: 20,
            height: 20,
            child: CircularProgressIndicator(strokeWidth: 2),
          ),
          const SizedBox(width: 12),
          Text(
            'Loading desa...',
            style: TextStyle(
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray500,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildErrorField(BuildContext context, bool isDark) {
    return Container(
      height: 50,
      padding: const EdgeInsets.symmetric(horizontal: 16),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: AppColors.statusError),
        color: isDark ? AppColors.darkGray700 : AppColors.neutralWhite,
      ),
      child: Row(
        children: [
          Icon(Icons.error_outline, color: AppColors.statusError, size: 20),
          const SizedBox(width: 8),
          Text(
            'Error loading desa',
            style: TextStyle(color: AppColors.statusError),
          ),
        ],
      ),
    );
  }

  Widget _buildSearchableDropdown(
    BuildContext context,
    bool isDark,
    List<Village> villages,
  ) {
    return GestureDetector(
      onTap: () {
        // Close dropdown ketika tap di luar
        if (_isDropdownOpen) {
          setState(() {
            _isDropdownOpen = false;
          });
        }
      },
      child: Column(
        children: [
          TextFormField(
            controller: _searchController,
            validator: (value) =>
                widget.validator?.call(widget.selectedVillage),
            decoration: InputDecoration(
              hintText: widget.hintText ?? 'Cari atau pilih desa/kelurahan',
              prefixIcon: widget.prefixIcon != null
                  ? Icon(
                      widget.prefixIcon,
                      color: isDark
                          ? AppColors.neutralGray400
                          : AppColors.neutralGray600,
                    )
                  : null,
              suffixIcon: GestureDetector(
                onTap: () {
                  setState(() {
                    if (_isDropdownOpen) {
                      _isDropdownOpen = false;
                    } else {
                      _filteredVillages = villages;
                      _isDropdownOpen = true;
                    }
                  });
                },
                child: Icon(
                  _isDropdownOpen
                      ? Icons.keyboard_arrow_up
                      : Icons.keyboard_arrow_down,
                  color: isDark
                      ? AppColors.neutralGray400
                      : AppColors.neutralGray600,
                ),
              ),
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
                borderSide: BorderSide(color: AppColors.primaryRed),
              ),
              filled: true,
              fillColor: isDark
                  ? AppColors.darkGray700
                  : AppColors.neutralWhite,
              contentPadding: const EdgeInsets.symmetric(
                horizontal: 16,
                vertical: 14,
              ),
            ),
            style: TextStyle(
              color: isDark
                  ? AppColors.neutralGray200
                  : AppColors.neutralGray900,
              fontSize: 16,
            ),
            onChanged: (query) {
              setState(() {
                _filteredVillages = villages
                    .where(
                      (village) => village.name.toLowerCase().contains(
                        query.toLowerCase(),
                      ),
                    )
                    .toList();
                _isDropdownOpen =
                    query.isNotEmpty || _filteredVillages.isNotEmpty;
              });

              // Reset selected village jika text berubah tapi tidak match dengan selected
              if (widget.selectedVillage != null &&
                  widget.selectedVillage!.name != query) {
                widget.onChanged(null);
              }
            },
            onTap: () {
              setState(() {
                _filteredVillages = villages;
                _isDropdownOpen = true;
              });
            },
          ),
          if (_isDropdownOpen && _filteredVillages.isNotEmpty) ...[
            const SizedBox(height: 4),
            Container(
              constraints: const BoxConstraints(maxHeight: 200),
              decoration: BoxDecoration(
                borderRadius: BorderRadius.circular(12),
                border: Border.all(
                  color: isDark
                      ? AppColors.darkGray600
                      : AppColors.neutralGray300,
                ),
                color: isDark ? AppColors.darkGray700 : AppColors.neutralWhite,
                boxShadow: [
                  BoxShadow(
                    color: isDark ? Colors.black26 : Colors.black12,
                    blurRadius: 8,
                    offset: const Offset(0, 2),
                  ),
                ],
              ),
              child: ListView.builder(
                shrinkWrap: true,
                itemCount: _filteredVillages.length,
                itemBuilder: (context, index) {
                  final village = _filteredVillages[index];
                  final isSelected = widget.selectedVillage?.id == village.id;

                  return Container(
                    decoration: BoxDecoration(
                      color: isSelected
                          ? (isDark
                                ? AppColors.primaryRed.withValues(alpha: 0.2)
                                : AppColors.primaryRed.withValues(alpha: 0.1))
                          : Colors.transparent,
                    ),
                    child: ListTile(
                      title: Text(
                        village.name,
                        style: TextStyle(
                          color: isSelected
                              ? AppColors.primaryRed
                              : (isDark
                                    ? AppColors.neutralGray200
                                    : AppColors.neutralGray900),
                          fontSize: 14,
                          fontWeight: isSelected
                              ? FontWeight.w600
                              : FontWeight.normal,
                        ),
                      ),
                      onTap: () {
                        widget.onChanged(village);
                        _searchController.text = village.name;
                        setState(() {
                          _isDropdownOpen = false;
                        });
                      },
                    ),
                  );
                },
              ),
            ),
          ],
          if (_isDropdownOpen &&
              _filteredVillages.isEmpty &&
              _searchController.text.isNotEmpty) ...[
            const SizedBox(height: 4),
            Container(
              height: 50,
              padding: const EdgeInsets.symmetric(horizontal: 16),
              decoration: BoxDecoration(
                borderRadius: BorderRadius.circular(12),
                border: Border.all(
                  color: isDark
                      ? AppColors.darkGray600
                      : AppColors.neutralGray300,
                ),
                color: isDark ? AppColors.darkGray700 : AppColors.neutralWhite,
              ),
              child: Row(
                children: [
                  Icon(
                    Icons.search_off,
                    color: isDark
                        ? AppColors.neutralGray400
                        : AppColors.neutralGray500,
                    size: 20,
                  ),
                  const SizedBox(width: 8),
                  Text(
                    'Tidak ditemukan hasil pencarian',
                    style: TextStyle(
                      color: isDark
                          ? AppColors.neutralGray400
                          : AppColors.neutralGray500,
                      fontSize: 14,
                    ),
                  ),
                ],
              ),
            ),
          ],
        ],
      ),
    );
  }
}
