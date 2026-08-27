import 'package:google_mlkit_text_recognition/google_mlkit_text_recognition.dart';

/// KTP OCR Result
class KTPOcrResult {
  final String? nik;
  final String? name;
  final bool success;
  final String? errorMessage;

  KTPOcrResult({this.nik, this.name, this.success = false, this.errorMessage});

  factory KTPOcrResult.success({String? nik, String? name}) {
    return KTPOcrResult(nik: nik, name: name, success: true);
  }

  factory KTPOcrResult.error(String message) {
    return KTPOcrResult(success: false, errorMessage: message);
  }
}

/// Service untuk OCR (Optical Character Recognition) dari foto KTP
///
/// Menggunakan Google ML Kit Text Recognition untuk membaca:
/// - NIK (16 digit)
/// - Nama Lengkap
class KtpOcrService {
  static final _textRecognizer = TextRecognizer(
    script: TextRecognitionScript.latin,
  );

  /// Extract NIK and Name from KTP image
  static Future<KTPOcrResult> extractKTPData(String imagePath) async {
    try {
      final inputImage = InputImage.fromFilePath(imagePath);
      final recognizedText = await _textRecognizer.processImage(inputImage);

      String? nik;
      String? name;

      // Process each text block
      for (var block in recognizedText.blocks) {
        for (var line in block.lines) {
          final text = line.text.trim();

          // Extract NIK (16 digits)
          nik ??= _extractNIK(text);

          // Extract Name (after "Nama" label)
          name ??= _extractName(text, recognizedText.blocks);
        }
      }

      return KTPOcrResult.success(nik: nik, name: name);
    } catch (e) {
      return KTPOcrResult.error('Failed to read KTP: $e');
    }
  }

  /// Extract NIK (16 consecutive digits)
  static String? _extractNIK(String text) {
    // Remove spaces and special characters
    final cleanText = text.replaceAll(RegExp(r'[\s\-\.]'), '');

    // Look for 16 consecutive digits
    final nikPattern = RegExp(r'\b\d{16}\b');
    final match = nikPattern.firstMatch(cleanText);

    if (match != null) {
      return match.group(0);
    }

    // Alternative: Look for pattern like "NIK: 1234567890123456" or "NIK 1234567890123456"
    final nikLabelPattern = RegExp(r'NIK[:\s]*(\d{16})');
    final labelMatch = nikLabelPattern.firstMatch(text);

    if (labelMatch != null) {
      return labelMatch.group(1);
    }

    return null;
  }

  /// Extract Name from KTP
  /// Typically appears after "Nama" label
  static String? _extractName(String text, List<TextBlock> allBlocks) {
    // Look for "Nama" or "Name" label
    if (text.toUpperCase().contains('NAMA') ||
        text.toUpperCase().contains('NAME')) {
      // Try to extract name from same line
      final namePattern = RegExp(r'NAMA[:\s]*(.+)', caseSensitive: false);
      final match = namePattern.firstMatch(text);

      if (match != null && match.group(1) != null) {
        final extractedName = match.group(1)!.trim();
        // Make sure it's not just "Nama" or empty
        if (extractedName.isNotEmpty &&
            !extractedName.toUpperCase().contains('NAMA') &&
            extractedName.length > 3) {
          return _cleanName(extractedName);
        }
      }

      // If not found in same line, look in subsequent blocks
      final currentBlockIndex = allBlocks.indexWhere((block) {
        return block.lines.any((line) => line.text.contains(text));
      });

      if (currentBlockIndex != -1 && currentBlockIndex + 1 < allBlocks.length) {
        final nextBlock = allBlocks[currentBlockIndex + 1];
        if (nextBlock.lines.isNotEmpty) {
          final potentialName = nextBlock.lines.first.text.trim();
          // Validate it looks like a name (letters and spaces, min 3 chars)
          if (potentialName.length > 3 &&
              RegExp(
                r'^[A-Z\s]+$',
                caseSensitive: false,
              ).hasMatch(potentialName)) {
            return _cleanName(potentialName);
          }
        }
      }
    }

    return null;
  }

  /// Clean and format name
  static String _cleanName(String name) {
    // Remove extra spaces and trim
    name = name.replaceAll(RegExp(r'\s+'), ' ').trim();

    // Convert to title case
    return name
        .split(' ')
        .map(
          (word) => word.isEmpty
              ? ''
              : word[0].toUpperCase() + word.substring(1).toLowerCase(),
        )
        .join(' ');
  }

  /// Dispose text recognizer
  static Future<void> dispose() async {
    await _textRecognizer.close();
  }
}
