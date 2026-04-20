// Copyright (c) OpenLobster contributors. See LICENSE for details.
 

import { describe, it, expect } from 'vitest';
import { render } from '@solidjs/testing-library';
import { Input } from './Input';

describe('Input Component', () => {
  it('renders input element', () => {
    const { container } = render(() => <Input />);
    const input = container.querySelector('input');
    expect(input).toBeTruthy();
    expect(input?.tagName).toBe('INPUT');
  });

  it('renders wrapper div', () => {
    const { container } = render(() => <Input />);
    const wrapper = container.querySelector('.input-wrapper');
    expect(wrapper).toBeTruthy();
  });

  it('renders label when provided', () => {
    const { container } = render(() => <Input label="Email" />);
    const label = container.querySelector('.input-label');
    expect(label).toBeTruthy();
    expect(label?.textContent).toContain('Email');
  });

  it('does not render label when not provided', () => {
    const { container } = render(() => <Input />);
    const label = container.querySelector('.input-label');
    expect(label).toBeFalsy();
  });

  it('renders error message when provided', () => {
    const { container } = render(() => <Input error="Field is required" />);
    const error = container.querySelector('.input-error-text');
    expect(error).toBeTruthy();
    expect(error?.textContent).toContain('Field is required');
  });

  it('does not render error message when not provided', () => {
    const { container } = render(() => <Input />);
    const error = container.querySelector('.input-error-text');
    expect(error).toBeFalsy();
  });

  it('applies error class when error is provided', () => {
    const { container } = render(() => <Input error="Error" />);
    const input = container.querySelector('input');
    expect(input?.classList.contains('input-error')).toBe(true);
  });

  it('does not apply error class without error', () => {
    const { container } = render(() => <Input />);
    const input = container.querySelector('input');
    expect(input?.classList.contains('input-error')).toBe(false);
  });

  it('renders hint text when provided', () => {
    const { container } = render(() => <Input hint="Optional field" />);
    const hint = container.querySelector('.input-hint');
    expect(hint).toBeTruthy();
    expect(hint?.textContent).toContain('Optional field');
  });

  it('does not render hint text when not provided', () => {
    const { container } = render(() => <Input />);
    const hint = container.querySelector('.input-hint');
    expect(hint).toBeFalsy();
  });

  it('supports text input type by default', () => {
    const { container } = render(() => <Input />);
    const input = container.querySelector('input') as HTMLInputElement;
    expect(input?.type).toBe('text');
  });

  it('supports email input type', () => {
    const { container } = render(() => <Input type="email" />);
    const input = container.querySelector('input') as HTMLInputElement;
    expect(input?.type).toBe('email');
  });

  it('supports password input type', () => {
    const { container } = render(() => <Input type="password" />);
    const input = container.querySelector('input') as HTMLInputElement;
    expect(input?.type).toBe('password');
  });

  it('supports number input type', () => {
    const { container } = render(() => <Input type="number" />);
    const input = container.querySelector('input') as HTMLInputElement;
    expect(input?.type).toBe('number');
  });

  it('accepts placeholder attribute', () => {
    const { container } = render(() => <Input placeholder="Enter text..." />);
    const input = container.querySelector('input') as HTMLInputElement;
    expect(input?.placeholder).toBe('Enter text...');
  });

  it('accepts initial value', () => {
    const { container } = render(() => <Input value="initial" />);
    const input = container.querySelector('input') as HTMLInputElement;
    expect(input?.value).toBe('initial');
  });

  it('is enabled by default', () => {
    const { container } = render(() => <Input />);
    const input = container.querySelector('input') as HTMLInputElement;
    expect(input?.disabled).toBe(false);
  });

  it('is disabled when disabled prop is true', () => {
    const { container } = render(() => <Input disabled />);
    const input = container.querySelector('input') as HTMLInputElement;
    expect(input?.disabled).toBe(true);
  });

  it('is not readonly by default', () => {
    const { container } = render(() => <Input />);
    const input = container.querySelector('input') as HTMLInputElement;
    expect(input?.readOnly).toBe(false);
  });

  it('becomes readonly when readonly prop is true', () => {
    const { container } = render(() => <Input readonly />);
    const input = container.querySelector('input') as HTMLInputElement;
    expect(input?.readOnly).toBe(true);
  });

  it('is required when required prop is true', () => {
    const { container } = render(() => <Input required />);
    const input = container.querySelector('input') as HTMLInputElement;
    expect(input?.required).toBe(true);
  });

  it('applies input base class', () => {
    const { container } = render(() => <Input />);
    const input = container.querySelector('input');
    expect(input?.classList.contains('input')).toBe(true);
  });

  it('applies custom class', () => {
    const { container } = render(() => <Input class="custom-input" />);
    const input = container.querySelector('input');
    expect(input?.classList.contains('custom-input')).toBe(true);
  });

  it('supports aria-label attribute', () => {
    const { container } = render(() => <Input aria-label="Username" />);
    const input = container.querySelector('input');
    expect(input?.getAttribute('aria-label')).toBe('Username');
  });

  it('supports data attributes', () => {
    const { container } = render(() => <Input data-testid="email-input" />);
    const input = container.querySelector('input');
    expect(input?.getAttribute('data-testid')).toBe('email-input');
  });

  it('associates label with input via htmlFor', () => {
    const { container } = render(() => <Input label="Email" id="email-input" />);
    const input = container.querySelector('input');
    expect(input?.id).toBe('email-input');
  });
});
