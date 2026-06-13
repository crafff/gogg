# Image Assets Directory Structure

This directory contains all static image assets used in the application.

## Directory Structure

```
images/
├── heroes/              # Champion/hero icons
├── items/               # Equipment/item icons
├── common/              # Common UI icons (search, filter, etc.)
├── ui/                  # UI-related images
└── backgrounds/         # Background images
```

## Adding Assets

1. **Heroes**: Place champion images in `heroes/` folder
   - Naming convention: `{championName}.png` (e.g., `ahri.png`)

2. **Items**: Place equipment/item images in `items/` folder
   - Naming convention: `{itemName}.png` (e.g., `trinity-force.png`)

3. **Common Icons**: UI icons in `common/` folder
   - e.g., search.svg, filter.svg, etc.

4. **Backgrounds**: Large background images in `backgrounds/`

## Import Usage

```typescript
// In TypeScript/React components:
import heroImage from '@assets/images/heroes/ahri.png';

// In CSS:
background-image: url('@assets/images/heroes/ahri.png');
```

## Optimization Tips

- Use WebP format for better compression when possible
- Optimize SVGs with tools like SVGO
- Compress PNG/JPG with tools like TinyPNG
- Consider using CSS sprites for multiple small icons
