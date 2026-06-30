import { useState } from 'react'
import { Button, Form, TextField, Input, Label, FieldError } from 'react-aria-components'
import { useTranslation } from 'react-i18next'
import { userClient } from '~/lib/api/client'
import { useAuthStore } from '~/store/auth'
import type { AuthUser } from '~/store/auth'

export function ProfilePage() {
  const { t } = useTranslation()
  const { user, setAuth, accessToken } = useAuthStore()
  const [error, setError] = useState<string | null>(null)
  const [pending, setPending] = useState(false)
  const [saved, setSaved] = useState(false)

  async function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault()
    setError(null)
    setPending(true)
    const form = new FormData(e.currentTarget)
    try {
      const res = await userClient.updateProfile({ displayName: form.get('displayName') as string })
      const updated: AuthUser = {
        id: res.user!.id,
        email: res.user!.email,
        displayName: res.user!.displayName,
      }
      setAuth(accessToken!, updated)
      setSaved(true)
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Update failed')
    } finally {
      setPending(false)
    }
  }

  return (
    <main>
      <h1>{t('account.profile.title', 'Profile')}</h1>
      <p>{user?.email}</p>
      <Form onSubmit={handleSubmit}>
        <TextField name="displayName" defaultValue={user?.displayName} isRequired>
          <Label>{t('account.profile.displayName', 'Display name')}</Label>
          <Input />
          <FieldError />
        </TextField>
        {error && <p role="alert">{error}</p>}
        {saved && <p>{t('account.profile.saved', 'Saved!')}</p>}
        <Button type="submit" isDisabled={pending}>
          {t('account.profile.submit', 'Save')}
        </Button>
      </Form>
    </main>
  )
}
