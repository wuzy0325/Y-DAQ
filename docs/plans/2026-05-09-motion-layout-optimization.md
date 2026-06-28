# Motion Layout Optimization Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Improve the motion controller page layout so operators can read status, connect safely, and control axes with less visual clutter.

**Architecture:** This is a frontend-only layout refinement. Keep all existing motion store APIs, Wails calls, and control behavior unchanged; modify only Vue templates and scoped SCSS in the motion view and axis card.

**Tech Stack:** Vue 3 `<script setup>`, TypeScript, Element Plus, scoped SCSS, existing YX-DAQ Neon design tokens.

---

### Task 1: Motion View Shell

**Files:**
- Modify: `frontend/src/views/MotionView.vue`

**Step 1: Reorganize the status bar template**

Group the header into clear zones: identity/status, connection fields, and safety/actions. Do not rename existing functions or store fields.

**Step 2: Update scoped SCSS**

Use the existing spacing rhythm (`8/12/16/24px`) and SCSS variables. Make the status bar wrap cleanly and keep the emergency stop visually dominant.

**Step 3: Adjust log presentation**

Keep the log collapsible, reduce its default visual weight, and prevent it from competing with axis controls.

**Step 4: Verify**

Run: `cd frontend && npm run build`

Expected: TypeScript and Vite build succeed.

### Task 2: Axis Control Card Layout

**Files:**
- Modify: `frontend/src/components/MotionControl/AxisControlCard.vue`

**Step 1: Rebalance visual hierarchy**

Keep the axis header, current position, input controls, and movement buttons, but separate them into clearer sections.

**Step 2: Improve button layout**

Make point movement buttons symmetric, make run/stop actions easy to distinguish, and keep zeroing as a secondary action.

**Step 3: Improve responsive behavior**

Ensure card content remains usable in the single-column mobile layout.

**Step 4: Verify**

Run: `cd frontend && npm run build`

Expected: TypeScript and Vite build succeed.

### Task 3: Final Validation

**Files:**
- Review: `frontend/src/views/MotionView.vue`
- Review: `frontend/src/components/MotionControl/AxisControlCard.vue`

**Step 1: Review behavior preservation**

Confirm all event handlers remain wired: connect, disconnect, emergency stop, configure axis, target change, relative change, jog start/stop, move to target, stop, home.

**Step 2: Run final build**

Run: `cd frontend && npm run build`

Expected: Build succeeds.
