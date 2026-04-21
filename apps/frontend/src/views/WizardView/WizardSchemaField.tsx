// Copyright (c) OpenLobster contributors. See LICENSE for details.

import { createMemo, Show, For, type Component } from "solid-js";
import { t } from "@/App";
import type { SchemaProperty } from "@/schemas/config.schema";
import { getSchemaFieldI18nKey } from "@/schemas/config.schema";
import { WizardField } from "./WizardField";

export interface WizardSchemaFieldProps {
  field: string;
  schema: SchemaProperty;
  values: Record<string, unknown>;
  onChange: (field: string, value: unknown) => void;
}

function getValueAtPath(values: Record<string, unknown>, path: string): unknown {
  const parts = path.split(".");
  let current: unknown = values;
  for (const part of parts) {
    if (current && typeof current === "object") {
      current = (current as Record<string, unknown>)[part];
    } else {
      return undefined;
    }
  }
  return current;
}

/**
 * Specialized version of SchemaField for the FirstBootWizard.
 * Enforces a strictly vertical "Sandwich" layout (Title -> Description -> Field)
 * and ensures proper vertical margins through the WizardField container.
 */
export const WizardSchemaField: Component<WizardSchemaFieldProps> = (props) => {
  const fieldValue = createMemo(() => getValueAtPath(props.values, props.field));
  
  const titleKey = () => getSchemaFieldI18nKey(props.field, false);
  const descKey = () => getSchemaFieldI18nKey(props.field, true);
  
  const displayTitle = () => {
    const key = titleKey();
    const translated = t(key);
    if (translated && translated !== key) return translated;
    if (props.schema.title) return props.schema.title;

    const parts = props.field.split(".");
    const lastPart = parts[parts.length - 1];
    const fallback = lastPart
      .replace(/([A-Z])/g, " $1")
      .replace(/_/g, " ")
      .replace(/^./, (str) => str.toUpperCase())
      .trim();
    
    return fallback || key;
  };

  const displayDescription = () => {
    const key = descKey();
    const translated = t(key);
    if (translated && translated !== key) return translated;
    
    // If not translated, use schema description or a smart fallback
    if (props.schema.description) return props.schema.description;
    
    // Smart Fallback: Generate a readable placeholder description
    const title = displayTitle();
    return t("wizard.common.fieldConfigPlaceholder", { name: title }) || `Configure the ${title} parameter`;
  };

  const handleChange = (value: unknown) => {
    props.onChange(props.field, value);
  };

  return (
    <WizardField 
      label={displayTitle()} 
      description={displayDescription()}
      vertical={props.schema.type === "boolean"}
    >
      {/* Boolean / Toggle */}
      <Show when={props.schema.type === "boolean"}>
        <label class="toggle-switch">
          <input
            type="checkbox"
            checked={(fieldValue() as boolean) ?? (props.schema.default as boolean) ?? false}
            onChange={(e) => handleChange(e.currentTarget.checked)}
          />
          <span class="toggle-slider" />
        </label>
      </Show>

      {/* Enum / Select */}
      <Show when={props.schema.type === "string" && props.schema.enum}>
        <select
          class="field-select"
          value={(fieldValue() as string) ?? (props.schema.default as string) ?? ""}
          onChange={(e) => handleChange(e.currentTarget.value)}
        >
          <For each={props.schema.enum}>
            {(option) => <option value={option}>{option}</option>}
          </For>
        </select>
      </Show>

      {/* String Input */}
      <Show when={
        props.schema.type === "string" && 
        !props.schema.enum && 
        props.schema.format !== "textarea" &&
        props.schema.format !== "password"
      }>
        <input
          type="text"
          class="field-input"
          value={(fieldValue() as string) ?? (props.schema.default as string) ?? ""}
          placeholder={displayTitle()}
          onInput={(e) => handleChange(e.currentTarget.value)}
        />
      </Show>

      {/* Password Input */}
      <Show when={props.schema.type === "string" && props.schema.format === "password"}>
        <input
          type="password"
          class="field-input"
          value={(fieldValue() as string) ?? ""}
          placeholder={displayTitle()}
          onInput={(e) => handleChange(e.currentTarget.value)}
        />
      </Show>

      {/* Textarea */}
      <Show when={props.schema.type === "string" && props.schema.format === "textarea"}>
        <textarea
          class="field-textarea"
          value={(fieldValue() as string) ?? (props.schema.default as string) ?? ""}
          placeholder={displayTitle()}
          onInput={(e) => handleChange(e.currentTarget.value)}
        />
      </Show>

      {/* Integer Input */}
      <Show when={props.schema.type === "integer"}>
        <input
          type="number"
          class="field-input"
          value={(fieldValue() as number) ?? (props.schema.default as number) ?? 0}
          onInput={(e) => handleChange(parseInt(e.currentTarget.value, 10))}
        />
      </Show>
    </WizardField>
  );
};
