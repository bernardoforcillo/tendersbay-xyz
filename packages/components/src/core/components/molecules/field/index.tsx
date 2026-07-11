import {
  FieldError,
  Input,
  Label,
  Text,
  TextField,
  type TextFieldProps,
} from 'react-aria-components';
import { cn } from '../../../cn';

export type FieldProps = Omit<TextFieldProps, 'className' | 'children'> & {
  label: string;
  description?: string;
  errorMessage?: string;
  placeholder?: string;
  className?: string;
};

export function Field({
  label,
  description,
  errorMessage,
  placeholder,
  className,
  ...props
}: FieldProps) {
  return (
    <TextField
      {...props}
      isInvalid={props.isInvalid ?? Boolean(errorMessage)}
      className={cn('flex flex-col gap-1.5', className)}
    >
      <Label className="text-sm font-medium text-ink-700">{label}</Label>
      <Input
        placeholder={placeholder}
        className={cn(
          'h-10 rounded-xl border border-cream-300 bg-white px-3 text-sm text-ink-900 outline-none',
          'transition-colors duration-150 placeholder:text-ink-300',
          'data-[focused]:border-brand-600 data-[focused]:ring-2 data-[focused]:ring-brand-600/25',
          'data-[invalid]:border-red-500',
        )}
      />
      {description && !errorMessage && (
        <Text slot="description" className="text-xs text-ink-500">
          {description}
        </Text>
      )}
      <FieldError className="text-xs font-medium text-red-600 empty:hidden">
        {errorMessage}
      </FieldError>
    </TextField>
  );
}
