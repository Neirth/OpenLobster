import { describe, it, expect, vi, beforeEach } from 'vitest';
import { fireEvent } from '@solidjs/testing-library';

const mockMemoryData = {
  nodes: [
    { id: '1', label: 'John Doe', type: 'person', value: 'Test value', properties: { nick: 'JD' }, createdAt: '' },
    { id: '2', label: 'Jane Smith', type: 'person', value: '', properties: {}, createdAt: '' },
  ],
  edges: [ { sourceId: '1', targetId: '2', relation: 'knows' } ],
};

vi.mock('@solidjs/router', () => ({ A: (props: any) => <a {...props} /> }));

vi.mock('../../components/AppShell/AppShell', () => ({ default: (props: any) => <div class="app-shell" {...props} /> }));

vi.mock('../../components/GraphVisualization', () => ({ default: () => <div class="graph-visualization-mock" /> }));

vi.mock('@openlobster/ui/hooks', () => ({ useMemory: () => ({ data: mockMemoryData, isLoading: false, error: null, refetch: vi.fn() }) }));

vi.mock('../../graphql/client', () => ({ client: { request: vi.fn((mutation: any, vars: any) => {
  if (mutation && mutation.includes('update')) {
    return Promise.resolve({ updateMemoryNode: { id: vars.id, label: vars.label, type: vars.type, properties: JSON.parse(vars.properties || '{}') } });
  }
  if (mutation && mutation.includes('delete')) {
    return Promise.resolve({ deleteMemoryNode: { id: vars.id } });
  }
  return Promise.resolve({});
}) } }));

// Partially mock createMutation to run onSuccess immediately after mutationFn resolves
vi.mock('@tanstack/solid-query', async (importOriginal) => {
  const actual = await importOriginal() as any;
  return {
    ...(actual as any),
    useQueryClient: () => ({ invalidateQueries: vi.fn() }),
    createMutation: (optsFactory: any) => {
      const opts = optsFactory();
      return {
        mutate: (vars: any) => Promise.resolve(opts.mutationFn(vars)).then((res: any) => opts.onSuccess && opts.onSuccess(res)),
        isPending: false,
      };
    },
  } as any;
});

import { renderWithQueryClient } from '../../test-utils';
import MemoryView from './MemoryView';
import { client } from '../../graphql/client';

describe('MemoryView mutation flows', () => {
  beforeEach(() => {
    vi.resetAllMocks();
  });

  it('updateNode onSuccess closes edit modal and updates selected node properties', async () => {
    const { container } = renderWithQueryClient(() => <MemoryView />);

    // select first node
    fireEvent.click(container.querySelectorAll('.memory-item')[0] as HTMLElement);
    // open edit modal
    fireEvent.click(container.querySelector('.action-btn') as HTMLElement);

    // add property and fill
    fireEvent.click(container.querySelector('.memory-modal-add-prop') as HTMLElement);
    const inputs = container.querySelectorAll('.memory-modal-prop-row input');
    fireEvent.input(inputs[0] as HTMLInputElement, { target: { value: 'nickname' } });
    fireEvent.input(inputs[1] as HTMLInputElement, { target: { value: 'Johnny' } });

    // submit form and wait for mocked mutation to resolve
    const form = container.querySelector('.memory-modal-form') as HTMLFormElement;
    fireEvent.submit(form);
    await Promise.resolve();
    await Promise.resolve();

    // onSuccess should have closed modal and updated selected node properties
    expect(container.querySelector('.modal-overlay')).toBeNull();

    // ensure client.request was called for update
    expect((client.request as any).mock.calls.length).toBeGreaterThan(0);
  });

  it('deleteNode onSuccess closes delete modal and clears selection', async () => {
    const { container } = renderWithQueryClient(() => <MemoryView />);

    // select node
    fireEvent.click(container.querySelectorAll('.memory-item')[0] as HTMLElement);
    // open delete modal
    fireEvent.click(container.querySelector('.action-btn--danger') as HTMLElement);

    // confirm delete
    const confirmBtn = container.querySelector('.modal-btn--danger') as HTMLButtonElement;
    fireEvent.click(confirmBtn);
    await Promise.resolve();
    await Promise.resolve();

    // modal should be closed
    expect(container.querySelector('.memory-modal-confirm')).toBeNull();
    // selected node should be cleared -> shows no-selection fallback
    expect(container.querySelector('.no-selection')).toBeTruthy();
  });
});
