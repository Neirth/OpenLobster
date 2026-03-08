// Copyright (c) OpenLobster contributors. See LICENSE for details.

import { createMemo, Show, For, type Component } from "solid-js";
import { t } from "../../App";
import type { SchemaProperty } from "../../schemas/config.schema";
import { getSchemaFieldI18nKey } from "../../schemas/config.schema";
import "./SchemaForm.css";

export interface SchemaFormProps {
  /**
   * Field key from the schema
   */
  field: string;
  /**
   * Schema property definition
   */
  schema: SchemaProperty;
  /**
   * Current form values (entire config object)
   */
  values: Record<string, any>;
  /**
   * Callback when field value changes
   */
  onChange: (field: string, value: any) => void;
}

/**
 * Check if a field should be shown based on its dependencies
 */
function checkDependencies(
  dependencies: Record<string, any> | undefined,
  values: Record<string, any>
): boolean {
  if (!dependencies) return true;

  for (const [depField, depCondition] of Object.entries(dependencies)) {
    if (depCondition.properties) {
      const fieldPath = depField.split(".");
      let currentValue = values;
      
      for (const pathPart of fieldPath) {
        if (currentValue && typeof currentValue === "object") {
          currentValue = currentValue[pathPart];
        } else {
          return false;
        }
      }

      const conditionKey = Object.keys(depCondition.properties)[0];
      const conditionValue = depCondition.properties[conditionKey];
      
      if (conditionValue.const !== undefined) {
        if (currentValue !== conditionValue.const) {
          return false;
        }
      }
    }

    if (depCondition.oneOf) {
      let matched = false;
      for (const condition of depCondition.oneOf) {
        if (condition.properties) {
          const condKey = Object.keys(condition.properties)[0];
          const condValue = condition.properties[condKey].const;
          if (values[condKey] === condValue) {
            matched = true;
            break;
          }
        }
      }
      if (!matched) return false;
    }
  }

  return true;
}

function getValueAtPath(values: Record<string, any>, path: string): any {
  const parts = path.split(".");
  let current: any = values;

  for (const part of parts) {
    if (current && typeof current === "object") {
      current = current[part];
    } else {
      return undefined;
    }
  }

  return current;
}

/**
 * Renders a single form field based on JSON Schema property
 */
export const SchemaField: Component<SchemaFormProps> = (props) => {
  const isVisible = createMemo(() => 
    checkDependencies(props.schema.dependencies, props.values)
  );

  const fieldValue = createMemo(() => getValueAtPath(props.values, props.field));

  const fieldClass = createMemo(() => {
    if (props.schema.type === "object") return "schema-field schema-field--group";
    if (props.schema.type === "string" && props.schema.format === "textarea") {
      return "schema-field schema-field--row schema-field--textarea";
    }
    return "schema-field schema-field--row";
  });

  const objectProperties = createMemo(() => {
    const schemaWithProps = props.schema as SchemaProperty & {
      properties?: Record<string, SchemaProperty>;
    };
    return schemaWithProps.properties;
  });

  const handleChange = (value: any) => {
    props.onChange(props.field, value);
  };

  const titleKey = getSchemaFieldI18nKey(props.field, false);
  const descKey = getSchemaFieldI18nKey(props.field, true);
  const displayTitle = () => {
    const translated = t(titleKey);
    return translated !== titleKey ? translated : props.schema.title;
  };
  const displayDescription = () => {
    const translated = t(descKey);
    return translated !== descKey ? translated : props.schema.description;
  };

  return (
    <Show when={isVisible()}>
      <div class={fieldClass()}>
        <div class="field-info">
          <label class="field-label">{displayTitle()}</label>
          <p class="field-description">{displayDescription()}</p>
        </div>

        {/* Boolean / Toggle */}
        <Show when={props.schema.type === "boolean"}>
          <label class="toggle-switch field-control">
            <input
              type="checkbox"
              checked={fieldValue() ?? props.schema.default ?? false}
              onChange={(e) => handleChange(e.currentTarget.checked)}
            />
            <span class="toggle-slider" />
          </label>
        </Show>

        {/* String with enum / Select */}
        <Show when={props.schema.type === "string" && props.schema.enum}>
          <select
            class="field-select field-control"
            value={fieldValue() ?? props.schema.default ?? ""}
            onChange={(e) => handleChange(e.currentTarget.value)}
          >
            <For each={props.schema.enum}>
              {(option) => <option value={option}>{option}</option>}
            </For>
          </select>
        </Show>

        {/* String input */}
        <Show when={
          props.schema.type === "string" && 
          !props.schema.enum && 
          props.schema.format !== "textarea" &&
          props.schema.format !== "password"
        }>
          <input
            type="text"
            class="field-input field-control"
            value={fieldValue() ?? props.schema.default ?? ""}
            placeholder={displayTitle()}
            onInput={(e) => handleChange(e.currentTarget.value)}
          />
        </Show>

        {/* Password input */}
        <Show when={props.schema.type === "string" && props.schema.format === "password"}>
          <input
            type="password"
            class="field-input field-control"
            value={fieldValue() ?? ""}
            placeholder={displayTitle()}
            onInput={(e) => handleChange(e.currentTarget.value)}
          />
        </Show>

        {/* Textarea */}
        <Show when={props.schema.type === "string" && props.schema.format === "textarea"}>
          <textarea
            class="field-textarea field-control"
            value={fieldValue() ?? props.schema.default ?? ""}
            placeholder={displayTitle()}
            onInput={(e) => handleChange(e.currentTarget.value)}
          />
        </Show>

        {/* Integer input */}
        <Show when={props.schema.type === "integer"}>
          <input
            type="number"
            class="field-input field-control"
            value={fieldValue() ?? props.schema.default ?? 0}
            min={props.schema.minimum}
            max={props.schema.maximum}
            onInput={(e) => handleChange(parseInt(e.currentTarget.value, 10))}
          />
        </Show>

        {/* Object (nested fields) */}
        <Show when={props.schema.type === "object" && objectProperties()}>
          <div class="field-group">
            <For each={Object.entries(objectProperties() || {})}>
              {([nestedKey, nestedSchema]) => (
                <SchemaField
                  field={`${props.field}.${nestedKey}`}
                  schema={nestedSchema as SchemaProperty}
                  values={props.values}
                  onChange={props.onChange}
                />
              )}
            </For>
          </div>
        </Show>
      </div>
    </Show>
  );
};
