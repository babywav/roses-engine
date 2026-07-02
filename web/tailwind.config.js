/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      fontFamily: {
        sans: ['"Geist Variable"', "system-ui", "sans-serif"],
        mono: ['"Geist Mono Variable"', "ui-monospace", "monospace"],
      },
      colors: {
        ink: {
          900: "#0A0C11",
          800: "#0E1117",
          700: "#121620",
          600: "#161B26",
          500: "#1C2230",
        },
        accent: {
          DEFAULT: "#3B82F6",
          soft: "#5B9DFF",
          deep: "#2563EB",
        },
        muted: "#8A93A6",
      },
      boxShadow: {
        glow: "0 0 0 1px rgba(59,130,246,0.35), 0 8px 30px -6px rgba(59,130,246,0.5)",
        card: "0 20px 60px -20px rgba(0,0,0,0.7)",
      },
      borderRadius: {
        "4xl": "2rem",
      },
      keyframes: {
        float: {
          "0%,100%": { transform: "translateY(0)" },
          "50%": { transform: "translateY(-10px)" },
        },
        pulseGlow: {
          "0%,100%": { opacity: "0.5" },
          "50%": { opacity: "1" },
        },
      },
      animation: {
        float: "float 6s ease-in-out infinite",
        pulseGlow: "pulseGlow 3s ease-in-out infinite",
      },
    },
  },
  plugins: [],
};
