import { describe, it, expect } from 'vitest'
import { render } from '@testing-library/react'
import { axe } from 'vitest-axe'

import { SkeletonTable, SkeletonCardGrid, SkeletonDetailPage, SkeletonStatCard } from './SkeletonLoaders'

describe('SkeletonLoaders a11y', () => {
  it('SkeletonTable has no a11y violations', async () => {
    const { container } = render(<SkeletonTable rows={3} cols={4} />)
    expect(await axe(container)).toHaveNoViolations()
  })

  it('SkeletonCardGrid has no a11y violations', async () => {
    const { container } = render(<SkeletonCardGrid count={3} />)
    expect(await axe(container)).toHaveNoViolations()
  })

  it('SkeletonDetailPage has no a11y violations', async () => {
    const { container } = render(<SkeletonDetailPage />)
    expect(await axe(container)).toHaveNoViolations()
  })

  it('SkeletonStatCard has no a11y violations', async () => {
    const { container } = render(<SkeletonStatCard />)
    expect(await axe(container)).toHaveNoViolations()
  })
})
