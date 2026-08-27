# 🧩 LABUDA Component Factory System

Sistem komponen atomik untuk mencegah file bloat dan enforce modularitas.

## 🎯 Tujuan

- **Prevent File Bloat**: Mustahil buat file >300 lines
- **Reusability**: Komponen bisa dipakai dimana-mana
- **Consistency**: API dan behavior yang konsisten
- **Maintainability**: Update 1 komponen, semua usage kebeneran

## 📦 Struktur Komponen

```
lib/shared/ui/
├── base/
│   └── base_component.dart          # Base class untuk semua komponen
├── factory/
│   └── component_factory.dart       # Factory methods untuk create komponen
├── atomic/
│   ├── input/                       # Input components
│   │   ├── title_input_component.dart
│   │   ├── description_input_component.dart
│   │   └── price_input_component.dart
│   ├── media/
│   │   └── media_upload_component.dart
│   ├── location/
│   │   └── location_picker_component.dart
│   ├── tagging/
│   │   └── user_tagging_component.dart
│   └── settings/
│       └── visibility_settings_component.dart
└── templates/
    └── creation_screen_template.dart # Template untuk creation screens
```

## 🚀 Quick Start

### 1. Setup Component Factory

```dart
// Inisialisasi factory dengan config
ComponentFactory.initialize(ComponentFactoryConfig.indonesian());
```

### 2. Build Screen dengan Components

```dart
class CreatePostScreen extends ConsumerWidget {
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return CreationScreenTemplate(
      title: 'Create Post',
      components: [
        // Tambah field = tambah 1 baris!
        ComponentFactory.titleInput(
          label: 'Post Title',
          isRequired: true,
          onChanged: (value) => handleTitleChange(value),
        ),

        ComponentFactory.descriptionInput(
          maxLines: 5,
          enableRichText: true,
          onChanged: (value) => handleDescriptionChange(value),
        ),

        ComponentFactory.mediaUpload(
          maxFiles: 10,
          onMediaChanged: (urls) => handleMediaChange(urls),
        ),
      ],
      onSubmit: (data) => handleSubmit(data),
    );
  }
}
```

### 3. Benefits

**Before (Monster File):**
```dart
// create_post_screen.dart (1520 lines)
class CreatePostScreen extends StatefulWidget {
  // 200 lines of controllers
  // 300 lines of UI building
  // 400 lines of validation logic
  // 300 lines of submit logic
  // 320 lines of media handling
  // ... unmaintainable!
}
```

**After (Component-Based):**
```dart
// create_post_screen.dart (50 lines)
class CreatePostScreen extends ConsumerWidget {
  Widget build(context, ref) {
    return CreationScreenTemplate(
      components: [
        ComponentFactory.titleInput(),
        ComponentFactory.descriptionInput(),
        ComponentFactory.mediaUpload(),
        // Add more = 1 line each!
      ],
    );
  }
}
```

## 📋 Available Components

### Input Components
```dart
// Title input dengan validation
ComponentFactory.titleInput(
  label: 'Title',
  hint: 'Enter title...',
  maxLength: 100,
  isRequired: true,
  onChanged: (value) => {},
)

// Rich text description
ComponentFactory.descriptionInput(
  maxLines: 5,
  enableRichText: true,
  onChanged: (value) => {},
)

// Price dengan currency formatting
ComponentFactory.priceInput(
  currency: 'IDR',
  minPrice: 1000,
  onChanged: (price) => {},
)
```

### Media Components
```dart
// Media upload dengan preview
ComponentFactory.mediaUpload(
  maxFiles: 10,
  allowedTypes: ['image', 'video'],
  showPreview: true,
  onMediaChanged: (urls) => {},
)
```

### Location Components
```dart
// Location picker dengan GPS
ComponentFactory.locationPicker(
  enableGPS: true,
  enableManualInput: true,
  onLocationChanged: (location) => {},
)
```

### Tagging Components
```dart
// User tagging dengan autocomplete
ComponentFactory.userTagging(
  maxTags: 10,
  onTagsChanged: (users) => {},
)
```

### Settings Components
```dart
// Visibility settings
ComponentFactory.visibilitySettings(
  availableOptions: ['Public', 'Private', 'Friends'],
  onVisibilityChanged: (visibility) => {},
)
```

## 🎨 Customization

### Custom Component Wrapper
```dart
ComponentFactory.custom(
  child: MyCustomWidget(),
  isRequired: true,
  errorMessage: 'Error occurred',
  isLoading: false,
)
```

### Spacing & Layout
```dart
ComponentFactory.spacing(ComponentSpacing.lg),
ComponentFactory.sectionHeader(
  title: 'Media Settings',
  subtitle: 'Upload photos and videos',
  isRequired: true,
)
```

## 🔧 Adding New Components

### 1. Create Atomic Component
```dart
// lib/shared/ui/atomic/new/my_component.dart
class MyComponent extends BaseComponent {
  @override
  Widget buildContent(BuildContext context) {
    return MyCustomWidget();
  }
}
```

### 2. Add to Factory
```dart
// lib/shared/ui/factory/component_factory.dart
static Widget myComponent({
  // parameters
}) {
  return MyComponent(
    // configuration
  );
}
```

### 3. Use in Screens
```dart
// Screens langsung bisa pakai
ComponentFactory.myComponent(
  // parameters
)
```

## ✅ Guidelines

### Component Rules
- **Max 100 lines per component**
- **Single responsibility**
- **No business logic**
- **Implement required interfaces**

### Screen Rules
- **Max 150 lines per screen**
- **Use CreationScreenTemplate**
- **Compose dengan components**
- **No custom UI building**

### Adding Fields Rules
- **Add component, not code**
- **1 new field = 1 new line**
- **Reuse existing components**
- **Never modify existing components**

## 📊 Impact

### File Size Reduction
- **Before**: 1520+ lines per screen
- **After**: 50-100 lines per screen
- **Reduction**: 90%+ size reduction

### Development Speed
- **Add field**: 1 line vs 50+ lines
- **Modify field**: Change factory call vs hunt through 1000+ lines
- **Bug fix**: Fix component once vs fix every screen

### Maintainability
- **Component testing**: Test once, confident everywhere
- **Design changes**: Update component, all screens updated
- **New developers**: Understand components vs understand monoliths

## 🚨 Anti-Patterns

### ❌ DON'T: Custom UI in Screens
```dart
// SALAH - jangan build UI custom di screen
Widget build(context) {
  return Column(
    children: [
      TextFormField(...), // 50+ lines
      CustomWidget(...),  // 100+ lines
      // File jadi besar lagi!
    ],
  );
}
```

### ✅ DO: Use Component Factory
```dart
// BENAR - pakai component factory
Widget build(context) {
  return CreationScreenTemplate(
    components: [
      ComponentFactory.titleInput(),    // 1 line
      ComponentFactory.mediaUpload(),   // 1 line
      ComponentFactory.customField(),   // 1 line
    ],
  );
}
```

### ❌ DON'T: Modify Existing Components
```dart
// SALAH - jangan modify existing component
class TitleInputComponent {
  Widget build() {
    return Column([
      TextFormField(),
      NewFeature(), // JANGAN TAMBAH DISINI!
    ]);
  }
}
```

### ✅ DO: Create New Component
```dart
// BENAR - buat component baru
class EnhancedTitleInputComponent extends BaseComponent {
  Widget buildContent() {
    return Column([
      TitleInputComponent(),
      NewFeature(), // Component baru
    ]);
  }
}
```

---

**Result**: File size under control, development speed 10x faster, maintenance jadi mudah! 🎉