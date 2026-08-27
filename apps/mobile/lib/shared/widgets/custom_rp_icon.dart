import 'package:labuda/core/src/theme/app_colors.dart';
import 'package:flutter/material.dart';

/// Custom Rupiah (Rp) icon widget yang menyerupai Material Icons
/// Bisa digunakan sebagai replacement untuk Icons.attach_money
class CustomRpIcon extends StatelessWidget {
  const CustomRpIcon({super.key, this.size = 24.0, this.color});

  final double size;
  final Color? color;

  @override
  Widget build(BuildContext context) {
    final iconColor = color ?? IconTheme.of(context).color ?? AppColors.dark;

    return SizedBox(
      width: size,
      height: size,
      child: CustomPaint(painter: _RpIconPainter(color: iconColor)),
    );
  }
}

class _RpIconPainter extends CustomPainter {
  final Color color;

  _RpIconPainter({required this.color});

  @override
  void paint(Canvas canvas, Size size) {
    final strokePaint = Paint()
      ..color = color
      ..style = PaintingStyle.stroke
      ..strokeWidth = size.width * 0.08
      ..strokeCap = StrokeCap.round;

    final centerX = size.width / 2;
    final centerY = size.height / 2;
    final radius = size.width * 0.4;

    // Draw circle background
    canvas.drawCircle(Offset(centerX, centerY), radius, strokePaint);

    // Draw "Rp" text
    final textPainter = TextPainter(
      text: TextSpan(
        text: 'Rp',
        style: TextStyle(
          color: color,
          fontSize: size.width * 0.32,
          fontWeight: FontWeight.bold,
          fontFamily: 'Roboto',
        ),
      ),
      textDirection: TextDirection.ltr,
    );

    textPainter.layout();

    final textOffset = Offset(
      centerX - textPainter.width / 2,
      centerY - textPainter.height / 2,
    );

    textPainter.paint(canvas, textOffset);
  }

  @override
  bool shouldRepaint(covariant CustomPainter oldDelegate) {
    return oldDelegate is! _RpIconPainter || oldDelegate.color != color;
  }
}

/// Widget helper untuk membuat Rp icon dengan style yang konsisten
class RpIcon extends StatelessWidget {
  const RpIcon({super.key, this.size = 24.0, this.color});

  final double size;
  final Color? color;

  @override
  Widget build(BuildContext context) {
    return CustomRpIcon(size: size, color: color);
  }

  /// Factory constructor untuk membuat icon berukuran kecil (16px)
  factory RpIcon.small({Color? color}) {
    return RpIcon(size: 16.0, color: color);
  }

  /// Factory constructor untuk membuat icon berukuran normal (24px)
  factory RpIcon.normal({Color? color}) {
    return RpIcon(size: 24.0, color: color);
  }

  /// Factory constructor untuk membuat icon berukuran besar (32px)
  factory RpIcon.large({Color? color}) {
    return RpIcon(size: 32.0, color: color);
  }
}
