import { motion, AnimatePresence } from 'framer-motion'
import { useLocation } from 'react-router-dom'

const pageVariants = {
  initial: { opacity: 0, y: 6 },
  in: { opacity: 1, y: 0 },
  out: { opacity: 0, y: -6 },
}

const pageTransition = {
  type: 'tween' as const,
  ease: 'easeInOut' as const,
  duration: 0.18,
}

interface PageTransitionProps {
  children: React.ReactNode
}

export function PageTransition({ children }: PageTransitionProps) {
  const location = useLocation()
  return (
    <AnimatePresence mode="sync">
      <motion.div
        key={location.pathname}
        initial="initial"
        animate="in"
        exit="out"
        variants={pageVariants}
        transition={pageTransition}
        className="h-full"
      >
        {children}
      </motion.div>
    </AnimatePresence>
  )
}
