---  
name: design-md-apply  
description: >-  
  Apply design tokens from DESIGN.md to UI components: replace hardcoded colors,  
  fonts, spacing, rounded corners, and enforce Do's and Don'ts. Use this skill  
  whenever you need to style UI components, implement visual designs, or ensure  
  consistency with an existing DESIGN.md file in the project. Optionally accepts  
  a path to a specific DESIGN.md file.  
metadata:  
  spec-url: https://github.com/google-labs-code/design.md  
  version: "1.0"  
---  
   
# design-md-apply  
   
## Purpose  
   
This skill enables you (the agent) to consume a project's `DESIGN.md` file and apply its design system to UI components. You will replace hardcoded visual values (colors, font stacks, spacing, border radii, etc.) with token references or directly with the values defined in `DESIGN.md`, and ensure compliance with the project's Do's and Don'ts.  
   
For the complete `DESIGN.md` specification, see: https://github.com/google-labs-code/design.md/blob/main/docs/spec.md  
   
## When to use this skill  
   
Use this skill when:  
- The user asks you to "apply the design system" to a component, page, or whole project.  
- You see a `DESIGN.md` file in the project root or documented location.  
- The user mentions tokens, design system, or visual consistency across UI.  
- You are generating new UI code that should follow an existing `DESIGN.md`.  
- The user provides a specific path to a `DESIGN.md` file (e.g., "apply design system from ./docs/DESIGN.md").  
   
Do not use this skill if no `DESIGN.md` exists, or if the user explicitly wants ad-hoc styling.  
   
## How to apply DESIGN.md  
   
### 0. Validate DESIGN.md first (best practice)  
   
Before applying any tokens, validate the design system file:  
   
```bash  
npx @google/design.md lint <design_file_path>  
```  
   
If the linter reports **errors** (`summary.errors > 0`), **stop immediately** and inform the user. Do not apply a broken design system. If it reports only warnings, proceed but note them in your output.  
   
**Linter reference:** All eight lint rules are documented at:    
https://github.com/google-labs-code/design.md/blob/main/docs/spec.md#linting-rules  
   
### 1. Locate and parse DESIGN.md  
   
**Determine the DESIGN.md file path:**  
- If the user provides a specific path in their request (e.g., "use ./docs/DESIGN.md", "apply from path/to/DESIGN.md"), extract that path and use it as `<design_file_path>`.  
- If no path is provided, look for `DESIGN.md` in the current working directory, then recursively up to the project root.  
- If multiple design systems exist and no path was specified, ask the user which one to apply (default: the one closest to the current file).  
  
**Parse the file:**  
- Extract the **YAML front matter** (between `---` lines). This contains machine-readable tokens (colors, typography, spacing, rounded, components).  
- Read the **markdown body** for human-readable guidelines, especially the **Do's and Don'ts** section.  
   
If the file is missing or malformed, stop and inform the user.  
   
**Format specification:** YAML schema details are documented at:    
https://github.com/google-labs-code/design.md/blob/main/docs/spec.md#yaml-front-matter  
   
### 2. Understand token resolution  
   
- Tokens in YAML can reference other tokens using `{path.to.token}`, e.g. `backgroundColor: "{colors.primary}"`. Resolve all references recursively.  
- For the `components` section, property names follow this **allowed set**: `backgroundColor`, `textColor`, `typography`, `rounded`, `padding`, `size`, `height`, `width`. No other properties (e.g. `borderColor`) are valid. If you encounter an unknown property, warn the user.  
- Component properties can accept **token references** (e.g. `{colors.primary}`), **raw hex values** (e.g. `#1A1C1E`), **raw RGBA strings** (e.g. `rgba(255, 255, 255, 0.1)`), or **composite references** (e.g. `{typography.body-md}`).  
- If a reference cannot be resolved, treat it as an error. Do not guess values.  
- **Cycle detection**: If you encounter a circular reference (e.g. `a: "{b}"`, `b: "{a}"`), stop resolution and report the error.  
   
**Token reference rules:** https://github.com/google-labs-code/design.md/blob/main/docs/spec.md#token-references  
   
### 3. Replace hardcoded values in UI code  
   
For each UI component or style block you modify:  
   
