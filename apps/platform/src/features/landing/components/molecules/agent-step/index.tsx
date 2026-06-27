import { Icon, type IconName } from '~/features/landing/components/atoms';

type AgentStepProps = { index: number; icon: IconName; title: string; body: string };

export function AgentStep({ index, icon, title, body }: AgentStepProps) {
  const label = String(index).padStart(2, '0');
  return (
    <div className="relative flex flex-col">
      <span className="font-mono text-sm font-semibold tracking-[0.2em] text-white/55">
        {label}
      </span>
      <span className="mt-4 inline-flex h-12 w-12 items-center justify-center rounded-2xl bg-white/10 text-[21px] text-white ring-1 ring-white/20">
        <Icon name={icon} />
      </span>
      <h3 className="mt-5 font-display text-xl text-white">{title}</h3>
      <p className="mt-2.5 text-[15px] leading-relaxed text-brand-50">{body}</p>
    </div>
  );
}
