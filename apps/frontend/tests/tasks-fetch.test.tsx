import { render, waitFor } from '@solidjs/testing-library';
import { expect, test, vi } from 'vitest';

vi.mock('@/hooks', async () => {
  return await vi.importActual('../src/hooks/index.ts');
});
import { client } from '../src/graphql/config';
import { QueryClient, QueryClientProvider } from '@tanstack/solid-query';

vi.mock('../src/components/AppShell', () => ({
  default: (props: any) => <div>{props.children}</div>,
}));

import TasksView from '../src/views/TasksView/TasksView';

test('TasksView sends fetch request', async () => {
  const queryClient = new QueryClient();
  const spy = vi.spyOn(client, 'request').mockResolvedValue({ tasks: [] });

  render(() => (
    <QueryClientProvider client={queryClient}>
      <TasksView />
    </QueryClientProvider>
  ));
  
  await waitFor(() => {
    expect(spy).toHaveBeenCalled();
  });
});
