// Copyright (c) OpenLobster contributors. See LICENSE for details.

import { For } from "solid-js";
import { describe, it, expect } from 'vitest';
import { render } from '@solidjs/testing-library';
import { Card } from './Card';

describe('Card Component', () => {
  describe('Rendering', () => {
    it('renders card element', () => {
      const { container } = render(() => <Card>Content</Card>);
      const card = container.querySelector('.card');
      expect(card).toBeTruthy();
    });

    it('renders children content', () => {
      const { container } = render(() => <Card>Card content</Card>);
      expect(container.textContent).toContain('Card content');
    });

    it('renders multiple children', () => {
      const { container } = render(() => (
        <Card>
          <p>Paragraph 1</p>
          <p>Paragraph 2</p>
        </Card>
      ));
      expect(container.textContent).toContain('Paragraph 1');
      expect(container.textContent).toContain('Paragraph 2');
    });

    it('renders JSX children', () => {
      const { getByText } = render(() => (
        <Card>
          <button>Click me</button>
        </Card>
      ));
      expect(getByText('Click me')).toBeTruthy();
    });

    it('renders text nodes as children', () => {
      const { container } = render(() => <Card>Plain text</Card>);
      expect(container.textContent).toContain('Plain text');
    });
  });

  describe('Title Rendering', () => {
    it('renders title when provided', () => {
      const { getByText } = render(() => <Card title="Test Title">Content</Card>);
      expect(getByText('Test Title')).toBeTruthy();
    });

    it('does not render title element when title not provided', () => {
      const { container } = render(() => <Card>Content</Card>);
      const title = container.querySelector('.card-title');
      expect(title).toBeNull();
    });

    it('applies card-title class to title', () => {
      const { container } = render(() => <Card title="Title">Content</Card>);
      const title = container.querySelector('.card-title');
      expect(title?.classList.contains('card-title')).toBe(true);
    });

    it('does not render empty title string', () => {
      const { container } = render(() => <Card title="">Content</Card>);
      const title = container.querySelector('.card-title');
      expect(title).toBeFalsy();
    });

    it('renders title with special characters', () => {
      const specialTitle = 'Title & <Special> "Chars"';
      const { getByText } = render(() => <Card title={specialTitle}>Content</Card>);
      expect(getByText(specialTitle)).toBeTruthy();
    });
  });

  describe('Content Structure', () => {
    it('wraps children in card-content div', () => {
      const { container } = render(() => <Card>Content</Card>);
      const content = container.querySelector('.card-content');
      expect(content).toBeTruthy();
    });

    it('applies card-content class', () => {
      const { container } = render(() => <Card>Content</Card>);
      const content = container.querySelector('.card-content');
      expect(content?.classList.contains('card-content')).toBe(true);
    });

    it('places title before content', () => {
      const { container } = render(() => <Card title="Title">Content</Card>);
      const card = container.querySelector('.card');
      const title = card?.querySelector('.card-title');
      const content = card?.querySelector('.card-content');

      expect(card?.children[0]).toBe(title);
      expect(card?.children[1]).toBe(content);
    });
  });

  describe('CSS Classes', () => {
    it('applies card base class', () => {
      const { container } = render(() => <Card>Content</Card>);
      const card = container.querySelector('.card');
      expect(card?.classList.contains('card')).toBe(true);
    });

    it('applies custom className', () => {
      const { container } = render(() => <Card class="custom-card">Content</Card>);
      const card = container.querySelector('.card');
      expect(card?.classList.contains('custom-card')).toBe(true);
    });

    it('combines base and custom classes', () => {
      const { container } = render(() => <Card class="highlight">Content</Card>);
      const card = container.querySelector('.card');
      expect(card?.classList.contains('card')).toBe(true);
      expect(card?.classList.contains('highlight')).toBe(true);
    });

    it('supports multiple custom classes', () => {
      const { container } = render(() => <Card class="class1 class2 class3">Content</Card>);
      const card = container.querySelector('.card');
      expect(card?.classList.contains('class1')).toBe(true);
      expect(card?.classList.contains('class2')).toBe(true);
      expect(card?.classList.contains('class3')).toBe(true);
    });

    it('handles empty class string', () => {
      const { container } = render(() => <Card class="">Content</Card>);
      const card = container.querySelector('.card');
      expect(card?.classList.contains('card')).toBe(true);
    });
  });

  describe('Complex Content', () => {
    it('renders nested components', () => {
      const { getByText } = render(() => (
        <Card title="Nested">
          <div>
            <p>Paragraph inside div</p>
          </div>
        </Card>
      ));
      expect(getByText('Paragraph inside div')).toBeTruthy();
    });

    it('renders lists in content', () => {
      const { getByText } = render(() => (
        <Card title="List">
          <ul>
            <li>Item 1</li>
            <li>Item 2</li>
          </ul>
        </Card>
      ));
      expect(getByText('Item 1')).toBeTruthy();
      expect(getByText('Item 2')).toBeTruthy();
    });

    it('renders dynamic content', () => {
      const items = ['A', 'B', 'C'];
      const { getByText } = render(() => (
        <Card title="Dynamic">
          <For each={items}>{(item) => (
            <div>{item}</div>
          )}</For>
        </Card>
      ));
      expect(getByText('A')).toBeTruthy();
      expect(getByText('B')).toBeTruthy();
      expect(getByText('C')).toBeTruthy();
    });

    it('renders conditionally rendered content', () => {
      const shouldShow = true;
      const { getByText, queryByText } = render(() => (
        <Card title="Conditional">
          {shouldShow && <p>Visible content</p>}
          {!shouldShow && <p>Hidden content</p>}
        </Card>
      ));
      expect(getByText('Visible content')).toBeTruthy();
      expect(queryByText('Hidden content')).toBeNull();
    });

    it('renders cards within cards', () => {
      const { getByText } = render(() => (
        <Card title="Outer">
          <Card title="Inner">
            <p>Nested card content</p>
          </Card>
        </Card>
      ));
      expect(getByText('Outer')).toBeTruthy();
      expect(getByText('Inner')).toBeTruthy();
      expect(getByText('Nested card content')).toBeTruthy();
    });
  });

  describe('Styling', () => {
    it('has correct HTML structure for styling', () => {
      const { container } = render(() => <Card title="Title">Content</Card>);
      const card = container.querySelector('.card');
      const title = card?.querySelector('.card-title');
      const content = card?.querySelector('.card-content');

      expect(card).toBeTruthy();
      expect(title).toBeTruthy();
      expect(content).toBeTruthy();
    });

    it('preserves content styling', () => {
      const { container } = render(() => (
        <Card>
          <p style={{"color":"red"}}>Red text</p>
        </Card>
      ));
      const p = container.querySelector('p');
      expect(p?.style.color).toBe('red');
    });
  });

  describe('Empty States', () => {
    it('renders with empty content', () => {
      const { container } = render(() => <Card>{''}</Card>);
      const card = container.querySelector('.card');
      expect(card).toBeTruthy();
    });

    it('renders with empty title and empty content', () => {
      const { container } = render(() => <Card title="">{''}</Card>);
      const card = container.querySelector('.card');
      expect(card).toBeTruthy();
    });

    it('renders card-content even when empty', () => {
      const { container } = render(() => <Card>{'  '}</Card>);
      const content = container.querySelector('.card-content');
      expect(content).toBeTruthy();
    });
  });
});
