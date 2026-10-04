// ESLint flat config (v9). Mínimo corporativo; endurecer según standards/react.md.
import js from '@eslint/js'

export default [
  js.configs.recommended,
  {
    files: ['src/**/*.{ts,tsx}'],
    languageOptions: { ecmaVersion: 2020, sourceType: 'module' },
    rules: {
      'no-unused-vars': 'warn',
    },
  },
]
