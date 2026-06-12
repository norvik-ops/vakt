import js from '@eslint/js'
import globals from 'globals'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'
import tseslint from 'typescript-eslint'

export default tseslint.config(
  {
    ignores: [
      'dist',
      'node_modules',
      'e2e/**',
      'playwright.config.ts',
      'src/api/generated.ts',
      'src/components/ui/**',
    ],
  },
  {
    extends: [js.configs.recommended, ...tseslint.configs.strictTypeChecked],
    files: ['**/*.{ts,tsx}'],
    languageOptions: {
      ecmaVersion: 2020,
      globals: globals.browser,
      parserOptions: {
        projectService: {
          defaultProject: 'tsconfig.app.json',
          allowDefaultProject: ['src/modules/vaktprivacy/pages/DSRPage.test.tsx'],
        },
        tsconfigRootDir: import.meta.dirname,
      },
    },
    plugins: {
      'react-hooks': reactHooks,
      'react-refresh': reactRefresh,
    },
    rules: {
      ...reactHooks.configs.recommended.rules,
      'react-refresh/only-export-components': [
        'warn',
        { allowConstantExport: true },
      ],
      '@typescript-eslint/no-explicit-any': 'error',
      // Numbers and booleans stringify fine — only objects/undefined are problematic.
      '@typescript-eslint/restrict-template-expressions': ['error', { allowNumber: true, allowBoolean: true }],
      // TanStack Query uses void as a generic type argument (e.g. useMutation<void, Error, Input>).
      '@typescript-eslint/no-invalid-void-type': 'warn',
      // Defensive optional chains are harmless; TypeScript's non-null inference is sometimes overly confident.
      '@typescript-eslint/no-unnecessary-condition': 'warn',
      // Non-null assertions (!) are used in delete/action handlers where id is guaranteed by route.
      '@typescript-eslint/no-non-null-assertion': 'warn',
      // LLM response parsing and some API shapes necessarily use any; warn but don't block CI.
      '@typescript-eslint/no-unsafe-assignment': 'warn',
      '@typescript-eslint/no-unsafe-member-access': 'warn',
      '@typescript-eslint/no-unsafe-argument': 'warn',
      '@typescript-eslint/no-unsafe-return': 'warn',
      '@typescript-eslint/no-unsafe-call': 'warn',
    },
  },
  {
    // Test files need `as any` for mock return typing — vi.mocked().mockReturnValue()
    // requires partial shapes that don't satisfy the full UseQueryResult/UseMutationResult types.
    files: ['**/*.test.{ts,tsx}', 'src/test-utils/**'],
    rules: {
      '@typescript-eslint/no-explicit-any': 'warn',
    },
  },
)
