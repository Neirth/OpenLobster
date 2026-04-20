// Copyright (c) OpenLobster contributors. See LICENSE for details.
/* eslint-disable solid/reactivity */

import type { Component, JSX } from 'solid-js';
import './Button.css';

export type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger';
export type ButtonSize = 'sm' | 'md' | 'lg';

interface ButtonProps extends JSX.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  size?: ButtonSize;
  isLoading?: boolean;
  disabled?: boolean;
}

const Button: Component<ButtonProps> = (props) => {
  const {
    variant = 'primary',
    size = 'md',
    isLoading = false,
    disabled = false,
    children,
    class: className,
    ...rest
  } = props;

  const baseClass = `btn btn-${variant} btn-${size}`;
  const finalClass = `${baseClass} ${className || ''}`.trim();

  return (
    <button
      class={finalClass}
      disabled={disabled || isLoading}
      {...rest}
    >
      {isLoading ? <span class="spinner" /> : children}
    </button>
  );
};

export { Button };
