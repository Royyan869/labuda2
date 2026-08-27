# Labuda Mobile App

Flutter-based super app untuk komunitas Koi Indonesia dengan social commerce platform.

## 🚀 Quick Start

### Prerequisites
- Flutter SDK: 3.9.2 or higher
- Dart SDK: 3.9.2 or higher
- Android Studio / Xcode (for platform-specific builds)

### Setup

1. **Install dependencies**:
   ```bash
   cd apps/mobile
   flutter pub get
   ```

2. **Configure environment variables**:
   ```bash
   cp .env.example .env
   # Edit .env with client-safe values only.
   # Never add a Midtrans server key to the mobile env.
   ```

3. **Run the app**:
   ```bash
   # Android
   flutter run

   # iOS (macOS only)
   flutter run -d ios

   # Web
   flutter run -d chrome

   # Specific device
   flutter devices  # List available devices
   flutter run -d <device-id>
   ```

## 🏗️ Build

### Android
```bash
# Debug APK
flutter build apk --debug

# Release APK
flutter build apk --release

# App Bundle (for Play Store)
flutter build appbundle --release
```

### iOS
```bash
# Debug build (no code signing)
flutter build ios --debug --no-codesign

# Release build
flutter build ios --release
```

### Web
```bash
flutter build web --release
```

## 🧪 Testing

```bash
# Run all tests
flutter test

# Run with coverage
flutter test --coverage

# Run specific test file
flutter test test/features/auth/auth_test.dart
```

## 🛠️ Development

### Code Generation
```bash
# Generate Riverpod code
dart run build_runner build --delete-conflicting-outputs

# Watch mode (auto-generate on file changes)
dart run build_runner watch
```

### Code Analysis
```bash
# Analyze code
flutter analyze

# Format code
dart format lib/ test/
```

### Clean Build
```bash
flutter clean
flutter pub get
flutter run
```

## 📁 Project Structure

```
apps/mobile/
├── lib/                    # Dart source code
│   ├── core/               # Core utilities & services
│   ├── features/           # Feature modules
│   ├── app.dart            # Main app widget
│   └── main.dart           # Entry point
├── test/                   # Unit & widget tests
├── android/                # Android platform code
├── ios/                    # iOS platform code
├── web/                    # Web platform code
├── assets/                 # Images, fonts, data files
├── pubspec.yaml            # Dependencies
└── .env                    # Environment variables
```

## 🔧 Configuration

### Firebase Setup

#### Android
1. Download `google-services.json` from Firebase Console
2. Place it in `android/app/google-services.json`

#### iOS
1. Download `GoogleService-Info.plist` from Firebase Console
2. Place it in `ios/Runner/GoogleService-Info.plist`

### Environment Variables

Edit `.env` file:
```env
GOOGLE_MAPS_API_KEY=your_api_key_here
MIDTRANS_MERCHANT_ID=your_midtrans_merchant_id_here
MIDTRANS_CLIENT_KEY=your_midtrans_client_key_here
# Never put a Midtrans server key in the mobile app or mobile env files.
```

### Backend Base URL (dev)

Dev defaults are platform-aware: the Android emulator reaches the host via
`10.0.2.2:8080`, everything else uses `localhost:8080`. For a physical device
or a backend on another host, pass the override at run/build time (no source
edit):

```bash
flutter run --dart-define=API_BASE_URL=http://192.168.1.50:8080/api/v1
# Optional matching WebSocket override:
flutter run --dart-define=API_BASE_URL=http://192.168.1.50:8080/api/v1 \
            --dart-define=API_WS_URL=ws://192.168.1.50:8080/api/v1/ws
```

## 🐛 Troubleshooting

### Build Issues

**Problem**: Gradle build fails
```bash
cd android
./gradlew clean
cd ..
flutter clean
flutter pub get
```

**Problem**: CocoaPods issues (iOS)
```bash
cd ios
pod deintegrate
pod install
cd ..
flutter clean
flutter pub get
```

**Problem**: Assets not loading
```bash
flutter clean
flutter pub get
# Verify assets paths in pubspec.yaml
```

### Hot Reload Not Working
1. Stop the app
2. Run `flutter clean`
3. Restart the app with `flutter run`

## 📱 Supported Platforms

- ✅ Android (5.0+)
- ✅ iOS (12.0+)
- ✅ Web
- ⚠️ Windows (experimental)
- ⚠️ macOS (experimental)
- ⚠️ Linux (experimental)

## 📄 License

Copyright © 2024 Labuda. All rights reserved.
