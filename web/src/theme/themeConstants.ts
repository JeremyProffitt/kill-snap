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
  // Brighter Themes (20)
  {
    id: "bright-sky",
    name: "Bright Sky",
    primary: "#00bfff",
    secondary: "#0099cc",
    background: "#1a2530",
    sidebar: "#253545"
  },
  {
    id: "bright-lime",
    name: "Bright Lime",
    primary: "#7fff00",
    secondary: "#66cc00",
    background: "#1a2518",
    sidebar: "#253520"
  },
  {
    id: "bright-coral",
    name: "Bright Coral",
    primary: "#ff7f7f",
    secondary: "#ff5555",
    background: "#2a1a1a",
    sidebar: "#3a2525"
  },
  {
    id: "bright-aqua",
    name: "Bright Aqua",
    primary: "#00ffff",
    secondary: "#00cccc",
    background: "#1a2828",
    sidebar: "#253a3a"
  },
  {
    id: "bright-yellow",
    name: "Bright Yellow",
    primary: "#ffff00",
    secondary: "#cccc00",
    background: "#252518",
    sidebar: "#3a3a25"
  },
  {
    id: "bright-pink",
    name: "Bright Pink",
    primary: "#ff69b4",
    secondary: "#ff1493",
    background: "#281a25",
    sidebar: "#3a2535"
  },
  {
    id: "bright-mint",
    name: "Bright Mint",
    primary: "#98ff98",
    secondary: "#7acc7a",
    background: "#1a2820",
    sidebar: "#253a2a"
  },
  {
    id: "bright-peach",
    name: "Bright Peach",
    primary: "#ffcba4",
    secondary: "#ffb380",
    background: "#282018",
    sidebar: "#3a3025"
  },
  {
    id: "bright-lavender",
    name: "Bright Lavender",
    primary: "#e6e6fa",
    secondary: "#d8bfd8",
    background: "#201a28",
    sidebar: "#302540"
  },
  {
    id: "bright-turquoise",
    name: "Bright Turquoise",
    primary: "#40e0d0",
    secondary: "#30c0b0",
    background: "#182525",
    sidebar: "#253838"
  },
  {
    id: "electric-blue",
    name: "Electric Blue",
    primary: "#7df9ff",
    secondary: "#00d4ff",
    background: "#182028",
    sidebar: "#253040"
  },
  {
    id: "vivid-green",
    name: "Vivid Green",
    primary: "#00ff7f",
    secondary: "#00cc66",
    background: "#18251a",
    sidebar: "#253a28"
  },
  {
    id: "hot-magenta",
    name: "Hot Magenta",
    primary: "#ff00ff",
    secondary: "#cc00cc",
    background: "#251a25",
    sidebar: "#3a2838"
  },
  {
    id: "sunshine-gold",
    name: "Sunshine Gold",
    primary: "#ffd700",
    secondary: "#daa520",
    background: "#252015",
    sidebar: "#3a3020"
  },
  {
    id: "bright-violet",
    name: "Bright Violet",
    primary: "#ee82ee",
    secondary: "#da70d6",
    background: "#251a28",
    sidebar: "#382840"
  },
  {
    id: "neon-green",
    name: "Neon Green",
    primary: "#39ff14",
    secondary: "#32e010",
    background: "#151f15",
    sidebar: "#203020"
  },
  {
    id: "candy-pink",
    name: "Candy Pink",
    primary: "#ffb6c1",
    secondary: "#ffa0b0",
    background: "#281820",
    sidebar: "#3a2530"
  },
  {
    id: "lemon-yellow",
    name: "Lemon Yellow",
    primary: "#fff44f",
    secondary: "#eedc30",
    background: "#25251a",
    sidebar: "#383820"
  },
  {
    id: "ice-blue",
    name: "Ice Blue",
    primary: "#a0d2eb",
    secondary: "#80c2e0",
    background: "#1a2025",
    sidebar: "#253038"
  },
  {
    id: "spring-green",
    name: "Spring Green",
    primary: "#00ff7f",
    secondary: "#00e070",
    background: "#152018",
    sidebar: "#203525"
  },
  // Orange-Based Themes (20)
  {
    id: "tangerine",
    name: "Tangerine",
    primary: "#ff9966",
    secondary: "#ff7744",
    background: "#1f1510",
    sidebar: "#2e2018"
  },
  {
    id: "burnt-orange",
    name: "Burnt Orange",
    primary: "#cc5500",
    secondary: "#aa4400",
    background: "#1a1008",
    sidebar: "#2a1810"
  },
  {
    id: "amber-glow",
    name: "Amber Glow",
    primary: "#ffbf00",
    secondary: "#e6a800",
    background: "#1f1a0a",
    sidebar: "#2e2810"
  },
  {
    id: "rust-orange",
    name: "Rust Orange",
    primary: "#b7410e",
    secondary: "#8b3008",
    background: "#18100a",
    sidebar: "#251810"
  },
  {
    id: "carrot-orange",
    name: "Carrot Orange",
    primary: "#ed9121",
    secondary: "#d07818",
    background: "#1a1508",
    sidebar: "#2a2010"
  },
  {
    id: "mango-tango",
    name: "Mango Tango",
    primary: "#ff8243",
    secondary: "#e06830",
    background: "#1f150f",
    sidebar: "#2e2018"
  },
  {
    id: "copper-shine",
    name: "Copper Shine",
    primary: "#b87333",
    secondary: "#9a5f28",
    background: "#18120a",
    sidebar: "#251a10"
  },
  {
    id: "pumpkin-spice",
    name: "Pumpkin Spice",
    primary: "#ff7518",
    secondary: "#e06010",
    background: "#1a1208",
    sidebar: "#2a1a10"
  },
  {
    id: "tiger-orange",
    name: "Tiger Orange",
    primary: "#fc6a03",
    secondary: "#dd5500",
    background: "#1f1008",
    sidebar: "#2e1810"
  },
  {
    id: "apricot-cream",
    name: "Apricot Cream",
    primary: "#fbceb1",
    secondary: "#e8b090",
    background: "#201a15",
    sidebar: "#302820"
  },
  {
    id: "terracotta",
    name: "Terracotta",
    primary: "#e2725b",
    secondary: "#c85a45",
    background: "#1a1210",
    sidebar: "#281a18"
  },
  {
    id: "safety-orange",
    name: "Safety Orange",
    primary: "#ff6700",
    secondary: "#e05500",
    background: "#1a1008",
    sidebar: "#2a1810"
  },
  {
    id: "persimmon",
    name: "Persimmon",
    primary: "#ec5800",
    secondary: "#cc4800",
    background: "#18100a",
    sidebar: "#251810"
  },
  {
    id: "papaya-orange",
    name: "Papaya Orange",
    primary: "#ff9f00",
    secondary: "#e08800",
    background: "#1a150a",
    sidebar: "#2a2010"
  },
  {
    id: "clementine",
    name: "Clementine",
    primary: "#f28500",
    secondary: "#d47000",
    background: "#1a120a",
    sidebar: "#281a10"
  },
  {
    id: "coral-orange",
    name: "Coral Orange",
    primary: "#ff7f50",
    secondary: "#e06840",
    background: "#1f1510",
    sidebar: "#2e2018"
  },
  {
    id: "mandarin",
    name: "Mandarin",
    primary: "#f37a48",
    secondary: "#d86038",
    background: "#1a1210",
    sidebar: "#281a15"
  },
  {
    id: "autumn-orange",
    name: "Autumn Orange",
    primary: "#eb9605",
    secondary: "#cc8000",
    background: "#1a1508",
    sidebar: "#282010"
  },
  {
    id: "bronze-glow",
    name: "Bronze Glow",
    primary: "#cd7f32",
    secondary: "#b06a28",
    background: "#18120a",
    sidebar: "#251a10"
  },
  {
    id: "desert-sand",
    name: "Desert Sand",
    primary: "#edc9af",
    secondary: "#d4b090",
    background: "#1f1a15",
    sidebar: "#2e2820"
  },
  // Orange-Purple Themes (20)
  {
    id: "sunset-dream",
    name: "Sunset Dream",
    primary: "#ff6b35",
    secondary: "#9b4dca",
    background: "#1a1018",
    sidebar: "#281820"
  },
  {
    id: "tropical-dusk",
    name: "Tropical Dusk",
    primary: "#ff8c42",
    secondary: "#8b5cf6",
    background: "#18101a",
    sidebar: "#251825"
  },
  {
    id: "fire-orchid",
    name: "Fire Orchid",
    primary: "#ff5722",
    secondary: "#ba68c8",
    background: "#1a0f15",
    sidebar: "#281520"
  },
  {
    id: "mango-plum",
    name: "Mango Plum",
    primary: "#ffa726",
    secondary: "#ab47bc",
    background: "#1a1215",
    sidebar: "#281a20"
  },
  {
    id: "peach-violet",
    name: "Peach Violet",
    primary: "#ffab91",
    secondary: "#ce93d8",
    background: "#1f1520",
    sidebar: "#2e2028"
  },
  {
    id: "ember-lavender",
    name: "Ember Lavender",
    primary: "#ff7043",
    secondary: "#7e57c2",
    background: "#181015",
    sidebar: "#25181f"
  },
  {
    id: "citrus-grape",
    name: "Citrus Grape",
    primary: "#ff9800",
    secondary: "#673ab7",
    background: "#1a1018",
    sidebar: "#281820"
  },
  {
    id: "tangerine-amethyst",
    name: "Tangerine Amethyst",
    primary: "#ff6f00",
    secondary: "#9c27b0",
    background: "#180f18",
    sidebar: "#251520"
  },
  {
    id: "coral-mauve",
    name: "Coral Mauve",
    primary: "#ff8a65",
    secondary: "#b39ddb",
    background: "#1a1518",
    sidebar: "#282025"
  },
  {
    id: "copper-violet",
    name: "Copper Violet",
    primary: "#bf6e40",
    secondary: "#7c4dff",
    background: "#151015",
    sidebar: "#201820"
  },
  {
    id: "amber-iris",
    name: "Amber Iris",
    primary: "#ffc107",
    secondary: "#9575cd",
    background: "#1a1515",
    sidebar: "#282020"
  },
  {
    id: "nectar-bloom",
    name: "Nectar Bloom",
    primary: "#ffb74d",
    secondary: "#e040fb",
    background: "#1a1218",
    sidebar: "#281a22"
  },
  {
    id: "rust-magenta",
    name: "Rust Magenta",
    primary: "#bf360c",
    secondary: "#d81b60",
    background: "#150a10",
    sidebar: "#201015"
  },
  {
    id: "pumpkin-orchid",
    name: "Pumpkin Orchid",
    primary: "#ef6c00",
    secondary: "#8e24aa",
    background: "#180f15",
    sidebar: "#251520"
  },
  {
    id: "marmalade-plum",
    name: "Marmalade Plum",
    primary: "#fb8c00",
    secondary: "#6a1b9a",
    background: "#1a1015",
    sidebar: "#28181f"
  },
  {
    id: "burnt-lilac",
    name: "Burnt Lilac",
    primary: "#e65100",
    secondary: "#aa00ff",
    background: "#150a15",
    sidebar: "#201020"
  },
  {
    id: "spice-violet",
    name: "Spice Violet",
    primary: "#f57c00",
    secondary: "#5e35b1",
    background: "#181018",
    sidebar: "#251822"
  },
  {
    id: "apricot-purple",
    name: "Apricot Purple",
    primary: "#ffcc80",
    secondary: "#d500f9",
    background: "#1a1518",
    sidebar: "#282025"
  },
  {
    id: "tiger-iris",
    name: "Tiger Iris",
    primary: "#ff5722",
    secondary: "#651fff",
    background: "#180a15",
    sidebar: "#251020"
  },
  {
    id: "harvest-twilight",
    name: "Harvest Twilight",
    primary: "#ff9e80",
    secondary: "#ea80fc",
    background: "#1f1520",
    sidebar: "#2e2030"
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
