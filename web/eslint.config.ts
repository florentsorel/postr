import js from "@eslint/js"
import ts from "typescript-eslint"
import vue from "eslint-plugin-vue"
import vueTs from "@vue/eslint-config-typescript"
import prettier from "eslint-config-prettier"

export default ts.config(
  js.configs.recommended,
  ...ts.configs.recommended,
  ...vue.configs["flat/recommended"],
  ...vueTs(),
  prettier,
  {
    rules: {
      "vue/multi-word-component-names": "off",
    },
  },
  {
    ignores: ["dist/**", "node_modules/**", "auto-imports.d.ts", "components.d.ts"],
  }
)
