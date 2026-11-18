/**
 * Eine Liste „Aufgaben" je Kontrolle (Migration 273): Die frühere Checkliste
 * „Umsetzungsschritte" und die zugewiesenen Aufgaben sind ein Datenmodell.
 * Geprüft: alle Aufgaben in einer Liste, abhaken, Person/Frist optional,
 * Leser sieht die Liste ohne Knöpfe.
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, within } from '@testing-library/react'
import i18n from '../../../i18n'
import { TasksPanel } from './TasksPanel'
import { signInAs } from '../../../test-utils/signInAs'
import type { CollabTask } from '../types'

const createMutate = vi.fn()
const updateMutate = vi.fn()
const deleteMutate = vi.fn()
let tasks: CollabTask[] = []

vi.mock('../hooks/useTasks', () => ({
  useTasks: () => ({ data: tasks, isLoading: false }),
  useCreateTask: () => ({ mutate: createMutate, isPending: false }),
  useUpdateTask: () => ({ mutate: updateMutate, isPending: false }),
  useDeleteTask: () => ({ mutate: deleteMutate, isPending: false }),
}))

function task(over: Partial<CollabTask>): CollabTask {
  return {
    id: 't', org_id: 'o', entity_type: 'control', entity_id: 'ctl-1',
    title: 'x', description: '', assignee_email: '', due_date: null,
    status: 'open', priority: 'medium', created_by: '',
    created_at: '2026-11-01T10:00:00Z', updated_at: '2026-11-01T10:00:00Z',
    ...over,
  }
}

const k = (key: string) => i18n.t(`vaktcomply.tasksPanel.${key}`)

beforeEach(() => {
  vi.clearAllMocks()
  tasks = [
    // Übernommener Checklisten-Schritt: ohne Person, ohne Frist, erledigt.
    task({ id: 't1', title: 'Richtlinie entwerfen', status: 'done' }),
    // Zugewiesene Aufgabe mit Person und Frist.
    task({ id: 't2', title: 'Freigabe einholen', assignee_email: 'ciso@example.org', due_date: '2099-12-31', priority: 'high' }),
  ]
})

describe('TasksPanel — eine Liste „Aufgaben“', () => {
  it('zeigt Checklisten-Schritte und zugewiesene Aufgaben in derselben Liste', () => {
    signInAs('Admin')
    render(<TasksPanel entityType="control" entityId="ctl-1" />)
    const items = screen.getAllByRole('listitem')
    expect(items).toHaveLength(2)
    expect(within(items[0]).getByText('Richtlinie entwerfen')).toBeInTheDocument()
    expect(within(items[1]).getByText('Freigabe einholen')).toBeInTheDocument()
    expect(within(items[1]).getByText('ciso@example.org')).toBeInTheDocument()
    // Ohne Person/Frist: keine leeren Felder, keine „Mittel“-Plakette.
    expect(within(items[0]).queryByText(k('priorityMedium'))).toBeNull()
    expect(screen.getByText(k('openCount').replace('{{count}}', '1'))).toBeInTheDocument()
  })

  it('jede Aufgabe ist abhakbar — und wieder aufhakbar', () => {
    signInAs('SecurityAnalyst')
    render(<TasksPanel entityType="control" entityId="ctl-1" />)
    const boxes = screen.getAllByRole('checkbox')
    expect(boxes).toHaveLength(2)
    expect(boxes[0]).toHaveAttribute('aria-checked', 'true')
    expect(boxes[1]).toHaveAttribute('aria-checked', 'false')

    fireEvent.click(boxes[1])
    expect(updateMutate).toHaveBeenCalledWith({ taskId: 't2', entityType: 'control', entityId: 'ctl-1', status: 'done' })
    fireEvent.click(boxes[0])
    expect(updateMutate).toHaveBeenCalledWith({ taskId: 't1', entityType: 'control', entityId: 'ctl-1', status: 'open' })
  })

  it('anlegen nur mit Titel — Person und Frist bleiben optional', () => {
    signInAs('Admin')
    render(<TasksPanel entityType="control" entityId="ctl-1" />)
    // Person/Frist sind zugeklappt, der Titel reicht.
    expect(screen.queryByLabelText(k('labelAssignee'))).toBeNull()
    fireEvent.change(screen.getByLabelText(k('labelTitle')), { target: { value: '  Schulung planen ' } })
    fireEvent.click(screen.getByRole('button', { name: k('addButton') }))
    expect(createMutate).toHaveBeenCalledTimes(1)
    expect(createMutate.mock.calls[0][0]).toEqual({
      title: 'Schulung planen', assignee_email: undefined, due_date: undefined, priority: 'medium',
    })
  })

  it('anlegen mit Person und Frist, wenn aufgeklappt', () => {
    signInAs('Admin')
    render(<TasksPanel entityType="control" entityId="ctl-1" />)
    fireEvent.click(screen.getByRole('button', { name: k('details') }))
    fireEvent.change(screen.getByLabelText(k('labelTitle')), { target: { value: 'Audit vorbereiten' } })
    fireEvent.change(screen.getByLabelText(k('labelAssignee')), { target: { value: 'it@example.org' } })
    fireEvent.change(screen.getByLabelText(k('dueDate')), { target: { value: '2026-12-31' } })
    fireEvent.click(screen.getByRole('button', { name: k('addButton') }))
    expect(createMutate.mock.calls[0][0]).toMatchObject({
      title: 'Audit vorbereiten', assignee_email: 'it@example.org', due_date: '2026-12-31',
    })
  })

  it('leerer Titel legt nichts an', () => {
    signInAs('Admin')
    render(<TasksPanel entityType="control" entityId="ctl-1" />)
    expect(screen.getByRole('button', { name: k('addButton') })).toBeDisabled()
  })

  it.each(['Viewer', 'AuditorReadOnly'] as const)('%s sieht die Liste, aber keinen Knopf', (role) => {
    signInAs(role)
    render(<TasksPanel entityType="control" entityId="ctl-1" />)
    expect(screen.getAllByRole('listitem')).toHaveLength(2)
    expect(screen.getByText('Richtlinie entwerfen')).toBeInTheDocument()
    expect(screen.queryAllByRole('button')).toHaveLength(0)
    expect(screen.queryAllByRole('checkbox')).toHaveLength(0)
    expect(screen.queryByLabelText(k('labelTitle'))).toBeNull()
    // Der Erledigt-Zustand bleibt lesbar.
    expect(screen.getByLabelText(k('done'))).toBeInTheDocument()
  })

  it('leere Liste: Leser bekommt keinen Aufruf zum Anlegen', () => {
    tasks = []
    signInAs('Viewer')
    render(<TasksPanel entityType="control" entityId="ctl-1" />)
    expect(screen.getByText(k('emptyReader'))).toBeInTheDocument()
    expect(screen.queryByText(k('empty'))).toBeNull()
  })
})
