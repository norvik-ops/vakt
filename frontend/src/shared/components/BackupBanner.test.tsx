import { describe, it, expect } from 'vitest'
import { screen, fireEvent } from '@testing-library/react'
import { renderWithProviders } from '../../test-utils'
import { BackupBanner } from './BackupBanner'

// UX-10: Eine frische Instanz zeigte ab der ersten Minute „Kein Backup in den letzten
// 7 Tagen". Jetzt: Einrichtungshinweis, solange setup_pending; Warnung, sobald fällig.
describe('BackupBanner', () => {
  it('frische Instanz: freundlicher Hinweis „Backup einrichten" statt der Warnung', () => {
    renderWithProviders(<BackupBanner status={{ last_backup_at: null, stale: true, setup_pending: true }} />)
    expect(screen.getByTestId('backup-setup-hint')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Backup einrichten' })).toHaveAttribute('href', '/settings#system')
    expect(screen.queryByText(/Kein Backup in den letzten 7 Tagen/)).not.toBeInTheDocument()
  })

  it('fälliges Backup: die Warnung bleibt', () => {
    renderWithProviders(<BackupBanner status={{ last_backup_at: null, stale: true, setup_pending: false }} />)
    expect(screen.getByTestId('backup-stale-warning')).toBeInTheDocument()
    expect(screen.getByText(/Kein Backup in den letzten 7 Tagen/)).toBeInTheDocument()
    expect(screen.queryByTestId('backup-setup-hint')).not.toBeInTheDocument()
  })

  it('älteres Backend ohne setup_pending: Warnung wie bisher', () => {
    renderWithProviders(<BackupBanner status={{ last_backup_at: null, stale: true }} />)
    expect(screen.getByTestId('backup-stale-warning')).toBeInTheDocument()
  })

  it('aktuelles Backup: kein Banner', () => {
    const { container } = renderWithProviders(
      <BackupBanner status={{ last_backup_at: new Date().toISOString(), stale: false, setup_pending: false }} />,
    )
    expect(container).toBeEmptyDOMElement()
  })

  it('lässt sich wegklicken', () => {
    renderWithProviders(<BackupBanner status={{ last_backup_at: null, stale: true, setup_pending: true }} />)
    fireEvent.click(screen.getByRole('button', { name: /schließen|close/i }))
    expect(screen.queryByTestId('backup-setup-hint')).not.toBeInTheDocument()
  })
})
