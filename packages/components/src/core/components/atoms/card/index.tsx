import type { HTMLAttributes } from 'react';
import { cn } from '../../../cn';

export type CardProps = HTMLAttributes<HTMLDivElement> & {
  padding?: 'md' | 'none';
};

export function Card({ padding = 'md', className, ...props }: CardProps) {
  return (
    <div
      {...props}
      className={cn('rounded-2xl bg-white shadow-soft', padding === 'md' && 'p-5', className)}
    />
  );
}
