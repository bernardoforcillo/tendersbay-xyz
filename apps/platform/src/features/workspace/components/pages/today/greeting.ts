export type GreetingKey = 'morning' | 'afternoon' | 'evening';

export function greetingKey(hour: number): GreetingKey {
  if (hour < 12) return 'morning';
  if (hour < 18) return 'afternoon';
  return 'evening';
}
