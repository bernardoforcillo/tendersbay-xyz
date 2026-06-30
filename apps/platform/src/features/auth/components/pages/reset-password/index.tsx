import { useState } from 'react'
import { useNavigate, useSearch, useParams } from '@tanstack/react-router'
import { Button, Form, TextField, Input, Label, FieldError } from 'react-aria-components'
import { useTranslation } from 'react-i18next'
import { authClient } from '~/lib/api/client'

export function ResetPasswordPage() {
  const { token } = useSearch({ from: '/$locale/auth/reset-password' }) as { token?: string }
  const { locale } = useParams({ from: '/$locale/auth/reset-password' })
  const navigate = useNavigate()
  const { t } = useTranslation()
  const [error, setError] = useState<string | null>(null)
  const [pending, setPending] = useState(false)

  async function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (!token) return
    setError(null)
    setPending(true)
    const form = new FormData(e.currentTarget)
    try {
      await authClient.resetPassword({
        token,
        newPassword: form.get('password') as string,
      })
      await navigate({ to: '/$locale/auth/login', params: { locale } })
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Reset failed')
    } finally {
      setPending(false)
    }
  }

  return (
    <main>
      <h1>{t('auth.resetPassword.title', 'Set new password')}</h1>
      <Form onSubmit={handleSubmit}>
        <TextField name="password" type="password" isRequired minLength={12}>
          <Label>{t('auth.resetPassword.password', 'New password')}</Label>
          <Input autoComplete="new-password" />
          <FieldError />
        </TextField>
        {error && <p role="alert">{error}</p>}
        <Button type="submit" isDisabled={pending || !token}>
          {pending ? t('auth.resetPassword.submitting', 'Saving…') : t('auth.resetPassword.submit', 'Set password')}
        </Button>
      </Form>
    </main>
  )
}
