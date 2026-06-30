import { useState } from 'react'
import { useParams } from '@tanstack/react-router'
import { Button, Form, TextField, Input, Label, FieldError } from 'react-aria-components'
import { useTranslation } from 'react-i18next'
import { authClient } from '~/lib/api/client'

export function SignupPage() {
  const { locale } = useParams({ from: '/$locale/auth/signup' })
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
      await authClient.signUp({
        email: form.get('email') as string,
        password: form.get('password') as string,
        displayName: form.get('displayName') as string,
        locale,
      })
      setDone(true)
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Sign-up failed')
    } finally {
      setPending(false)
    }
  }

  if (done) {
    return (
      <main>
        <h1>{t('auth.signup.checkEmail', 'Check your email')}</h1>
        <p>{t('auth.signup.verifyPrompt', 'We sent you a verification link. Click it to activate your account.')}</p>
      </main>
    )
  }

  return (
    <main>
      <h1>{t('auth.signup.title', 'Create account')}</h1>
      <Form onSubmit={handleSubmit}>
        <TextField name="displayName" isRequired>
          <Label>{t('auth.signup.displayName', 'Name')}</Label>
          <Input autoComplete="name" />
          <FieldError />
        </TextField>
        <TextField name="email" type="email" isRequired>
          <Label>{t('auth.signup.email', 'Email')}</Label>
          <Input autoComplete="email" />
          <FieldError />
        </TextField>
        <TextField name="password" type="password" isRequired>
          <Label>{t('auth.signup.password', 'Password')}</Label>
          <Input autoComplete="new-password" />
          <FieldError />
        </TextField>
        {error && <p role="alert">{error}</p>}
        <Button type="submit" isDisabled={pending}>
          {pending ? t('auth.signup.submitting', 'Creating…') : t('auth.signup.submit', 'Create account')}
        </Button>
        <a href={`/${locale}/auth/login`}>{t('auth.signup.login', 'Already have an account?')}</a>
      </Form>
    </main>
  )
}
