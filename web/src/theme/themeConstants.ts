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
  },
  // Light Themes (20)
  {
    id: "light-sky",
    name: "Light Sky",
    primary: "#0077cc",
    secondary: "#005599",
    background: "#e8f4fc",
    sidebar: "#cce5f5"
  },
  {
    id: "light-mint",
    name: "Light Mint",
    primary: "#00a86b",
    secondary: "#008855",
    background: "#e8fcf2",
    sidebar: "#c5f0dc"
  },
  {
    id: "light-coral",
    name: "Light Coral",
    primary: "#e05050",
    secondary: "#cc3333",
    background: "#fceaea",
    sidebar: "#f5d0d0"
  },
  {
    id: "light-lavender",
    name: "Light Lavender",
    primary: "#8855cc",
    secondary: "#6633aa",
    background: "#f4eafc",
    sidebar: "#e5d0f5"
  },
  {
    id: "light-lemon",
    name: "Light Lemon",
    primary: "#ccaa00",
    secondary: "#aa8800",
    background: "#fcfce8",
    sidebar: "#f5f0c5"
  },
  {
    id: "light-rose",
    name: "Light Rose",
    primary: "#dd5588",
    secondary: "#cc3366",
    background: "#fceaf0",
    sidebar: "#f5d0e0"
  },
  {
    id: "light-sage",
    name: "Light Sage",
    primary: "#669966",
    secondary: "#558855",
    background: "#f0f8f0",
    sidebar: "#d8e8d8"
  },
  {
    id: "light-peach",
    name: "Light Peach",
    primary: "#dd7744",
    secondary: "#cc5522",
    background: "#fcf0e8",
    sidebar: "#f5dcc8"
  },
  {
    id: "light-periwinkle",
    name: "Light Periwinkle",
    primary: "#6677cc",
    secondary: "#4455aa",
    background: "#eef0fc",
    sidebar: "#d8dcf5"
  },
  {
    id: "light-aqua",
    name: "Light Aqua",
    primary: "#00aaaa",
    secondary: "#008888",
    background: "#e8fcfc",
    sidebar: "#c5f0f0"
  },
  {
    id: "cream-blue",
    name: "Cream Blue",
    primary: "#3388cc",
    secondary: "#2266aa",
    background: "#f8f8f0",
    sidebar: "#e8e8d8"
  },
  {
    id: "cream-green",
    name: "Cream Green",
    primary: "#44aa66",
    secondary: "#338855",
    background: "#f8f8f0",
    sidebar: "#e0e8d8"
  },
  {
    id: "snow-white",
    name: "Snow White",
    primary: "#5577aa",
    secondary: "#445588",
    background: "#fafafa",
    sidebar: "#e8e8e8"
  },
  {
    id: "ivory-gold",
    name: "Ivory Gold",
    primary: "#bb8833",
    secondary: "#996622",
    background: "#faf8f0",
    sidebar: "#f0e8d0"
  },
  {
    id: "cotton-candy",
    name: "Cotton Candy",
    primary: "#cc66aa",
    secondary: "#aa4488",
    background: "#fcf0f8",
    sidebar: "#f5d8ec"
  },
  {
    id: "morning-fog",
    name: "Morning Fog",
    primary: "#668899",
    secondary: "#556677",
    background: "#f0f4f8",
    sidebar: "#dce4e8"
  },
  {
    id: "vanilla-cream",
    name: "Vanilla Cream",
    primary: "#aa8855",
    secondary: "#886633",
    background: "#faf8f2",
    sidebar: "#f0e8d8"
  },
  {
    id: "soft-teal",
    name: "Soft Teal",
    primary: "#339999",
    secondary: "#227777",
    background: "#f0f8f8",
    sidebar: "#d8e8e8"
  },
  {
    id: "blush-pink",
    name: "Blush Pink",
    primary: "#cc7788",
    secondary: "#aa5566",
    background: "#fcf4f4",
    sidebar: "#f5e0e0"
  },
  {
    id: "pale-jade",
    name: "Pale Jade",
    primary: "#55aa88",
    secondary: "#448866",
    background: "#f0f8f4",
    sidebar: "#d8e8dc"
  },
  // Orange-Based Light Themes (20)
  {
    id: "light-tangerine",
    name: "Light Tangerine",
    primary: "#e87830",
    secondary: "#cc6020",
    background: "#fcf4e8",
    sidebar: "#f5e0c8"
  },
  {
    id: "light-amber",
    name: "Light Amber",
    primary: "#cc9000",
    secondary: "#aa7800",
    background: "#fcf8e8",
    sidebar: "#f5ecc8"
  },
  {
    id: "light-apricot",
    name: "Light Apricot",
    primary: "#e08050",
    secondary: "#c86838",
    background: "#fcf0e8",
    sidebar: "#f5dcc8"
  },
  {
    id: "light-pumpkin",
    name: "Light Pumpkin",
    primary: "#dd6620",
    secondary: "#bb5010",
    background: "#fcf0e0",
    sidebar: "#f5dcc0"
  },
  {
    id: "light-copper",
    name: "Light Copper",
    primary: "#b87040",
    secondary: "#a06030",
    background: "#f8f0e8",
    sidebar: "#f0dcc8"
  },
  {
    id: "light-papaya",
    name: "Light Papaya",
    primary: "#e89040",
    secondary: "#cc7828",
    background: "#fcf4e4",
    sidebar: "#f5e4c0"
  },
  {
    id: "light-marigold",
    name: "Light Marigold",
    primary: "#e8a020",
    secondary: "#cc8810",
    background: "#fcf8e0",
    sidebar: "#f5ecc0"
  },
  {
    id: "cream-orange",
    name: "Cream Orange",
    primary: "#dd7744",
    secondary: "#c06030",
    background: "#faf8f0",
    sidebar: "#f0e8d8"
  },
  {
    id: "peach-light",
    name: "Peach Light",
    primary: "#e08060",
    secondary: "#c86848",
    background: "#fcf4f0",
    sidebar: "#f5e0d8"
  },
  {
    id: "melon-orange",
    name: "Melon Orange",
    primary: "#e87050",
    secondary: "#d05838",
    background: "#fcf0ec",
    sidebar: "#f5dcd4"
  },
  {
    id: "honey-gold",
    name: "Honey Gold",
    primary: "#cc9933",
    secondary: "#aa7722",
    background: "#f8f4e8",
    sidebar: "#f0e4d0"
  },
  {
    id: "cantaloupe",
    name: "Cantaloupe",
    primary: "#e89060",
    secondary: "#d07848",
    background: "#fcf4ec",
    sidebar: "#f5e0d4"
  },
  {
    id: "sunset-cream",
    name: "Sunset Cream",
    primary: "#e07040",
    secondary: "#c85828",
    background: "#faf4f0",
    sidebar: "#f5e4dc"
  },
  {
    id: "marmalade-light",
    name: "Marmalade Light",
    primary: "#dd8030",
    secondary: "#c06818",
    background: "#fcf4e4",
    sidebar: "#f5e0c4"
  },
  {
    id: "citrus-light",
    name: "Citrus Light",
    primary: "#e89820",
    secondary: "#cc8010",
    background: "#fcf8e4",
    sidebar: "#f5ecc4"
  },
  {
    id: "butterscotch",
    name: "Butterscotch",
    primary: "#cc8844",
    secondary: "#b07030",
    background: "#f8f4e8",
    sidebar: "#f0e4d4"
  },
  {
    id: "nectar-light",
    name: "Nectar Light",
    primary: "#e08840",
    secondary: "#c87028",
    background: "#fcf4e8",
    sidebar: "#f5e4cc"
  },
  {
    id: "tiger-lily",
    name: "Tiger Lily",
    primary: "#e06830",
    secondary: "#cc5020",
    background: "#fcf0e4",
    sidebar: "#f5dcc4"
  },
  {
    id: "coral-cream",
    name: "Coral Cream",
    primary: "#e07050",
    secondary: "#cc5838",
    background: "#fcf4f0",
    sidebar: "#f5e0d8"
  },
  {
    id: "golden-peach",
    name: "Golden Peach",
    primary: "#dd9050",
    secondary: "#c07838",
    background: "#fcf4ec",
    sidebar: "#f5e4d4"
  },
  // Orange-Purple Light Themes (20)
  {
    id: "sunset-light",
    name: "Sunset Light",
    primary: "#e06030",
    secondary: "#9050b0",
    background: "#fcf0f4",
    sidebar: "#f5dce4"
  },
  {
    id: "tropical-dawn",
    name: "Tropical Dawn",
    primary: "#e08040",
    secondary: "#8060c0",
    background: "#fcf4f8",
    sidebar: "#f5e0ec"
  },
  {
    id: "orchid-cream",
    name: "Orchid Cream",
    primary: "#e05030",
    secondary: "#a860b0",
    background: "#faf4f8",
    sidebar: "#f5e4ec"
  },
  {
    id: "mango-violet",
    name: "Mango Violet",
    primary: "#e09030",
    secondary: "#9048a8",
    background: "#fcf4f4",
    sidebar: "#f5e4e8"
  },
  {
    id: "peach-lilac",
    name: "Peach Lilac",
    primary: "#e08050",
    secondary: "#b088c0",
    background: "#fcf8f8",
    sidebar: "#f5ecf0"
  },
  {
    id: "ember-iris",
    name: "Ember Iris",
    primary: "#dd6040",
    secondary: "#7050b0",
    background: "#fcf4f8",
    sidebar: "#f5e0e8"
  },
  {
    id: "citrus-lavender",
    name: "Citrus Lavender",
    primary: "#e08820",
    secondary: "#6040a8",
    background: "#fcf4f0",
    sidebar: "#f5e8e4"
  },
  {
    id: "tangerine-violet",
    name: "Tangerine Violet",
    primary: "#dd6800",
    secondary: "#8830a0",
    background: "#fcf0f0",
    sidebar: "#f5e0e4"
  },
  {
    id: "coral-plum",
    name: "Coral Plum",
    primary: "#e07858",
    secondary: "#a088b8",
    background: "#fcf8f4",
    sidebar: "#f5ece8"
  },
  {
    id: "amber-purple",
    name: "Amber Purple",
    primary: "#b86840",
    secondary: "#7050c0",
    background: "#f8f4f4",
    sidebar: "#f0e8ec"
  },
  {
    id: "gold-iris",
    name: "Gold Iris",
    primary: "#ccaa20",
    secondary: "#8870b8",
    background: "#fcf8f4",
    sidebar: "#f5ece8"
  },
  {
    id: "nectar-violet",
    name: "Nectar Violet",
    primary: "#e0a048",
    secondary: "#b050d0",
    background: "#fcf8f8",
    sidebar: "#f5ecf0"
  },
  {
    id: "terra-magenta",
    name: "Terra Magenta",
    primary: "#b04818",
    secondary: "#c03068",
    background: "#f8f0f0",
    sidebar: "#f0e4e4"
  },
  {
    id: "pumpkin-lavender",
    name: "Pumpkin Lavender",
    primary: "#d86000",
    secondary: "#7830a0",
    background: "#fcf4f4",
    sidebar: "#f5e4e8"
  },
  {
    id: "marmalade-violet",
    name: "Marmalade Violet",
    primary: "#e08000",
    secondary: "#5828a0",
    background: "#fcf4f0",
    sidebar: "#f5e4e4"
  },
  {
    id: "spice-lilac",
    name: "Spice Lilac",
    primary: "#d06000",
    secondary: "#9020d0",
    background: "#fcf0f4",
    sidebar: "#f5e0e8"
  },
  {
    id: "papaya-plum",
    name: "Papaya Plum",
    primary: "#dd7000",
    secondary: "#5038a8",
    background: "#fcf4f8",
    sidebar: "#f5e4ec"
  },
  {
    id: "apricot-mauve",
    name: "Apricot Mauve",
    primary: "#e0a070",
    secondary: "#b020d0",
    background: "#fcf8f8",
    sidebar: "#f5ecf0"
  },
  {
    id: "tiger-violet",
    name: "Tiger Violet",
    primary: "#e05030",
    secondary: "#5830d0",
    background: "#fcf4f8",
    sidebar: "#f5e0ec"
  },
  {
    id: "harvest-bloom",
    name: "Harvest Bloom",
    primary: "#e08868",
    secondary: "#c870d8",
    background: "#fcf8f8",
    sidebar: "#f5f0f4"
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
