import { useState } from 'react';
import { Button, Form, Input, Label, Switch, TextField } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import {
  hasBit,
  PERMISSION_BITS,
  PERMISSION_KEYS,
  toggleBit,
} from '~/features/workspace/permissions';
import { BTN_PRIMARY, BTN_SECONDARY, ERROR_BOX, INPUT, LABEL } from '~/features/workspace/ui';

type RoleEditorProps = {
  initialName?: string;
  initialPermissions?: bigint;
  submitting?: boolean;
  error?: string | null;
  /** Whether the current user may grant a given permission bit (server enforces too). */
  canGrant: (bit: bigint) => boolean;
  onSubmit: (name: string, permissions: bigint) => void;
  onCancel: () => void;
};

export function RoleEditor({
  initialName = '',
  initialPermissions = 0n,
  submitting,
  error,
  canGrant,
  onSubmit,
  onCancel,
}: RoleEditorProps) {
  const { t } = useTranslation();
  const [name, setName] = useState(initialName);
  const [perms, setPerms] = useState(initialPermissions);

  return (
    <Form
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit(name.trim(), perms);
      }}
      className="flex flex-col gap-4"
    >
      <TextField value={name} onChange={setName} isRequired className="flex flex-col gap-1.5">
        <Label className={LABEL}>{t('workspace.roles.nameLabel', 'Role name')}</Label>
        <Input className={INPUT} placeholder={t('workspace.roles.namePlaceholder', 'Editor')} />
      </TextField>

      <div className="flex flex-col gap-1">
        <span className={LABEL}>{t('workspace.roles.permissionsLabel', 'Permissions')}</span>
        <div className="mt-1 flex flex-col divide-y divide-cream-200 rounded-xl border border-cream-200">
          {PERMISSION_KEYS.map((key) => {
            const bit = PERMISSION_BITS[key];
            const grantable = canGrant(bit);
            return (
              <Switch
                key={key}
                isSelected={hasBit(perms, bit)}
                isDisabled={!grantable}
                onChange={(v) => setPerms(toggleBit(perms, bit, v))}
                className="group flex items-center justify-between gap-3 px-4 py-3 data-[disabled]:opacity-50"
              >
                {({ isSelected }) => (
                  <>
                    <span className="flex flex-col">
                      <span className="text-sm font-medium text-ink-800">
                        {t(`workspace.permissions.${key}`, key)}
                      </span>
                      <span className="text-xs text-ink-500">
                        {t(`workspace.permissionsHint.${key}`, '')}
                      </span>
                    </span>
                    <span
                      className={`flex h-5 w-9 shrink-0 items-center rounded-full px-0.5 transition-colors ${isSelected ? 'bg-brand-600' : 'bg-cream-300'}`}
                    >
                      <span
                        className={`block h-4 w-4 rounded-full bg-white shadow-sm transition-transform ${isSelected ? 'translate-x-4' : 'translate-x-0'}`}
                      />
                    </span>
                  </>
                )}
              </Switch>
            );
          })}
        </div>
      </div>

      {error && (
        <p role="alert" className={ERROR_BOX}>
          {error}
        </p>
      )}

      <div className="flex gap-3">
        <Button type="submit" isDisabled={submitting || name.trim() === ''} className={BTN_PRIMARY}>
          {submitting
            ? t('workspace.common.saving', 'Saving…')
            : t('workspace.common.save', 'Save')}
        </Button>
        <Button type="button" onPress={onCancel} className={BTN_SECONDARY}>
          {t('workspace.common.cancel', 'Cancel')}
        </Button>
      </div>
    </Form>
  );
}
