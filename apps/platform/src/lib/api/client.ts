import type { Interceptor } from '@connectrpc/connect';
import { createClient } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { AuthService } from '@tendersbay/proto/auth/v1/auth_pb';
import { UserService } from '@tendersbay/proto/user/v1/user_pb';
import { useAuthStore } from '~/store/auth';

const authInterceptor: Interceptor = (next) => async (req) => {
  const token = useAuthStore.getState().accessToken;
  if (token) req.header.set('Authorization', `Bearer ${token}`);
  return next(req);
};

const transport = createConnectTransport({
  baseUrl: import.meta.env.VITE_API_URL,
  fetch: (input, init) => globalThis.fetch(input, { ...init, credentials: 'include' }),
  interceptors: [authInterceptor],
});

export const authClient = createClient(AuthService, transport);
export const userClient = createClient(UserService, transport);
