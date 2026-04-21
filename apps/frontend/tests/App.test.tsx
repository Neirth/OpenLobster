
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render } from '@solidjs/testing-library';
import App, { setConfigLoaded, setShowWizard } from '@/App';

// Mock locale files
vi.mock('@/locales/en.json', () => ({
  default: {
    'dashboard.title': 'Dashboard',
    'chat.title': 'Chat',
    'tasks.title': 'Tasks',
    'common.loading': 'loading...',
  },
}));

vi.mock('@/locales/es.json', () => ({
  default: {
    'dashboard.title': 'Panel',
    'chat.title': 'Chat',
    'tasks.title': 'Tareas',
    'common.loading': 'cargando...',
  },
}));

// Mock Router before importing App
vi.mock('@solidjs/router', () => ({
  Router: (props: any) => {
    const Root = props.root;
    if (Root) {
      return (
        <div class="router-mock">
          <Root children={props.children} />
        </div>
      );
    }
    return <div class="router-mock">{props.children}</div>;
  },
  Route: () => null,
  useLocation: () => ({ pathname: '/' }),
}));

// Mock AppShell
vi.mock('@/components/AppShell/AppShell', () => ({
  default: () => <div class="app-shell" />,
}));

// Mock views
vi.mock('@/views/ChatView/ChatView', () => ({
  default: () => <div>Chat</div>,
}));

vi.mock('@/views/DashboardView/DashboardView', () => ({
  default: () => <div>Dashboard</div>,
}));

vi.mock('@/views/TasksView/TasksView', () => ({
  default: () => <div>Tasks</div>,
}));

vi.mock('@/views/MemoryView/MemoryView', () => ({
  default: () => <div>Memory</div>,
}));

vi.mock('@/views/McpsView/McpsView', () => ({
  default: () => <div>MCPs</div>,
}));

vi.mock('@/views/SkillsView/SkillsView', () => ({
  default: () => <div>Skills</div>,
}));

vi.mock('@/views/SettingsView/SettingsView', () => ({
  default: () => <div>Settings</div>,
}));

vi.mock('@/graphql/config', () => ({
  GRAPHQL_ENDPOINT: '/graphql',
  client: { request: vi.fn() },
}));

vi.mock('@/components/AuthModals', () => ({
  default: (props: { children: any }) => <div class="auth-modals-mock">{props.children}</div>,
}));

vi.mock('@/views/WizardView/WizardView', () => ({
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
    // Reset global state to ensure tests are isolated
    setConfigLoaded(false);
    setShowWizard(false);
  });

  it('renders without crashing', () => {
    const { container } = render(() => <App />);
    expect(container).toBeTruthy();
  });

  it('renders with Router', async () => {
    const { container } = render(() => <App />);
    await vi.waitFor(() => {
      const routerDiv = container.querySelector('.router-mock');
      expect(routerDiv).toBeTruthy();
    });
  });

  it('includes Router mock', async () => {
    const { container } = render(() => <App />);
    await vi.waitFor(() => {
      const routerDiv = container.querySelector('.router-mock');
      expect(routerDiv).toBeTruthy();
    });
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
