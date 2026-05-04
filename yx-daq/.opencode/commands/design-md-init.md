---
description: Create or import DESIGN.md (no arguments — interactive survey, 'code' — extract from codebase, path to JSON — import Figma tokens)
agent: design-md-wizard
---
 
Execute the creation or update of DESIGN.md based on the argument `$ARGUMENTS`.
 
## Route detection (MUST follow strictly)
 
- If `$ARGUMENTS` is empty or exactly `""` → **interactive survey mode**.
- If `$ARGUMENTS` equals `code` → **codebase analysis mode**.
- If `$ARGUMENTS` is a non-empty path that does **not** equal `code` and the file **exists** → **Figma token import mode**.
- Otherwise → remind the user of valid options: no argument, `code`, or a path to an existing Figma JSON file.
 
## CRITICAL: No survey questions in code or import modes
 
- In **interactive survey mode** only, use the `question` tool to ask the five questions one by one.
- In **codebase analysis mode** and **Figma import mode**, do **NOT** ask any of those five questions. Proceed directly with the analysis or import workflow.
 
## Interactive survey mode (no arguments)
 
Ask the following five questions one at a time using the `question` tool. Wait for each answer before proceeding.
 
The questions:
 
1. **Aesthetic & personality** – Describe the overall look and feel. (e.g., "minimal and professional", "playful with warm colors", "dark & moody", "enterprise", "brutalist")
2. **Color palette** – What are the primary brand colors? Provide hex codes if known, or describe the scheme (e.g., "deep navy primary, coral accent").
3. **Typography** – Which font families should be used for headlines and body text? (e.g., Inter, Public Sans, system-ui) Mention any weight preferences.
4. **Spacing** – What base spacing unit and scale do you prefer? (e.g., 4px scale, 8px base padding, 16px gutters)
5. **Shapes & components** – How rounded should UI elements be (none, small, medium, pill)? Any component preferences (button style, card elevation, input borders)?
 
After all answers, generate a complete DESIGN.md conforming to the specification defined in the `design-md-wizard` agent.
 
## Codebase analysis mode (`code`)
 
Immediately scan the repository for existing design tokens, colors, typography, spacing, radii, and component patterns (stylesheets, Tailwind config, theme files, component code).
 
Produce a DESIGN.md that captures the existing design language.
 
If conflicting patterns are found, prompt the user **only** for clarification on those conflicts – do **not** fall back to the five survey questions.
 
## Figma import mode (path to JSON)
 
Read the given Figma token JSON file and convert it to the DESIGN.md schema (YAML tokens + markdown documentation).
 
Present the conversion result to the user for approval before writing the file.
 
Do **not** ask the five survey questions.
 
## Post-creation steps (all modes)
 
1. Validate the generated DESIGN.md: `npx @google/design.md lint DESIGN.md`
2. Fix any lint errors and re-validate until `errors` reach 0.
3. Update **AGENTS.md**:
   - Check if `AGENTS.md` exists in the project root.
   - Locate or create a block delimited by `<!-- DESIGN_SYSTEM_START -->` and `<!-- DESIGN_SYSTEM_END -->`.
   - Insert or replace a brief summary: design system name, primary color, typography, spacing scale, corner radius, and a link to `DESIGN.md`.
   - If `AGENTS.md` does not exist, create it with a title `# Project Instructions for AI Agents` and the design system block below it.
   - Ensure the block instructs other AI agents to always reference `DESIGN.md` when generating UI.
