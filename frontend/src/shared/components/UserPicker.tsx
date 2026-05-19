import { useTeamMembers } from '../../hooks/useTeam'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '../../components/ui/select'

interface UserPickerProps {
  value: string | undefined
  onChange: (name: string | undefined) => void
  placeholder?: string
  disabled?: boolean
}

const NOBODY_VALUE = '__nobody__'

export function UserPicker({
  value,
  onChange,
  placeholder = 'Verantwortlichen wählen',
  disabled = false,
}: UserPickerProps) {
  const { data: members, isLoading } = useTeamMembers()

  const selectValue = value && value.trim() !== '' ? value : NOBODY_VALUE

  function handleChange(selected: string) {
    if (selected === NOBODY_VALUE) {
      onChange(undefined)
    } else {
      onChange(selected)
    }
  }

  return (
    <Select value={selectValue} onValueChange={handleChange} disabled={disabled || isLoading}>
      <SelectTrigger className="bg-surface2 border-border text-primary">
        <SelectValue placeholder={placeholder} />
      </SelectTrigger>
      <SelectContent className="bg-surface2 border-border">
        <SelectItem value={NOBODY_VALUE} className="text-secondary italic">
          Niemand
        </SelectItem>
        {(members ?? []).map((member) => (
          <SelectItem key={member.id} value={member.name} className="text-primary">
            {member.name}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}
