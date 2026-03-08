// Copyright (c) OpenLobster contributors. See LICENSE for details.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render } from '@solidjs/testing-library';
import App from '../src/App';

// Mock locale files
vi.mock('../src/locales/en.json', () => ({
  default: {
    'dashboard.title': 'Dashboard',
    'chat.title': 'Chat',
    'tasks.title': 'Tasks',
    'common.loading': 'loading...',
  },
}));

vi.mock('../src/locales/es.json', () => ({
  default: {
    'dashboard.title': 'Panel',
    'chat.title': 'Chat',
    'tasks.title': 'Tareas',
    'common.loading': 'cargando...',
  },
}));

// Mock Router before importing App
vi.mock('@solidjs/router', () => ({
  Router: (props: any) => <div class="router-mock">{props.children}</div>,
  Route: () => null,
}));

// Mock AppShell
vi.mock('../src/components/AppShell/AppShell', () => ({
  default: () => <div class="app-shell" />,
}));

// Mock views
vi.mock('../src/views/ChatView/ChatView', () => ({
  default: () => <div>Chat</div>,
}));

vi.mock('../src/views/DashboardView/DashboardView', () => ({
  default: () => <div>Dashboard</div>,
}));

vi.mock('../src/views/TasksView/TasksView', () => ({
  default: () => <div>Tasks</div>,
}));

vi.mock('../src/views/MemoryView/MemoryView', () => ({
  default: () => <div>Memory</div>,
}));

vi.mock('../src/views/McpsView/McpsView', () => ({
  default: () => <div>MCPs</div>,
}));

vi.mock('../src/views/SkillsView/SkillsView', () => ({
  default: () => <div>Skills</div>,
}));

vi.mock('../src/views/SettingsView/SettingsView', () => ({
  default: () => <div>Settings</div>,
}));

vi.mock('../src/components/FirstBootWizard/FirstBootWizard', () => ({
  default: (props: { onComplete: () => void }) => (
    <div class="first-boot-wizard-mock">
      <button onClick={props.onComplete}>Complete</button>
    </div>
  ),
}));

const mockFetch = vi.fn();

describe('App Component', () => {
  beforeEach(() => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => ({ data: { config: { wizardCompleted: true } } }),
    });
    global.fetch = mockFetch;
  });

  it('renders without crashing', () => {
    const { container } = render(() => <App />);
    expect(container).toBeTruthy();
  });

  it('renders with Router', () => {
    const { container } = render(() => <App />);
    const routerDiv = container.querySelector('.router-mock');
    expect(routerDiv).toBeTruthy();
  });

  it('includes Router mock', () => {
    const { container } = render(() => <App />);
    const routerDiv = container.querySelector('.router-mock');
    expect(routerDiv).toBeTruthy();
  });

  it('shows FirstBootWizard when wizard not completed (first boot)', async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => ({ data: { config: { wizardCompleted: false } } }),
    });
    const { container } = render(() => <App />);
    await vi.waitFor(() => {
      const wizard = container.querySelector('.first-boot-wizard-mock');
      expect(wizard).toBeTruthy();
    });
  });
});
