/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { zodResolver } from '@hookform/resolvers/zod'
import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import * as z from 'zod'

import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'

import { SettingsSection } from '../components/settings-section'
import { SettingsPageFormActions } from '../components/settings-page-context'
import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { useUpdateOption } from '../hooks/use-update-option'
import { safeNumberFieldProps } from '../utils/numeric-field'

const auditSchema = z.object({
  AuditEnabled: z.boolean(),
  AuditPerRequestMaxBytes: z.number().int().min(1024).max(10 * 1024 * 1024),
  AuditContentTTLDays: z.number().int().min(0).max(3650),
})

type AuditFormValues = z.infer<typeof auditSchema>

type AuditSettingsSectionProps = {
  defaultValues: AuditFormValues
}

export function AuditSettingsSection({
  defaultValues,
}: AuditSettingsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const form = useForm<AuditFormValues>({
    resolver: zodResolver(auditSchema),
    defaultValues,
  })

  useEffect(() => {
    form.reset(defaultValues)
  }, [defaultValues, form])

  const onSubmit = async (values: AuditFormValues) => {
    const updates = Object.entries(values).filter(
      ([key, value]) => value !== defaultValues[key as keyof AuditFormValues]
    )

    for (const [key, value] of updates) {
      await updateOption.mutateAsync({ key, value })
    }
  }

  return (
    <SettingsSection title={t('Audit Content Monitoring')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
          />

          <FormField
            control={form.control}
            name='AuditEnabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Audit master switch')}</FormLabel>
                  <FormDescription>
                    {t('Capture request inputs/outputs into logs_content')}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <FormField
            control={form.control}
            name='AuditPerRequestMaxBytes'
            render={({ field }) => (
              <FormItem data-settings-form-span='full'>
                <FormLabel>{t('Per-request max bytes')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={1024}
                    max={10 * 1024 * 1024}
                    {...safeNumberFieldProps(field)}
                  />
                </FormControl>
                <FormDescription>
                  {t('Segments over this budget are downgraded (BR-009)')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='AuditContentTTLDays'
            render={({ field }) => (
              <FormItem data-settings-form-span='full'>
                <FormLabel>{t('Content retention days')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={0}
                    max={3650}
                    {...safeNumberFieldProps(field)}
                  />
                </FormControl>
                <FormDescription>
                  {t('Records older than this are skipped by rescan (BR-013)')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
