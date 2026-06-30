import { useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { Button, Form, TextField, Input, Label, FieldError } from 'react-aria-components'
import { useTranslation } from 'react-i18next'
import { userClient, authClient } from '~/lib/api/client'
import { useAuthStore } from '~/store/auth'
import { detectLocale } from '~/i18n/detect-locale'

export function DeleteAccountPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const clearAuth = useAuthStore((s) => s.clearAuth)
  const [error, setError] = useState<string | null>(null)
  const [pending, setPending] = useState(false)

  async function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault()
    setError(null)
    setPending(true)
    const form = new FormData(e.currentTarget)
    try {
      await userClient.deleteAccount({ password: form.get('password') as string })
      await authClient.logout({})
      clearAuth()
      await navigate({ to: '/$locale/auth/login', params: { locale: detectLocale() } })
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Delete failed')
    } finally {
      setPending(false)
    }
  }

  return (
    <main>
      <h1>{t('account.delete.title', 'Delete account')}</h1>
      <p>{t('account.delete.warning', 'This action is permanent and cannot be undone.')}</p>
      <Form onSubmit={handleSubmit}>
        <TextField name="password" type="password" isRequired>
          <Label>{t('account.delete.password', 'Confirm password')}</Label>
          <Input autoComplete="current-password" />
          <FieldError />
        </TextField>
        {error && <p role="alert">{error}</p>}
        <Button type="submit" isDisabled={pending}>
          {pending ? t('account.delete.submitting', 'Deleting…') : t('account.delete.submit', 'Delete my account')}
        </Button>
      </Form>
    </main>
  )
}
