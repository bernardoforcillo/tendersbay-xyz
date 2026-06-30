import { useState } from 'react'
import { useNavigate, useParams } from '@tanstack/react-router'
import { Button, Form, TextField, Input, Label, FieldError } from 'react-aria-components'
import { useTranslation } from 'react-i18next'
import { authClient } from '~/lib/api/client'
import { useAuthStore } from '~/store/auth'

export function LoginPage() {
  const { locale } = useParams({ from: '/$locale/auth/login' })
  const navigate = useNavigate()
  const { t } = useTranslation()
  const setAuth = useAuthStore((s) => s.setAuth)
  const [error, setError] = useState<string | null>(null)
  const [pending, setPending] = useState(false)

  async function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault()
    setError(null)
    setPending(true)
    const form = new FormData(e.currentTarget)
    try {
      const res = await authClient.login({
        email: form.get('email') as string,
        password: form.get('password') as string,
      })
      setAuth(res.accessToken, {
        id: res.user!.id,
        email: res.user!.email,
        displayName: res.user!.displayName,
      })
      await navigate({ to: '/account/profile' })
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Login failed')
    } finally {
      setPending(false)
    }
  }

  return (
    <main>
      <h1>{t('auth.login.title', 'Sign in')}</h1>
      <Form onSubmit={handleSubmit}>
        <TextField name="email" type="email" isRequired>
          <Label>{t('auth.login.email', 'Email')}</Label>
          <Input autoComplete="email" />
          <FieldError />
        </TextField>
        <TextField name="password" type="password" isRequired>
          <Label>{t('auth.login.password', 'Password')}</Label>
          <Input autoComplete="current-password" />
          <FieldError />
        </TextField>
        {error && <p role="alert">{error}</p>}
        <Button type="submit" isDisabled={pending}>
          {pending ? t('auth.login.submitting', 'Signing in…') : t('auth.login.submit', 'Sign in')}
        </Button>
        <a href={`/${locale}/auth/forgot-password`}>{t('auth.login.forgotPassword', 'Forgot password?')}</a>
        <a href={`/${locale}/auth/signup`}>{t('auth.login.signUp', 'Create account')}</a>
      </Form>
    </main>
  )
}
