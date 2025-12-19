# Kill-Snap Project Context

## Deployment

**NEVER use `sam deploy` directly. ALWAYS use the GitHub Actions pipeline for deployments.**

Push changes to the repository and let the CI/CD pipeline handle deployment.

## UI Terminology

- **Thumbnail Images**: The images displayed in the grid on the Image Review page (ImageGallery component)
- **Review Image**: The larger image shown in the popup modal (ImageModal component) when clicking a thumbnail

## Component Mapping

| Term | Component | Description |
|------|-----------|-------------|
| Image Review Page | `ImageGallery.tsx` | Main page with grid of thumbnail images |
| Thumbnail Images | Grid items in `ImageGallery.tsx` | Small preview images in the gallery grid |
| Review Image | `ImageModal.tsx` | Large image popup for detailed review and actions |
