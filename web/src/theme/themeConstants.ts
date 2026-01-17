// Theme colors and styles for the application

export interface ThemeColor {
  id: string;
  name: string;
  primary: string;
  secondary: string;
  background: string;
  sidebar: string;
}

export interface UIStyle {
  id: string;
  name: string;
  borderRadius: string;
  buttonStyle: string;
  shadowIntensity: string;
  fontWeight: string;
  spacing: string;
  accentStyle: string;
}

export const THEME_COLORS: ThemeColor[] = [
  {
    id: "ocean-blue",
    name: "Ocean Blue",
    primary: "#3498db",
    secondary: "#2980b9",
    background: "#0f0f1a",
    sidebar: "#1a1a2e"
  },
  {
    id: "forest-green",
    name: "Forest Green",
    primary: "#27ae60",
    secondary: "#229954",
    background: "#0a1a10",
    sidebar: "#0d2616"
  },
  {
    id: "sunset-orange",
    name: "Sunset Orange",
    primary: "#e67e22",
    secondary: "#d35400",
    background: "#1a120a",
    sidebar: "#2e1c0d"
  },
  {
    id: "royal-purple",
    name: "Royal Purple",
    primary: "#9b59b6",
    secondary: "#8e44ad",
    background: "#120a1a",
    sidebar: "#1e0d2e"
  },
  {
    id: "crimson-red",
    name: "Crimson Red",
    primary: "#e74c3c",
    secondary: "#c0392b",
    background: "#1a0a0a",
    sidebar: "#2e0d0d"
  },
  {
    id: "midnight-dark",
    name: "Midnight Dark",
    primary: "#ecf0f1",
    secondary: "#bdc3c7",
    background: "#0d0d0d",
    sidebar: "#1a1a1a"
  },
  {
    id: "neon-cyan",
    name: "Neon Cyan",
    primary: "#00d9ff",
    secondary: "#00b8cc",
    background: "#0a0e1a",
    sidebar: "#050810"
  },
  {
    id: "neon-pink",
    name: "Neon Pink",
    primary: "#ff006e",
    secondary: "#d60051",
    background: "#0f0a15",
    sidebar: "#1a0a1f"
  },
  {
    id: "sage-green",
    name: "Sage Green",
    primary: "#6b8e23",
    secondary: "#556b2f",
    background: "#0f1510",
    sidebar: "#1a251c"
  },
  {
    id: "slate-grey",
    name: "Slate Grey",
    primary: "#708090",
    secondary: "#556b7d",
    background: "#0f1015",
    sidebar: "#1a1c22"
  },
  {
    id: "teal-turquoise",
    name: "Teal Turquoise",
    primary: "#16a085",
    secondary: "#138d75",
    background: "#0a1515",
    sidebar: "#0d2525"
  },
  {
    id: "coral-pink",
    name: "Coral Pink",
    primary: "#ff6b6b",
    secondary: "#ee5a52",
    background: "#1a0f0f",
    sidebar: "#2e1a1a"
  },
  {
    id: "lavender-dream",
    name: "Lavender Dream",
    primary: "#b19cd9",
    secondary: "#967bb6",
    background: "#10101a",
    sidebar: "#1a1a2e"
  },
  {
    id: "golden-hour",
    name: "Golden Hour",
    primary: "#f39c12",
    secondary: "#d68910",
    background: "#1a150a",
    sidebar: "#2e250d"
  },
  {
    id: "mint-fresh",
    name: "Mint Fresh",
    primary: "#1abc9c",
    secondary: "#16a085",
    background: "#0a1a18",
    sidebar: "#0d2e28"
  },
  {
    id: "deep-indigo",
    name: "Deep Indigo",
    primary: "#4b0082",
    secondary: "#3d006b",
    background: "#0a0a1a",
    sidebar: "#10102e"
  },
  {
    id: "peachy-cream",
    name: "Peachy Cream",
    primary: "#ffb347",
    secondary: "#ff9a3d",
    background: "#1a150f",
    sidebar: "#2e251a"
  },
  {
    id: "bluebell",
    name: "Bluebell",
    primary: "#4169e1",
    secondary: "#3a5ecf",
    background: "#0a0f1a",
    sidebar: "#0d1a2e"
  },
  {
    id: "rose-dust",
    name: "Rose Dust",
    primary: "#d8949d",
    secondary: "#c2787d",
    background: "#1a1015",
    sidebar: "#2e1a20"
  },
  {
    id: "emerald-green",
    name: "Emerald Green",
    primary: "#50c878",
    secondary: "#3eb669",
    background: "#0a1a12",
    sidebar: "#0d2e1c"
  },
  {
    id: "charcoal-black",
    name: "Charcoal Black",
    primary: "#5a6270",
    secondary: "#4a525f",
    background: "#0a0a0a",
    sidebar: "#151515"
  },
  {
    id: "berry-blast",
    name: "Berry Blast",
    primary: "#c41e3a",
    secondary: "#9b1633",
    background: "#1a0a10",
    sidebar: "#2e0d1a"
  },
  {
    id: "sky-blue",
    name: "Sky Blue",
    primary: "#87ceeb",
    secondary: "#5faee3",
    background: "#0a101a",
    sidebar: "#0d1a2e"
  },
  {
    id: "ochre-brown",
    name: "Ochre Brown",
    primary: "#cc5500",
    secondary: "#aa4400",
    background: "#1a100a",
    sidebar: "#2e1a0d"
  },
  {
    id: "grape-purple",
    name: "Grape Purple",
    primary: "#6f2da8",
    secondary: "#5a1e8a",
    background: "#100a1a",
    sidebar: "#1a0d2e"
  },
  {
    id: "lime-zest",
    name: "Lime Zest",
    primary: "#32cd32",
    secondary: "#28a828",
    background: "#0a1a0a",
    sidebar: "#0d2e0d"
  },
  {
    id: "steel-blue",
    name: "Steel Blue",
    primary: "#4682b4",
    secondary: "#36648b",
    background: "#0a0f15",
    sidebar: "#0d1a25"
  },
  {
    id: "magenta-bright",
    name: "Magenta Bright",
    primary: "#ff1493",
    secondary: "#d90078",
    background: "#1a0a15",
    sidebar: "#2e0d20"
  },
  {
    id: "pistachio",
    name: "Pistachio",
    primary: "#93c572",
    secondary: "#7bb85e",
    background: "#0f150a",
    sidebar: "#1a250d"
  },
  {
    id: "plum-wine",
    name: "Plum Wine",
    primary: "#8b3a62",
    secondary: "#6b2c4a",
    background: "#150a10",
    sidebar: "#250d1a"
  }
];

