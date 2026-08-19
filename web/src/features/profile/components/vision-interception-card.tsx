/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { Eye, Loader2 } from 'lucide-react'
import { useEffect, useId, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { TitledCard } from '@/components/ui/titled-card'
import { useAuthStore } from '@/stores/auth-store'

import { updateUserVisionSettings } from '../api'
import { normalizeVisionPrompt, parseUserSettings } from '../lib'
import type { UserProfile } from '../types'

type VisionInterceptionCardProps = {
  profile: UserProfile | null
  onProfileUpdate: () => void
}

export function VisionInterceptionCard(props: VisionInterceptionCardProps) {
  const { t } = useTranslation()
  const { auth } = useAuthStore()
  const [saving, setSaving] = useState(false)
  const visionModelId = useId()
  const visionSuffixId = useId()
  const promptTemplateId = useId()
  const phashThresholdId = useId()
  const phashThresholdDescriptionId = useId()

  const savedVision = useMemo(() => {
    const settings = parseUserSettings(props.profile?.setting)
    return settings.vision ?? null
  }, [props.profile?.setting])

  const [enabled, setEnabled] = useState(savedVision?.enabled ?? false)
  const [visionModel, setVisionModel] = useState(
    savedVision?.vision_model ?? ''
  )
  const [visionSuffix, setVisionSuffix] = useState(
    savedVision?.vision_suffix ?? ''
  )
  const [promptTemplate, setPromptTemplate] = useState(
    normalizeVisionPrompt(savedVision?.prompt_template)
  )
  const [phashThreshold, setPhashThreshold] = useState(
    savedVision?.phash_threshold ?? 0
  )

  useEffect(() => {
    setEnabled(savedVision?.enabled ?? false)
    setVisionModel(savedVision?.vision_model ?? '')
    setVisionSuffix(savedVision?.vision_suffix ?? '')
    setPromptTemplate(normalizeVisionPrompt(savedVision?.prompt_template))
    setPhashThreshold(savedVision?.phash_threshold ?? 0)
  }, [savedVision])

  const handleSave = async () => {
    if (saving) {
      return
    }
    setSaving(true)
    const effectivePromptTemplate = normalizeVisionPrompt(promptTemplate)
    try {
      const response = await updateUserVisionSettings({
        vision: {
          enabled,
          vision_model: visionModel,
          vision_suffix: visionSuffix,
          prompt_template: effectivePromptTemplate,
          phash_threshold: phashThreshold,
        },
      })
      if (!response.success) {
        throw new Error(response.message || t('Failed to update settings'))
      }

      if (auth.user) {
        const existingSetting =
          typeof auth.user.setting === 'string'
            ? parseUserSettings(auth.user.setting)
            : (auth.user.setting ?? {})
        auth.setUser({
          ...auth.user,
          setting: JSON.stringify({
            ...existingSetting,
            vision: {
              enabled,
              vision_model: visionModel,
              vision_suffix: visionSuffix,
              prompt_template: effectivePromptTemplate,
              phash_threshold: phashThreshold,
            },
          }),
        })
      }

      setPromptTemplate(effectivePromptTemplate)
      props.onProfileUpdate()
      toast.success(t('Vision settings saved'))
    } catch {
      toast.error(t('Failed to update settings'))
    } finally {
      setSaving(false)
    }
  }

  return (
    <TitledCard
      title={t('Vision Interception')}
      description={t(
        'Replace images with text descriptions when the model matches the configured suffix.'
      )}
      icon={<Eye className='h-4 w-4' aria-hidden='true' />}
      iconTone='chart-2'
      disableHoverEffect
    >
      <form
        className='flex flex-col gap-4'
        aria-busy={saving}
        onSubmit={(event) => {
          event.preventDefault()
          void handleSave()
        }}
      >
        <div className='flex items-center justify-between gap-4'>
          <div className='space-y-1'>
            <div className='text-sm font-medium'>
              {t('Enable Vision Interception')}
            </div>
            <p className='text-muted-foreground line-clamp-2 text-xs sm:text-sm'>
              {t(
                'Images in matching requests are replaced with descriptions generated by the configured vision model.'
              )}
            </p>
          </div>
          <Switch
            aria-label={t('Enable Vision Interception')}
            checked={enabled}
            onCheckedChange={setEnabled}
          />
        </div>

        <div className='grid grid-cols-1 gap-3 sm:grid-cols-2'>
          <div className='space-y-1'>
            <label className='text-sm font-medium' htmlFor={visionModelId}>
              {t('Vision Model')}
            </label>
            <Input
              id={visionModelId}
              value={visionModel}
              onChange={(e) => setVisionModel(e.target.value)}
              placeholder={t('Vision model name')}
            />
          </div>
          <div className='space-y-1'>
            <label className='text-sm font-medium' htmlFor={visionSuffixId}>
              {t('Model Suffix')}
            </label>
            <Input
              id={visionSuffixId}
              value={visionSuffix}
              onChange={(e) => setVisionSuffix(e.target.value)}
              placeholder={t('e.g. -vision')}
            />
          </div>
        </div>

        <div className='space-y-1'>
          <label className='text-sm font-medium' htmlFor={promptTemplateId}>
            {t('Prompt Template')}
          </label>
          <Textarea
            id={promptTemplateId}
            rows={9}
            value={promptTemplate}
            onChange={(e) => setPromptTemplate(e.target.value)}
          />
        </div>

        <div className='space-y-1'>
          <label className='text-sm font-medium' htmlFor={phashThresholdId}>
            {t('pHash Threshold')}
          </label>
          <Input
            id={phashThresholdId}
            type='number'
            min={0}
            max={64}
            step={1}
            value={phashThreshold}
            aria-describedby={phashThresholdDescriptionId}
            onChange={(e) => setPhashThreshold(Number(e.target.value))}
          />
          <p
            id={phashThresholdDescriptionId}
            className='text-muted-foreground text-xs'
          >
            {t(
              'Hamming distance for near-duplicate image clustering. 0 disables perceptual deduplication.'
            )}
          </p>
        </div>

        <div className='flex items-center justify-end gap-2'>
          <Button type='submit' size='lg' disabled={saving}>
            {saving && (
              <Loader2
                className='animate-spin'
                data-icon='inline-start'
                aria-hidden='true'
              />
            )}
            {t('Save')}
          </Button>
        </div>
      </form>
    </TitledCard>
  )
}
