// Copyright (c) OpenLobster contributors. See LICENSE for details.
/* eslint-disable no-undef */

import { describe, it, expect } from 'vitest';
import { render } from '@solidjs/testing-library';
import { Button } from './Button';

describe('Button Component', () => {
  it('renders button element', () => {
    const { container } = render(() => <Button>Click me</Button>);
    const button = container.querySelector('button');
    expect(button).toBeTruthy();
    expect(button?.tagName).toBe('BUTTON');
  });

  it('renders button with text content', () => {
    const { container } = render(() => <Button>Click me</Button>);
    const button = container.querySelector('button');
    expect(button?.textContent).toBe('Click me');
  });

  it('applies primary variant class by default', () => {
    const { container } = render(() => <Button>Primary</Button>);
    const button = container.querySelector('button');
    expect(button?.classList.contains('btn')).toBe(true);
    expect(button?.classList.contains('btn-primary')).toBe(true);
  });

  it('applies secondary variant class', () => {
    const { container } = render(() => <Button variant="secondary">Secondary</Button>);
    const button = container.querySelector('button');
    expect(button?.classList.contains('btn-secondary')).toBe(true);
  });

  it('applies ghost variant class', () => {
    const { container } = render(() => <Button variant="ghost">Ghost</Button>);
    const button = container.querySelector('button');
    expect(button?.classList.contains('btn-ghost')).toBe(true);
  });

  it('applies danger variant class', () => {
    const { container } = render(() => <Button variant="danger">Danger</Button>);
    const button = container.querySelector('button');
    expect(button?.classList.contains('btn-danger')).toBe(true);
  });

  it('applies small size class', () => {
    const { container } = render(() => <Button size="sm">Small</Button>);
    const button = container.querySelector('button');
    expect(button?.classList.contains('btn-sm')).toBe(true);
  });

  it('applies medium size class by default', () => {
    const { container } = render(() => <Button>Medium</Button>);
    const button = container.querySelector('button');
    expect(button?.classList.contains('btn-md')).toBe(true);
  });

  it('applies large size class', () => {
    const { container } = render(() => <Button size="lg">Large</Button>);
    const button = container.querySelector('button');
    expect(button?.classList.contains('btn-lg')).toBe(true);
  });

  it('is enabled by default', () => {
    const { container } = render(() => <Button>Enabled</Button>);
    const button = container.querySelector('button') as HTMLButtonElement;
    expect(button?.disabled).toBe(false);
  });

  it('is disabled when disabled prop is true', () => {
    const { container } = render(() => <Button disabled>Disabled</Button>);
    const button = container.querySelector('button') as HTMLButtonElement;
    expect(button?.disabled).toBe(true);
  });

  it('is disabled when isLoading is true', () => {
    const { container } = render(() => <Button isLoading>Loading</Button>);
    const button = container.querySelector('button') as HTMLButtonElement;
    expect(button?.disabled).toBe(true);
  });

  it('shows spinner when loading', () => {
    const { container } = render(() => <Button isLoading>Loading</Button>);
    const spinner = container.querySelector('.spinner');
    expect(spinner).toBeTruthy();
  });

  it('hides children when loading', () => {
    const { container } = render(() => <Button isLoading>Click me</Button>);
    const button = container.querySelector('button');
    expect(button?.textContent).not.toContain('Click me');
  });

  it('applies custom class along with default classes', () => {
    const { container } = render(() => <Button class="custom-class">Button</Button>);
    const button = container.querySelector('button');
    expect(button?.classList.contains('btn')).toBe(true);
    expect(button?.classList.contains('custom-class')).toBe(true);
  });

  it('forwards HTML attributes', () => {
    const { container } = render(() => (
      <Button data-testid="custom-button" aria-label="Click button">
        Button
      </Button>
    ));
    const button = container.querySelector('button');
    expect(button?.getAttribute('data-testid')).toBe('custom-button');
    expect(button?.getAttribute('aria-label')).toBe('Click button');
  });

  it('handles click events', () => {
    let clicked = false;
    const { container } = render(() => (
      <Button onClick={() => { clicked = true; }}>Click me</Button>
    ));
    const button = container.querySelector('button') as HTMLButtonElement;
    button?.click();
    expect(clicked).toBe(true);
  });

  it('renders with children JSX', () => {
    const { container } = render(() => (
      <Button>
        <span>Icon</span>
        <span>Text</span>
      </Button>
    ));
    const button = container.querySelector('button');
    expect(button?.querySelector('span')).toBeTruthy();
  });
});
