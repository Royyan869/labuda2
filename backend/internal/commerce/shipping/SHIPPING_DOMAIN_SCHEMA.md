# Shipping Domain Schema Reference

## Overview

The shipping domain owns seller shipping options, geographic coverage, and city-level overrides.

## Tables

### shipping_options
Seller-configured shipping methods with generic transport types.

### shipping_coverages
Province-level coverage rows for each shipping option.

### shipping_city_overrides
City-specific pricing and availability overrides.

### product_shipping_options
Links products to the shipping options they can use.

## Foreign Keys

| Table | Column | References | On Delete |
|---|---|---|---|
| shipping_options | seller_id | users(id) | RESTRICT |
| shipping_coverages | shipping_option_id | shipping_options(id) | CASCADE |
| shipping_city_overrides | shipping_coverage_id | shipping_coverages(id) | CASCADE |
| product_shipping_options | product_id | products(id) | CASCADE |
| product_shipping_options | shipping_option_id | shipping_options(id) | CASCADE |

## Indexes

### shipping_options
| Index Name | Columns | Type | Condition |
|---|---|---|---|
| idx_shipping_options_seller_id | seller_id | B-tree | - |
| idx_shipping_options_seller_active | seller_id, is_active | B-tree | is_active = TRUE |
| idx_shipping_options_transport_type | transport_type | B-tree | - |
| idx_shipping_options_seller_name | seller_id, name | UNIQUE | is_active = TRUE |

### shipping_coverages
| Index Name | Columns | Type | Condition |
|---|---|---|---|
| idx_shipping_coverages_shipping_option_id | shipping_option_id | B-tree | - |
| idx_shipping_coverages_province_code | province_code | B-tree | - |
| idx_shipping_coverages_available | shipping_option_id, is_available | B-tree | is_available = TRUE |

### shipping_city_overrides
| Index Name | Columns | Type | Condition |
|---|---|---|---|
| idx_shipping_city_overrides_coverage_id | shipping_coverage_id | B-tree | - |
| idx_shipping_city_overrides_city_code | city_code | B-tree | - |
| idx_shipping_city_overrides_available | shipping_coverage_id, is_available | B-tree | is_available = TRUE |

### product_shipping_options
| Index Name | Columns | Type | Condition |
|---|---|---|---|
| idx_product_shipping_options_product_id | product_id | B-tree | - |
| idx_product_shipping_options_shipping_option_id | shipping_option_id | B-tree | - |
| idx_product_shipping_options_product_sort | product_id, sort_order | B-tree | is_available = TRUE |

## Constraints

### UNIQUE

| Table | Columns |
|---|---|
| shipping_coverages | (shipping_option_id, province_code) |
| shipping_city_overrides | (shipping_coverage_id, city_code) |
| product_shipping_options | (product_id, shipping_option_id) |

## Note

The active runtime contract is product-scoped through `product_shipping_options`.
