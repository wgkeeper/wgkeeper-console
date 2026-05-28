/// <reference types="vite/client" />

// Fontsource variable packages ship CSS with no type declarations; these
// side-effect imports register the @font-face rules at build time.
declare module '@fontsource-variable/geist';
declare module '@fontsource-variable/geist-mono';
