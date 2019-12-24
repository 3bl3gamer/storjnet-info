module.exports = {
	env: {
		browser: true,
		es6: true,
	},
	extends: 'plugin:prettier/recommended',
	globals: {
		Atomics: 'readonly',
		SharedArrayBuffer: 'readonly',
	},
	parserOptions: {
		ecmaVersion: 2020,
		sourceType: 'module',
	},
	rules: {
		'no-console': 'warn',
		'no-unused-vars': ['error', { vars: 'all', args: 'none' }],
		'no-undef': 'error',
	},
	globals: {
		process: true,
	},
}
