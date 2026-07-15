import type { Interceptor } from '@connectrpc/connect';
import { createClient } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { AgentService } from '@tendersbay/proto/agent/v1/agent_pb';
import { AuthService } from '@tendersbay/proto/auth/v1/auth_pb';
import { TenderService } from '@tendersbay/proto/tender/v1/tender_pb';
import { UserService } from '@tendersbay/proto/user/v1/user_pb';
import { WorkbenchService } from '@tendersbay/proto/workbench/v1/workbench_pb';
import { WorkspaceService } from '@tendersbay/proto/workspace/v1/workspace_pb';
import { useAuthStore } from '~/store/auth';

const authInterceptor: Interceptor = (next) => async (req) => {
  const token = useAuthStore.getState().accessToken;
  if (token) req.header.set('Authorization', `Bearer ${token}`);
  return next(req);
};

// Prefer runtime config injected by the Go server (window.__ENV__) so one image
// serves every environment; fall back to the build-time value (Vite/.env) in dev,
// then a localhost default so a missing value never crashes the RPC layer.
const baseUrl = window.__ENV__?.API_URL ?? import.meta.env.VITE_API_URL ?? 'http://localhost:8080';

const transport = createConnectTransport({
  baseUrl,
  fetch: (input, init) => globalThis.fetch(input, { ...init, credentials: 'include' }),
  interceptors: [authInterceptor],
});

export const agentClient = createClient(AgentService, transport);
export const authClient = createClient(AuthService, transport);
export const tenderClient = createClient(TenderService, transport);
export const userClient = createClient(UserService, transport);
export const workspaceClient = createClient(WorkspaceService, transport);
export const workbenchClient = createClient(WorkbenchService, transport);
