// Copyright (c) OpenLobster contributors. See LICENSE for details.
/* eslint-disable solid/reactivity */

import type { Component, JSX } from 'solid-js';
import './Input.css';

interface InputProps extends JSX.InputHTMLAttributes<HTMLInputElement> {
  label?: string;
  error?: string;
  hint?: string;
}

const Input: Component<InputProps> = (props) => {
  const { label, error, hint, class: className, ...rest } = props;

  return (
    <div class="input-wrapper">
      {label && <label class="input-label">{label}</label>}
      <input
        class={`input ${error ? 'input-error' : ''} ${className || ''}`}
        {...rest}
      />
      {error && <span class="input-error-text">{error}</span>}
      {hint && <span class="input-hint">{hint}</span>}
    </div>
  );
};

export { Input };