- **Colors**: Replace any literal hex, rgb, hsl, or named color with the exact resolved value from `DESIGN.md`. Output format depends on the target framework:  
  - **CSS**: Use CSS custom properties (e.g. `var(--color-primary)`) if the project uses them, otherwise use the resolved hex value.  
  - **Tailwind**: Use utility classes (e.g. `bg-primary`, `text-on-primary`).  
  - **CSS-in-JS / styled-components**: Use theme references (e.g. `theme.colors.primary`) or direct values.  
- **Typography**: Apply `fontFamily`, `fontSize`, `fontWeight`, `lineHeight`, `letterSpacing`, `fontFeature`, `fontVariation` from the appropriate typography token (e.g. `body-md`). Do not use arbitrary font sizes or families.  
- **Spacing**: Use the `spacing` scale (e.g. `spacing.md`) for margins, paddings, gaps. Replace inline numbers like `margin: 16px` with the corresponding semantic spacing token.  
- **Rounded corners**: Use the `rounded` scale (e.g. `rounded.md`). Replace any magic numbers with token references.  
- **Component-specific overrides**: If `DESIGN.md` defines a `components` entry (e.g. `button-primary`), apply those properties exactly. For component variants (e.g. `button-primary-hover`), apply only when the component state matches.  
   
**Example transformation**  
   
Hardcoded:  
```css  
.button {  
  background-color: #2665fd;  
  border-radius: 8px;  
  padding: 16px;  
  font-family: 'Inter', sans-serif;  
}  
```  
   
After applying `DESIGN.md` (CSS custom properties):  
```css  
.button {  
  background-color: var(--color-primary); /* resolves to #2665fd */  
  border-radius: var(--rounded-md);       /* resolves to 8px */  
  padding: var(--spacing-md);             /* resolves to 16px */  
  font-family: var(--font-family-body);   /* resolves to 'Inter', sans-serif */  
}  
```  
   
Where the actual output format depends on your target framework (CSS custom properties, Tailwind classes, styled-components, etc.). Choose the most natural mapping for the codebase.  
   
### 4. Enforce Do's and Don'ts  
   
- Read the `## Do's and Don'ts` section (or its aliases) from the markdown body.  
- Apply every statement as a hard rule:  
  - **Do** statements: you must actively follow them (e.g. "Do use the primary color only for the most important action" → never apply primary to secondary buttons).  
  - **Don't** statements: you must avoid those patterns (e.g. "Don't mix rounded and sharp corners in the same view" → ensure all interactive elements use the same corner rounding levels).  
- If a Do/Don't contradicts a token definition, the token wins for exact values, but the rule still guides when to use that token.  
   
### 5. Verify contrast and accessibility  
   
- For every component that defines **both** `backgroundColor` and `textColor`, compute the WCAG contrast ratio (relative luminance).  
- **WCAG AA threshold**: `4.5:1` for all text (the `@google/design.md` linter uses this fixed threshold; it does not differentiate large text).  
- If the ratio is below 4.5:1, add a warning to your output and suggest an alternative token (e.g. a darker background or lighter text from the same palette).  
- If `DESIGN.md` already contains a `contrast-ratio` warning from the linter, respect that and do not reintroduce poor contrast.  
- **Skip validation** if a component defines only `backgroundColor` or only `textColor` (e.g. transparent backgrounds, text overlays on images).  
   
**Contrast calculation reference:** WCAG relative luminance formula:    
https://www.w3.org/WAI/GL/wiki/Relative_luminance  
   
### 6. Handle missing tokens  
   
If a required token category is missing from `DESIGN.md`:  
- **Colors**: If no `colors` section exists, ask the user to define at least a `primary` color. Do not invent colors.  
- **Typography**: If no typography tokens exist, use system defaults but warn the user.  
- **Spacing / rounded**: If missing, ask the user to define them. Do not use arbitrary fallbacks.  
   
Never silently fall back to hardcoded values that are not in the design system.  
   
### 7. Semantic token usage  
   
