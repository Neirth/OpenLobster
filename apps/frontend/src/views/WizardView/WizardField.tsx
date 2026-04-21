// Copyright (c) OpenLobster contributors. See LICENSE for details.

import type { Component, JSX } from 'solid-js';

interface WizardFieldProps {
  label: string;
  description?: string;
  children: JSX.Element;
  error?: string;
  vertical?: boolean;
}

/**
 * WizardField is a dedicated row component for the FirstBootWizard.
 * It enforces a strict "Sandwich" layout. In standard mode, it uses a 
 * 1fr 320px grid. In vertical mode, it stacks Title, Description, and Field.
 */
export const WizardField: Component<WizardFieldProps> = (props) => {
  return (
    <div 
      class="wizard-field"
      classList={{ "wizard-field--vertical": props.vertical }}
    >
      <div class="field-info">
        <label class="field-label">{props.label}</label>
        {props.description && <span class="field-description">{props.description}</span>}
      </div>
      <div class="field-control">
        {props.children}
        {props.error && <span class="field-error-text">{props.error}</span>}
      </div>
    </div>
  );
};
