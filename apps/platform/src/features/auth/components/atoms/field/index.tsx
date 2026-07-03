import { FieldError, Input, Label, TextField } from 'react-aria-components';

type FieldProps = {
  name: string;
  label: string;
  type?: string;
  defaultValue?: string;
  autoComplete?: string;
  isRequired?: boolean;
};

export function Field({
  name,
  label,
  type = 'text',
  defaultValue,
  autoComplete,
  isRequired,
}: FieldProps) {
  return (
    <TextField
      name={name}
      type={type}
      defaultValue={defaultValue}
      isRequired={isRequired}
      className="flex flex-col gap-1.5"
    >
      <Label className="text-sm font-medium text-ink-700">{label}</Label>
      <Input
        autoComplete={autoComplete}
        className="w-full rounded-xl border border-cream-300 bg-cream-50 px-3.5 py-2.5 text-sm text-ink-900 outline-none transition placeholder:text-ink-300 focus:border-brand-400 focus:ring-2 focus:ring-brand-100"
      />
      <FieldError className="text-xs text-red-600" />
    </TextField>
  );
}
