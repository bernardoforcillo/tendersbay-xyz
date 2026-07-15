export type CreditsData = {
  remaining: number;
  monthlyMax: number;
  inputTokens: number;
  outputTokens: number;
};

type CreditDisplayProps = CreditsData & {
  onClose?: () => void;
};

export function CreditDisplay({
  remaining,
  monthlyMax,
  inputTokens,
  outputTokens,
  onClose,
}: CreditDisplayProps) {
  const pct = monthlyMax > 0 ? Math.round((remaining / monthlyMax) * 100) : 0;

  return (
    <div className="flex items-center justify-between gap-4 rounded-xl bg-cream-100 px-4 py-2 text-xs text-ink-500">
      <div className="flex items-center gap-2">
        <span className="font-medium">Credits</span>
        <div className="h-1.5 w-24 overflow-hidden rounded-full bg-cream-300">
          <div
            className="h-full rounded-full bg-brand-500 transition-all"
            style={{ width: `${pct}%` }}
          />
        </div>
        <span>
          {remaining.toLocaleString()} / {monthlyMax.toLocaleString()}
        </span>
      </div>
      {(inputTokens > 0 || outputTokens > 0) && (
        <span className="text-ink-400">+{inputTokens + outputTokens} tokens</span>
      )}
      {onClose && (
        <button type="button" onClick={onClose} className="text-ink-400 hover:text-ink-600">
          &times;
        </button>
      )}
    </div>
  );
}
