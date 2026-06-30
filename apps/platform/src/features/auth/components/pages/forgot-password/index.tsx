import { useState } from 'react'
import { useParams } from '@tanstack/react-router'
import { Button, Form, TextField, Input, Label, FieldError } from 'react-aria-components'
import { useTranslation } from 'react-i18next'
import { authClient } from '~/lib/api/client'

export function ForgotPasswordPage() {
  const { locale } = useParams({ from: '/$locale/auth/forgot-password' })
  const { t } = useTranslation()
  const [done, setDone] = useState(false)
  const [pending, setPending] = useState(false)

  async function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault()
    setPending(true)
    const form = new FormData(e.currentTarget)
    try {
      await authClient.forgotPassword({ email: form.get('email') as string, locale })
    } finally {
      // Always show "check email" to avoid email enumeration
      setDone(true)
      setPending(false)
    }
  }

  if (done) {
    return (
      <main>
        <h1>{t('auth.forgotPassword.checkEmail', 'Check your email')}</h1>
        <p>{t('auth.forgotPassword.prompt', 'If an account exists, we sent a reset link.')}</p>
      </main>
    )
  }

  return (
    <main>
      <h1>{t('auth.forgotPassword.title', 'Reset password')}</h1>
      <Form onSubmit={handleSubmit}>
        <TextField name="email" type="email" isRequired>
          <Label>{t('auth.forgotPassword.email', 'Email')}</Label>
          <Input autoComplete="email" />
          <FieldError />
        </TextField>
        <Button type="submit" isDisabled={pending}>
          {pending ? t('auth.forgotPassword.submitting', 'Sending…') : t('auth.forgotPassword.submit', 'Send reset link')}
        </Button>
        <a href={`/${locale}/auth/login`}>{t('auth.forgotPassword.back', 'Back to sign in')}</a>
      </Form>
    </main>
  )
}
