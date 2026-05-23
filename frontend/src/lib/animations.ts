import type { Variants, Transition } from 'framer-motion'

/** Fade + tiny lift — use for modals, cards, empty states, drawers */
export const subtleVariants: Variants = {
  hidden: { opacity: 0, y: 8, scale: 0.98 },
  visible: { opacity: 1, y: 0, scale: 1 },
}

/** Horizontal slide — use for step-based wizards and tab panels */
export const slideVariants: Variants = {
  enter: (dir: number) => ({ x: dir > 0 ? 36 : -36, opacity: 0 }),
  center: { x: 0, opacity: 1 },
  exit: (dir: number) => ({ x: dir < 0 ? 36 : -36, opacity: 0 }),
}

/** Staggered list container — wrap <ul> or list root */
export const listVariants: Variants = {
  hidden: {},
  visible: { transition: { staggerChildren: 0.045 } },
}

/** Staggered list item — wrap each <li> or card */
export const listItemVariants: Variants = {
  hidden: { opacity: 0, y: 6 },
  visible: { opacity: 1, y: 0 },
}

/** Standard transition presets */
export const transitions = {
  /** Springy, feels natural — default for most UI */
  gentle: { type: 'spring', stiffness: 300, damping: 30 } satisfies Transition,
  /** Snappier spring — for small widgets, badges, toggles */
  snappy: { type: 'spring', stiffness: 500, damping: 35 } satisfies Transition,
  /** Tween — for opacity-only fades */
  subtle: { type: 'tween', ease: 'easeOut', duration: 0.15 } satisfies Transition,
  /** Zero duration — opt out of animation for reduced-motion contexts */
  instant: { duration: 0 } satisfies Transition,
} as const
