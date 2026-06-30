import { useState } from 'react'
import { Button, Form, TextField, Input, Label, FieldError } from 'react-aria-components'
import { useTranslation } from 'react-i18next'
import { userClient } from '~/lib/api/client'

export function ChangePasswordPage() {
  const { t } = useTranslation()
  const [error, setError] = useState<string | null>(null)
  const [done, setDone] = useState(false)
  const [pending, setPending] = useState(false)

  async function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault()
    setError(null)
    setPending(true)
    const form = new FormData(e.currentTarget)
    try {
      await userClient.changePassword({
        currentPassword: form.get('currentPassword') as string,
        newPassword: form.get('newPassword') as string,
      })
      setDone(true)
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to change password')
    } finally {
      setPending(false)
    }
  }

  if (done) return <p>{t('account.changePassword.success', 'Password updated. Please sign in again.')}</p>

  return (
    <main>
      <h1>{t('account.changePassword.title', 'Change password')}</h1>
      <Form onSubmit={handleSubmit}>
        <TextField name="currentPassword" type="password" isRequired>
          <Label>{t('account.changePassword.current', 'Current password')}</Label>
          <Input autoComplete="current-password" />
          <FieldError />
        </TextField>
        <TextField name="newPassword" type="password" isRequired minLength={12}>
          <Label>{t('account.changePassword.new', 'New password')}</Label>
          <Input autoComplete="new-password" />
          <FieldError />
        </TextField>
        {error && <p role="alert">{error}</p>}
        <Button type="submit" isDisabled={pending}>
          {pending ? t('account.changePassword.submitting', 'Saving…') : t('account.changePassword.submit', 'Change password')}
        </Button>
      </Form>
    </main>
  )
}
