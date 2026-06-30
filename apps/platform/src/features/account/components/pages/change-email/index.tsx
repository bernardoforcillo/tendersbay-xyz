import { useState } from 'react'
import { Button, Form, TextField, Input, Label, FieldError } from 'react-aria-components'
import { useTranslation } from 'react-i18next'
import { userClient } from '~/lib/api/client'
import { detectLocale } from '~/i18n/detect-locale'

export function ChangeEmailPage() {
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
      await userClient.changeEmail({
        newEmail: form.get('newEmail') as string,
        password: form.get('password') as string,
        locale: detectLocale(),
      })
      setDone(true)
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to change email')
    } finally {
      setPending(false)
    }
  }

  if (done) {
    return (
      <main>
        <h1>{t('account.changeEmail.checkEmail', 'Check your new email')}</h1>
        <p>{t('account.changeEmail.prompt', 'We sent a verification link to your new address.')}</p>
      </main>
    )
  }

  return (
    <main>
      <h1>{t('account.changeEmail.title', 'Change email')}</h1>
      <Form onSubmit={handleSubmit}>
        <TextField name="newEmail" type="email" isRequired>
          <Label>{t('account.changeEmail.newEmail', 'New email')}</Label>
          <Input autoComplete="email" />
          <FieldError />
        </TextField>
        <TextField name="password" type="password" isRequired>
          <Label>{t('account.changeEmail.password', 'Current password')}</Label>
          <Input autoComplete="current-password" />
          <FieldError />
        </TextField>
        {error && <p role="alert">{error}</p>}
        <Button type="submit" isDisabled={pending}>
          {pending ? t('account.changeEmail.submitting', 'Saving…') : t('account.changeEmail.submit', 'Change email')}
        </Button>
      </Form>
    </main>
  )
}