- Prefer semantic usage: use `surface` for backgrounds, `on-surface` for text, `primary` for interactive elements, `error` for destructive actions, etc.  
- If semantic tokens like `on-surface` are not defined, **do not invent them**. Use only the tokens explicitly defined in `DESIGN.md`. If necessary, ask the user which token to use.  
- Do not apply a color token to a role it was not intended for (e.g. using `error` as a decorative background unless the design system explicitly allows it).  
   
### 8. Output format  
   
When you produce code:  
- Show the **before** (hardcoded) and **after** (tokenized) version for each modified component, unless the user asks for silent application.  
- If you cannot apply a specific rule (e.g. missing token), list those gaps explicitly.  
- Provide a summary of which tokens were used and which Do's/Don'ts were enforced.  
- If a specific DESIGN.md path was used, mention it in the summary.  
   
## Example workflows  
   
**User**: "Apply DESIGN.md to the login form component."  
   
1. Run `npx @google/design.md lint DESIGN.md` to validate the design system. If errors exist, stop.  
2. Read `DESIGN.md` from the project root.  
3. Parse YAML: `colors.primary = "#2665fd"`, `rounded.md = "8px"`, `spacing.md = "16px"`.  
4. Find the login form code (e.g. `LoginForm.tsx`).  
5. Replace hardcoded `#2665fd` with a reference to `colors.primary` (output as `var(--color-primary)` or `theme.colors.primary` depending on framework).  
6. Replace `borderRadius: '8px'` with `rounded.md`.  
7. Check Do's and Don'ts: the file says "Don't use primary for disabled buttons" – ensure no disabled button has primary background.  
8. Output the modified file and a report of changes.  
  
**User**: "Apply design system from ./docs/DESIGN.md to the header component."  
  
1. Extract path `./docs/DESIGN.md` from the user's request.  
2. Run `npx @google/design.md lint ./docs/DESIGN.md` to validate.  
3. Read `./docs/DESIGN.md` and proceed with token application.  
4. Output changes and note that the design system from `./docs/DESIGN.md` was used.  
   
## Edge cases  
   
    - **Multiple component variants**: If the design system defines `button-primary` and `button-secondary`, use the appropriate variant based on the component's semantic role.  
    - **No components section**: You can still apply colors, typography, spacing, and rounding globally. Do not invent component-specific overrides.  
    - **Inline styles vs CSS classes**: Adapt the skill to the existing code style. Prefer CSS custom properties or a design tokens file if the project already uses one.  
    - **Token references in references**: Resolve recursively until you reach primitive values (hex, dimension, etc.). Detect and break cycles.  
    - **Transparent or missing backgrounds**: If a component has `backgroundColor: transparent` or no `backgroundColor` defined, skip contrast validation for that component.  
    - **Invalid path**: If the user provides a path that doesn't exist or doesn't point to a valid DESIGN.md file, inform them and ask for the correct path.  
   
## Quality checklist  
   
Before finishing, verify:  
    - [ ] `DESIGN.md` passed validation (`npx @google/design.md lint <design_file_path>` reports 0 errors).  
    - [ ] Every hardcoded color, font size, spacing, and border radius has been replaced by a token or token reference.  
    - [ ] All token references resolve correctly (no broken references, no cycles).  
    - [ ] Do's are followed, Don'ts are avoided.  
    - [ ] Contrast ratios meet WCAG AA (4.5:1) for all components with both `backgroundColor` and `textColor`.  
    - [ ] You have warned the user about any missing tokens or unresolved references.  
    - [ ] If a specific path was provided, you used that exact file.  
   
## Integration with AGENTS.md  
   
If the project contains `AGENTS.md`, that file may define coding conventions (e.g. "use CSS modules", "prefer functional components"). This skill **reads** `AGENTS.md` to understand code structure preferences but **never modifies** it. The `DESIGN.md` takes precedence for **visual** rules; `AGENTS.md` for code structure. When both apply, combine them sensibly (e.g. `AGENTS.md` says "use CSS modules", `DESIGN.md` provides the token values – apply tokens inside CSS modules).  
   
## Further reading  
   
    - **Full specification**: https://github.com/google-labs-code/design.md/blob/main/docs/spec.md  
    - **CLI commands**: https://github.com/google-labs-code/design.md#cli-usage  
    - **Example design systems**: https://github.com/google-labs-code/design.md/tree/main/examples
