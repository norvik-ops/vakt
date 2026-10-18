import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter, Routes, Route, useLocation } from 'react-router-dom'
import { LegacyAssetRedirect } from './LegacyAssetRedirect'

function Where() {
  return <p data-testid="where">{useLocation().pathname}</p>
}

function renderAt(path: string) {
  render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/vaktscan/assets" element={<LegacyAssetRedirect />} />
        <Route path="/vaktscan/assets/:id" element={<LegacyAssetRedirect />} />
        <Route path="*" element={<Where />} />
      </Routes>
    </MemoryRouter>,
  )
}

describe('LegacyAssetRedirect (module-archivieren S1)', () => {
  it('leitet die alte Inventarliste nach Comply um', () => {
    renderAt('/vaktscan/assets')
    expect(screen.getByTestId('where')).toHaveTextContent('/vaktcomply/assets')
  })

  it('behält die Asset-ID beim Umleiten', () => {
    renderAt('/vaktscan/assets/1234')
    expect(screen.getByTestId('where')).toHaveTextContent('/vaktcomply/assets/1234')
  })
})
