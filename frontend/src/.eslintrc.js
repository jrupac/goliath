module.exports = {
  "extends": [
    "eslint:recommended",
    "plugin:react/recommended"
  ],
  "rules": {
    "@typescript-eslint/no-unused-vars": "warn",
    "@typescript-eslint/no-use-before-define": ["warn"],
    "default-case": "off",
    "func-style": 0,
    "id-length": "off",
    "indent": ["error", 2],
    "max-lines": "off",
    "no-bitwise": "off",
    "no-case-declarations": "warn",
    "no-console": "off",
    "no-extra-semi": "warn",
    "no-magic-numbers": "off",
    "no-param-reassign": "off",
    "no-use-before-define": "off",
    "react/no-set-state": "off",
    // For now.
    "require-jsdoc": "off",
    "strict": "off"
  },
  "env": {
    "node": true,
    "browser": true,
    "es6": true
  },
  "parser": "@typescript-eslint/parser",
  "plugins": [
    "@typescript-eslint", "react"
  ]
};