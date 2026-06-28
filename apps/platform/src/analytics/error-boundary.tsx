import { Component, type ErrorInfo, type ReactNode } from 'react';
import { getAnalytics } from './posthog';

type Props = { children: ReactNode; fallback?: ReactNode };
type State = { hasError: boolean };

/** Captures React render errors into PostHog (no-op until consent), then shows `fallback`. */
export class AnalyticsErrorBoundary extends Component<Props, State> {
  state: State = { hasError: false };

  static getDerivedStateFromError(): State {
    return { hasError: true };
  }

  componentDidCatch(error: Error, info: ErrorInfo): void {
    getAnalytics()?.captureException(error, { componentStack: info.componentStack });
  }

  render(): ReactNode {
    if (this.state.hasError) {
      return this.props.fallback ?? null;
    }
    return this.props.children;
  }
}
