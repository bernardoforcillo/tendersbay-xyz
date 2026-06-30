import { useEffect, useState } from 'react'
import { useNavigate, useSearch } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { authClient } from '~/lib/api/client'

export function VerifyEmailPage() {
  const { token, type } = useSearch({ from: '/$locale/auth/verify-email' }) as { token?: string; type?: string }
  const navigate = useNavigate()
  const { t } = useTranslation()
  const [status, setStatus] = useState<'loading' | 'success' | 'error'>('loading')

  useEffect(() => {
    if (!token || !type) {
      setStatus('error')
      return
    }
    authClient
      .verifyEmail({ token, type })
      .then(async () => {
        setStatus('success')
        await new Promise((r) => setTimeout(r, 2000))
        await navigate({ to: '/account/profile' })
      })
      .catch(() => setStatus('error'))
  }, [token, type])

  if (status === 'loading') return <p>{t('auth.verifyEmail.loading', 'Verifying…')}</p>
  if (status === 'error')
    return <p role="alert">{t('auth.verifyEmail.error', 'This link is invalid or has expired.')}</p>
  return <p>{t('auth.verifyEmail.success', 'Email verified! Redirecting…')}</p>
}
