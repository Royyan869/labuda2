import 'package:equatable/equatable.dart';

/// Order Item - individual item dalam order
class OrderItem extends Equatable {
  final String id;
  final String productId;
  final String listingName;
  final String listingImage;
  final double price;
  final int quantity;
  final String? variety;
  final String? size;
  final String? gender;
  final String? notes;

  const OrderItem({
    required this.id,
    required this.productId,
    required this.listingName,
    required this.listingImage,
    required this.price,
    this.quantity = 1,
    this.variety,
    this.size,
    this.gender,
    this.notes,
  });

  double get subtotal => price * quantity;

  @override
  List<Object?> get props => [
    id,
    productId,
    listingName,
    listingImage,
    price,
    quantity,
    variety,
    size,
    gender,
    notes,
  ];
}
