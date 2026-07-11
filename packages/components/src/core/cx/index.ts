type CxArg = string | false | null | undefined;

/** Join truthy class-name fragments with single spaces. */
export function cx(...args: CxArg[]): string {
  return args.filter(Boolean).join(' ');
}
