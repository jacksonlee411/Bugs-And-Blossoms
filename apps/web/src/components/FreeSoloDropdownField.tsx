import { Autocomplete, TextField } from '@mui/material'

interface FreeSoloDropdownFieldProps {
  label: string
  options: string[]
  value: string
  required?: boolean
  disabled?: boolean
  onChange: (nextValue: string) => void
}

export function uniqueNonEmptyStrings(values: string[]): string[] {
  return Array.from(
    new Set(
      values
        .map((value) => value.trim())
        .filter((value) => value.length > 0)
    )
  ).sort((left, right) => left.localeCompare(right))
}

export function mergeFreeSoloOptions(presets: string[], dynamic: string[], currentValue?: string): string[] {
  const current = (currentValue ?? '').trim()
  return uniqueNonEmptyStrings(current.length > 0 ? [...presets, ...dynamic, current] : [...presets, ...dynamic])
}

export function FreeSoloDropdownField(props: FreeSoloDropdownFieldProps) {
  return (
    <Autocomplete
      clearOnEscape
      disabled={props.disabled}
      freeSolo
      onChange={(_, nextValue) => {
        if (typeof nextValue === 'string') {
          props.onChange(nextValue)
          return
        }
        props.onChange('')
      }}
      onInputChange={(_, nextInputValue, reason) => {
        if (reason === 'input' || reason === 'clear') {
          props.onChange(nextInputValue)
        }
      }}
      options={props.options}
      value={props.value}
      renderInput={(params) => <TextField {...params} label={props.label} required={props.required} size='small' />}
    />
  )
}
