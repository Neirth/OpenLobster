// Copyright (c) OpenLobster contributors. See LICENSE for details.

import { createMemo, Show, For, type Component } from "solid-js";
import { t } from "../../App";
import type { SchemaProperty } from "../../schemas/config.schema";
import { getSchemaFieldI18nKey } from "../../schemas/config.schema";
import "./SchemaForm.css";

/** Primitive value types used in configuration form fields. */
type FormPrimitiveValue = string | number | boolean | undefined;

// Use an interface to model recursive objects. TypeScript rejects recursive type aliases,
// but recursive interfaces are allowed.
interface FormValueRecord {
  [key: string]: FormValue;
}

type FormValue = FormPrimitiveValue | FormValueRecord;

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
  values: Record<string, unknown>;
  /**
   * Callback when field value changes
   */
  onChange: (field: string, value: FormValue) => void;
}

/** Dependency condition shape used in JSON Schema. */
interface DependencyConditionNode {
  const?: unknown;
  properties?: Record<string, DependencyConditionNode>;
}

interface DependencyCondition {
  properties?: Record<string, DependencyConditionNode>;
  oneOf?: DependencyCondition[];
}

/**
 * Check if a field should be shown based on its dependencies
 */
function checkDependencies(
  dependencies: Record<string, DependencyCondition> | undefined,
  values: Record<string, unknown>
): boolean {
  if (!dependencies) return true;

  for (const [depField, depCondition] of Object.entries(dependencies)) {
    if (depCondition.properties) {
      const fieldPath = depField.split(".");
      let currentValue: unknown = values;

      for (const pathPart of fieldPath) {
        if (currentValue && typeof currentValue === "object") {
          currentValue = (currentValue as Record<string, unknown>)[pathPart];
        } else {
          return false;
        }
      }

      // Resolve the "leaf" node for this dependency path and compare its `const`
      // against the current value at the same path.
      const leafNode = (() => {
        const firstKey = fieldPath[0];
        let node: DependencyConditionNode | undefined = depCondition.properties?.[firstKey];
        for (let i = 1; i < fieldPath.length; i++) {
          node = node?.properties?.[fieldPath[i]];
        }
        return node;
      })();

      if (leafNode?.const !== undefined && currentValue !== leafNode.const) return false;
    }

    if (depCondition.oneOf) {
      const anyMatched = depCondition.oneOf.some((condition) => {
        if (!condition.properties) return false;

        const fieldPath = depField.split(".");
        let current: unknown = values;
        for (const part of fieldPath) {
          if (current && typeof current === "object") {
            current = (current as Record<string, unknown>)[part];
          } else {
            return false;
          }
        }

        const firstKey = fieldPath[0];
        let node: DependencyConditionNode | undefined = condition.properties[firstKey];
        for (let i = 1; i < fieldPath.length; i++) {
          node = node?.properties?.[fieldPath[i]];
        }
        return node?.const !== undefined && current === node.const;
      });

      if (!anyMatched) return false;
    }
  }

  return true;
}

function getValueAtPath(values: Record<string, unknown>, path: string): FormValue | undefined {
  const parts = path.split(".");
  let current: unknown = values;

  for (const part of parts) {
    if (current && typeof current === "object") {
      current = (current as Record<string, unknown>)[part];
    } else {
      return undefined;
    }
  }

  return current as FormValue | undefined;
}

/**
 * Renders a single form field based on JSON Schema property
 */
export const SchemaField: Component<SchemaFormProps> = (props) => {
  const isVisible = createMemo(() =>
    checkDependencies(
      props.schema.dependencies as Record<string, DependencyCondition> | undefined,
      props.values
    )
  );

  const fieldValue = createMemo(() => getValueAtPath(props.values, props.field));

  const fieldValueAsBoolean = () => (typeof fieldValue() === "boolean" ? (fieldValue() as boolean) : undefined);
  const fieldValueAsString = () => (typeof fieldValue() === "string" ? (fieldValue() as string) : undefined);
  const fieldValueAsNumber = () => (typeof fieldValue() === "number" ? (fieldValue() as number) : undefined);

  const fieldClass = createMemo(() => {
    if (props.schema.type === "object") return "schema-field schema-field--group";
    if (props.schema.type === "string" && props.schema.format === "textarea") {
      return "schema-field schema-field--row schema-field--textarea";
    }
    return "schema-field schema-field--row";
  });

  const objectProperties = createMemo(() => props.schema.properties);

  const handleChange = (value: FormValue) => {
    props.onChange(props.field, value);
  };

  const titleKey = () => getSchemaFieldI18nKey(props.field, false);
  const descKey = () => getSchemaFieldI18nKey(props.field, true);
  const displayTitle = () => {
    const key = titleKey();
    const translated = t(key);
    return translated !== key ? translated : props.schema.title;
  };
  const displayDescription = () => {
    const key = descKey();
    const translated = t(key);
    return translated !== key ? translated : props.schema.description;
  };

  return (
    <Show when={isVisible()}>
      <div class={fieldClass()}>
        <Show when={props.schema.type !== "object"}>
          <div class="field-info">
            <label class="field-label">{displayTitle()}</label>
            <p class="field-description">{displayDescription()}</p>
          </div>
        </Show>

        {/* Boolean / Toggle */}
        <Show when={props.schema.type === "boolean"}>
          <label class="toggle-switch field-control">
            <input
              type="checkbox"
              checked={fieldValueAsBoolean() ?? (props.schema.default as boolean | undefined) ?? false}
              onChange={(e) => handleChange(e.currentTarget.checked)}
            />
            <span class="toggle-slider" />
          </label>
        </Show>

        {/* String with enum / Select */}
        <Show when={props.schema.type === "string" && props.schema.enum}>
          <select
            class="field-select field-control"
            value={fieldValueAsString() ?? (props.schema.default as string | undefined) ?? ""}
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
            value={fieldValueAsString() ?? (props.schema.default as string | undefined) ?? ""}
            placeholder={displayTitle()}
            onInput={(e) => handleChange(e.currentTarget.value)}
          />
        </Show>

        {/* Password input */}
        <Show when={props.schema.type === "string" && props.schema.format === "password"}>
          <input
            type="password"
            class="field-input field-control"
            value={fieldValueAsString() ?? ""}
            placeholder={displayTitle()}
            onInput={(e) => handleChange(e.currentTarget.value)}
          />
        </Show>

        {/* Textarea */}
        <Show when={props.schema.type === "string" && props.schema.format === "textarea"}>
          <textarea
            class="field-textarea field-control"
            value={fieldValueAsString() ?? (props.schema.default as string | undefined) ?? ""}
            placeholder={displayTitle()}
            onInput={(e) => handleChange(e.currentTarget.value)}
          />
        </Show>

        {/* Integer input */}
        <Show when={props.schema.type === "integer"}>
          <input
            type="number"
            class="field-input field-control"
            value={fieldValueAsNumber() ?? (props.schema.default as number | undefined) ?? 0}
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
