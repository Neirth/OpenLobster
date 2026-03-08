// Copyright (c) OpenLobster contributors. See LICENSE for details.

import type { Component } from 'solid-js';
import { For, Show, createSignal } from 'solid-js';
import { createMutation, useQueryClient } from '@tanstack/solid-query';
import { useSkills } from '@openlobster/ui/hooks';
import { IMPORT_SKILL_MUTATION, DELETE_SKILL_MUTATION } from '@openlobster/ui/graphql/mutations';
import { client } from '../../graphql/client';
import AppShell from '../../components/AppShell/AppShell';
import { t } from '../../App';
import './SkillsView.css';

const SkillsView: Component = () => {
  const skills = useSkills(client);
  const queryClient = useQueryClient();
  let fileInputRef: HTMLInputElement | undefined;
  const [importError, setImportError] = createSignal('');
  const [confirmDelete, setConfirmDelete] = createSignal<string | null>(null);

  const importSkill = createMutation(() => ({
    mutationFn: async (base64Data: string) => {
      const res = await client.request(IMPORT_SKILL_MUTATION, { data: base64Data });
      const result = (res as { importSkill?: { success?: boolean; error?: string | null } })
        ?.importSkill;
      if (result && !result.success && result.error) {
        throw new Error(result.error);
      }
      return res;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['skills'] });
      setImportError('');
    },
    onError: (err: unknown) => {
      setImportError(err instanceof Error ? err.message : String(err));
    },
  }));

  const deleteSkill = createMutation(() => ({
    mutationFn: (name: string) => client.request(DELETE_SKILL_MUTATION, { name }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['skills'] });
      setConfirmDelete(null);
    },
  }));

  const handleFileChange = (e: Event) => {
    const file = (e.currentTarget as HTMLInputElement).files?.[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = () => {
      // readAsDataURL returns "data:application/zip;base64,UEsDBBQ..." — extract base64 part
      const dataUrl = reader.result as string;
      const base64 = dataUrl.includes(',') ? dataUrl.split(',')[1] ?? '' : dataUrl;
      if (!base64) {
        setImportError('Failed to read file');
        return;
      }
      importSkill.mutate(base64);
    };
    reader.readAsDataURL(file);
    (e.currentTarget as HTMLInputElement).value = '';
  };

  const triggerImport = () => fileInputRef?.click();

  return (
    <AppShell activeTab="skills">
      <input
        ref={fileInputRef}
        type="file"
        accept=".skill,.zip"
        style={{ display: 'none' }}
        onChange={handleFileChange}
      />
      <div class="skills-view">
        <Show when={skills.data && skills.data.length > 0}>
          <div class="skills-header">
            <div>
              <h1>{t('skills.capabilities')}</h1>
              <p>{t('skills.capabilitiesHint')}</p>
            </div>
            <button class="btn-import-skill" onClick={triggerImport}>
              <span class="material-symbols-outlined">upload</span>
              {t('skills.import')}
            </button>
          </div>
        </Show>

        <Show when={importError()}>
          <p class="skills-import-error">{importError()}</p>
        </Show>

        <Show when={!skills.isLoading && (!skills.data || skills.data.length === 0)}>
          <div class="skills-empty">
            <span class="material-symbols-outlined skills-empty-icon">auto_awesome</span>
            <p class="skills-empty-title">{t('skills.noSkillsTitle')}</p>
            <p class="skills-empty-hint">{t('skills.noSkillsHint')}</p>
            <button class="btn-import-skill" onClick={triggerImport}>
              <span class="material-symbols-outlined">upload</span>
              {t('skills.import')}
            </button>
          </div>
        </Show>

        <Show when={skills.data && skills.data.length > 0}>
          <div class="skills-section">
            <h2>{t('skills.skillsHeader')}</h2>
            <div class="skills-grid">
              <For each={skills.data}>
                {(skill) => (
                  <div class="skill-card">
                    <div class="skill-header">
                      <div class="skill-info">
                        <h3 class="skill-name">{skill.name}</h3>
                        <p class="skill-description">{skill.description}</p>
                      </div>
                    </div>
                    <Show
                      when={confirmDelete() === skill.name}
                      fallback={
                        <button
                          class="skill-delete-btn"
                          title={t('skills.delete')}
                          onClick={() => setConfirmDelete(skill.name)}
                        >
                          <span class="material-symbols-outlined">delete</span>
                        </button>
                      }
                    >
                      <div class="skill-delete-confirm">
                        <span class="skill-delete-confirm-label">{t('skills.deleteConfirm')}</span>
                        <button
                          class="skill-delete-confirm-yes"
                          onClick={() => deleteSkill.mutate(skill.name)}
                        >
                          {t('skills.deleteYes')}
                        </button>
                        <button
                          class="skill-delete-confirm-no"
                          onClick={() => setConfirmDelete(null)}
                        >
                          {t('skills.deleteNo')}
                        </button>
                      </div>
                    </Show>
                  </div>
                )}
              </For>
            </div>
          </div>
        </Show>
      </div>
    </AppShell>
  );
};

export default SkillsView;
