module.exports = {
  "extends": ["react-app"],
  "rules": {
    "no-use-before-define": "off",
    "@typescript-eslint/no-use-before-define": ["error"],
    "default-case": "off",
    "id-length": "off",
    "indent": ["error", 2],
    "func-style": 0,
    "max-lines": "off",
    "no-bitwise": "off",
    "no-console": "off",
    "no-magic-numbers": "off",
    "no-param-reassign": "off",
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