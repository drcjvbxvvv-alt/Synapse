import js from '@eslint/js'
import globals from 'globals'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'
import tseslint from 'typescript-eslint'
import { globalIgnores } from 'eslint/config'

export default tseslint.config([
  globalIgnores(['dist', 'e2e']),
  {
    files: ['**/*.{ts,tsx}'],
    extends: [
      js.configs.recommended,
      tseslint.configs.recommended,
      reactHooks.configs['recommended-latest'],
      reactRefresh.configs.vite,
    ],
    languageOptions: {
      ecmaVersion: 2020,
      globals: globals.browser,
    },
    rules: {
      // Prevent `any` from silently spreading — explicit disable comments
      // required whenever a genuine exception is needed.
      '@typescript-eslint/no-explicit-any': 'error',

      // Unused variables are usually bugs; allow leading-underscore prefix
      // for intentionally unused params (e.g. `_event`).
      '@typescript-eslint/no-unused-vars': ['error', {
        argsIgnorePattern: '^_',
        varsIgnorePattern: '^_',
        caughtErrorsIgnorePattern: '^_',
      }],

      // Require exhaustive deps to keep hooks correct.
      // Already provided by reactHooks plugin; this makes it an error
      // rather than the plugin default of warn.
      'react-hooks/exhaustive-deps': 'error',
    },
  },
])
