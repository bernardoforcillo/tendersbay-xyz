import { createFileRoute } from '@tanstack/react-router';
import { LandingPage } from '~/feature/landing';

export const Route = createFileRoute('/$locale/')({
  component: LandingPage,
});
