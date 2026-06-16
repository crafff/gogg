// PostCSS pipeline for Tailwind + autoprefixer. Plain JS rather than
// TS because postcss can't load TS config without an extra loader and
// this file is too small to justify one.
export default {
  plugins: {
    tailwindcss: {},
    autoprefixer: {},
  },
};
