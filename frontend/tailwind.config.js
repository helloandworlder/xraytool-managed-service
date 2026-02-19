/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{vue,ts,tsx}'],
  theme: {
    extend: {
      colors: {
        canvas: '#f4f6f8',
        ink: '#0f172a',
        steel: '#334155',
        brand: '#0f766e',
        sea: '#0369a1',
        warn: '#dc2626'
      },
      boxShadow: {
        panel: '0 12px 32px rgba(15, 23, 42, 0.08)'
      },
      fontFamily: {
        sans: ['Manrope', 'sans-serif']
      }
    }
  },
  plugins: []
}