export const UI_STYLES: UIStyle[] = [
  {
    id: "rounded-modern",
    name: "Rounded Modern",
    borderRadius: "12px",
    buttonStyle: "rounded",
    shadowIntensity: "light",
    fontWeight: "normal",
    spacing: "normal",
    accentStyle: "normal"
  },
  {
    id: "sharp-minimal",
    name: "Sharp Minimal",
    borderRadius: "0",
    buttonStyle: "square",
    shadowIntensity: "none",
    fontWeight: "light",
    spacing: "compact",
    accentStyle: "subtle"
  },
  {
    id: "pill-soft",
    name: "Pill Soft",
    borderRadius: "24px",
    buttonStyle: "pill",
    shadowIntensity: "light",
    fontWeight: "normal",
    spacing: "normal",
    accentStyle: "subtle"
  },
  {
    id: "brutalist-heavy",
    name: "Brutalist Heavy",
    borderRadius: "0",
    buttonStyle: "square",
    shadowIntensity: "heavy",
    fontWeight: "bold",
    spacing: "spacious",
    accentStyle: "bold"
  },
  {
    id: "retro-70s",
    name: "Retro 70s",
    borderRadius: "8px",
    buttonStyle: "rounded",
    shadowIntensity: "medium",
    fontWeight: "bold",
    spacing: "normal",
    accentStyle: "bold"
  },
  {
    id: "glass-morphism",
    name: "Glass Morphism",
    borderRadius: "16px",
    buttonStyle: "rounded",
    shadowIntensity: "light",
    fontWeight: "normal",
    spacing: "normal",
    accentStyle: "normal"
  },
  {
    id: "compact-dense",
    name: "Compact Dense",
    borderRadius: "4px",
    buttonStyle: "square",
    shadowIntensity: "none",
    fontWeight: "normal",
    spacing: "compact",
    accentStyle: "subtle"
  },
  {
    id: "spacious-airy",
    name: "Spacious Airy",
    borderRadius: "16px",
    buttonStyle: "rounded",
    shadowIntensity: "light",
    fontWeight: "light",
    spacing: "spacious",
    accentStyle: "subtle"
  },
  {
    id: "cyberpunk-neon",
    name: "Cyberpunk Neon",
    borderRadius: "2px",
    buttonStyle: "square",
    shadowIntensity: "heavy",
    fontWeight: "bold",
    spacing: "compact",
    accentStyle: "bold"
  },
  {
    id: "organic-rounded",
    name: "Organic Rounded",
    borderRadius: "20px",
    buttonStyle: "pill",
    shadowIntensity: "medium",
    fontWeight: "normal",
    spacing: "normal",
    accentStyle: "normal"
  },
  {
    id: "neo-classicism",
    name: "Neo Classicism",
    borderRadius: "6px",
    buttonStyle: "rounded",
    shadowIntensity: "medium",
    fontWeight: "bold",
    spacing: "spacious",
    accentStyle: "subtle"
  },
  {
    id: "minimalist-flat",
    name: "Minimalist Flat",
    borderRadius: "0",
    buttonStyle: "square",
    shadowIntensity: "none",
    fontWeight: "light",
    spacing: "normal",
    accentStyle: "subtle"
  },
  {
    id: "neumorphic",
    name: "Neumorphic",
    borderRadius: "12px",
    buttonStyle: "rounded",
    shadowIntensity: "medium",
    fontWeight: "normal",
    spacing: "compact",
    accentStyle: "subtle"
  },
  {
    id: "bold-geometric",
    name: "Bold Geometric",
    borderRadius: "0",
    buttonStyle: "square",
    shadowIntensity: "light",
    fontWeight: "bold",
    spacing: "spacious",
    accentStyle: "bold"
  },
  {
    id: "soft-gradient",
    name: "Soft Gradient",
    borderRadius: "18px",
    buttonStyle: "pill",
    shadowIntensity: "light",
    fontWeight: "normal",
    spacing: "normal",
    accentStyle: "normal"
  },
  {
    id: "vintage-serif",
    name: "Vintage Serif",
    borderRadius: "8px",
    buttonStyle: "rounded",
    shadowIntensity: "medium",
    fontWeight: "bold",
    spacing: "spacious",
    accentStyle: "normal"
  },
  {
    id: "tech-industrial",
    name: "Tech Industrial",
    borderRadius: "4px",
    buttonStyle: "square",
    shadowIntensity: "heavy",
    fontWeight: "normal",
    spacing: "compact",
    accentStyle: "bold"
  },
  {
    id: "playful-rounded",
    name: "Playful Rounded",
    borderRadius: "22px",
    buttonStyle: "pill",
    shadowIntensity: "light",
    fontWeight: "normal",
    spacing: "normal",
    accentStyle: "bold"
  },
  {
    id: "dark-minimal",
    name: "Dark Minimal",
    borderRadius: "6px",
    buttonStyle: "rounded",
    shadowIntensity: "light",
    fontWeight: "light",
    spacing: "compact",
    accentStyle: "subtle"
  },
  {
    id: "premium-elegant",
    name: "Premium Elegant",
    borderRadius: "10px",
    buttonStyle: "rounded",
    shadowIntensity: "medium",
    fontWeight: "normal",
    spacing: "spacious",
    accentStyle: "subtle"
  },
  {
    id: "sharp-angular",
    name: "Sharp Angular",
    borderRadius: "0",
    buttonStyle: "square",
    shadowIntensity: "none",
    fontWeight: "bold",
    spacing: "compact",
    accentStyle: "bold"
  },
  {
    id: "soft-shadows",
    name: "Soft Shadows",
    borderRadius: "14px",
    buttonStyle: "rounded",
    shadowIntensity: "light",
    fontWeight: "normal",
    spacing: "normal",
    accentStyle: "subtle"
  },
  {
    id: "bold-shadows",
    name: "Bold Shadows",
    borderRadius: "8px",
    buttonStyle: "rounded",
    shadowIntensity: "heavy",
    fontWeight: "bold",
    spacing: "spacious",
    accentStyle: "bold"
  },
  {
    id: "light-weight",
    name: "Light Weight",
    borderRadius: "12px",
    buttonStyle: "rounded",
    shadowIntensity: "none",
    fontWeight: "light",
    spacing: "spacious",
    accentStyle: "subtle"
  },
  {
    id: "mono-brutalist",
    name: "Mono Brutalist",
    borderRadius: "0",
    buttonStyle: "square",
    shadowIntensity: "heavy",
    fontWeight: "bold",
    spacing: "compact",
    accentStyle: "normal"
  },
  {
    id: "curved-modern",
    name: "Curved Modern",
    borderRadius: "20px",
    buttonStyle: "pill",
    shadowIntensity: "medium",
    fontWeight: "normal",
    spacing: "compact",
    accentStyle: "normal"
  },
  {
    id: "flat-bright",
    name: "Flat Bright",
    borderRadius: "8px",
    buttonStyle: "rounded",
    shadowIntensity: "none",
    fontWeight: "normal",
    spacing: "normal",
    accentStyle: "bold"
  },
  {
    id: "luxury-understated",
    name: "Luxury Understated",
    borderRadius: "6px",
    buttonStyle: "rounded",
    shadowIntensity: "medium",
    fontWeight: "light",
    spacing: "spacious",
    accentStyle: "subtle"
  },
  {
    id: "futuristic-clean",
    name: "Futuristic Clean",
    borderRadius: "2px",
    buttonStyle: "square",
    shadowIntensity: "light",
    fontWeight: "normal",
    spacing: "normal",
    accentStyle: "bold"
  },
  {
    id: "cozy-warm",
    name: "Cozy Warm",
    borderRadius: "16px",
    buttonStyle: "rounded",
    shadowIntensity: "medium",
    fontWeight: "normal",
    spacing: "spacious",
    accentStyle: "normal"
  }
];

// Default theme settings
export const DEFAULT_THEME_COLOR = "ocean-blue";
export const DEFAULT_UI_STYLE = "rounded-modern";
