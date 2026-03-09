import React, { useEffect, useMemo } from 'react';
import { useSelector, useDispatch } from 'react-redux';
import { RootState, setStatus, setActiveTab, setTheme, setLanguage, setCurrentSessionKey, toggleSidebar } from './store';
import { Sidebar } from './components/Sidebar';
import { ChatView } from './views/ChatView';
import { SessionsView } from './views/SessionsView';
import { ScheduledTasksView } from './views/ScheduledTasksView';
import { SkillsView } from './views/SkillsView';
import { MCPView } from './views/MCPView';
import { SettingsView } from './views/SettingsView';
import { wsClient } from './services/websocket';
import { useTranslation } from './i18n';

function App() {
  const dispatch = useDispatch();
  const { t, language } = useTranslation();
  const { activeTab, theme, sidebarCollapsed } = useSelector((state: RootState) => state.ui);
  const { status } = useSelector((state: RootState) => state.gateway);
  const isMac = window.electronAPI.platform.isMac;
  const controlAnchorStyle = isMac
    ? { left: '92px', top: '10px' }
    : { left: '12px', top: '10px' };
  const dragStripStyle = isMac
    ? { left: '176px', right: '0px', top: '0px' }
    : { left: '120px', right: '0px', top: '0px' };
  const statusLabel = useMemo(() => {
    const labels = {
      zh: {
        running: 'Gateway 在线',
        stopped: 'Gateway 离线',
        starting: 'Gateway 启动中',
        error: 'Gateway 异常'
      },
      en: {
        running: 'Gateway online',
        stopped: 'Gateway offline',
        starting: 'Gateway starting',
        error: 'Gateway error'
      }
    };

    return labels[language][status] || status;
  }, [language, status]);
  const activeTabLabel = useMemo(() => {
    const map = {
      chat: t('nav.chat'),
      sessions: t('nav.sessions'),
      scheduled: t('nav.scheduled'),
      skills: t('nav.skills'),
      mcp: t('nav.mcp'),
      settings: t('nav.settings')
    };
    return map[activeTab];
  }, [activeTab, t]);

  useEffect(() => {
    dispatch(setCurrentSessionKey(`desktop:${Date.now()}`));

    // Load app settings from electron store
    window.electronAPI.config.get().then((config) => {
      if (config.theme) {
        dispatch(setTheme(config.theme));
      }
      // Only override system-detected language if user has explicitly set it
      if (config.language) {
        dispatch(setLanguage(config.language));
      }
      // Note: if config.language is not set, the system-detected language from store/index.ts is used
    });

    // Initialize Gateway status
    window.electronAPI.gateway.getStatus().then(status => {
      dispatch(setStatus(status));
    });

    // Listen for status changes
    const unsubscribe = window.electronAPI.gateway.onStatusChange((status) => {
      dispatch(setStatus(status));
    });

    // Listen for config changes
    const unsubscribeConfig = window.electronAPI.config.onChange((config) => {
      if (config.theme) {
        dispatch(setTheme(config.theme));
      }
      if (config.language) {
        dispatch(setLanguage(config.language));
      }
    });

    // Connect WebSocket for real-time updates
    wsClient.connect();

    // Subscribe to WebSocket messages
    const unsubscribeWS = wsClient.on('message', (data) => {
      console.log('Received WebSocket message:', data);
      // Could trigger session refresh here if needed
    });

    // Listen for tray events
    const unsubscribeNewChat = window.electronAPI.tray.onNewChat(() => {
      dispatch(setActiveTab('chat'));
    });

    const unsubscribeSettings = window.electronAPI.tray.onOpenSettings(() => {
      dispatch(setActiveTab('settings'));
    });

    return () => {
      unsubscribe();
      unsubscribeConfig();
      unsubscribeNewChat();
      unsubscribeSettings();
      unsubscribeWS();
      wsClient.disconnect();
    };
  }, [dispatch]);

  // Apply theme to document
  useEffect(() => {
    document.documentElement.classList.remove('light', 'dark');
    if (theme === 'system') {
      const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
      document.documentElement.classList.add(prefersDark ? 'dark' : 'light');
    } else {
      document.documentElement.classList.add(theme);
    }
  }, [theme]);

  const handleNewTask = () => {
    const newSessionKey = `desktop:${Date.now()}`;
    dispatch(setCurrentSessionKey(newSessionKey));
    dispatch(setActiveTab('chat'));
  };

  return (
    <div className="desktop-shell h-screen overflow-hidden px-3 py-3 text-foreground md:px-5 md:py-5">
      <div className="desktop-glow" />
      <div className="desktop-panel relative flex h-full gap-3 overflow-hidden rounded-[34px] border border-white/55 bg-white/62 shadow-[0_32px_100px_rgba(25,34,52,0.2)] backdrop-blur-2xl">
        <div className={`absolute z-10 draggable ${isMac ? 'h-14' : 'h-12'}`} style={dragStripStyle} />
        <div className="absolute z-40 flex items-center gap-2 no-drag" style={controlAnchorStyle}>
          <button
            onClick={() => dispatch(toggleSidebar())}
            className="flex h-9 w-9 items-center justify-center rounded-xl border border-white/70 bg-white/84 text-foreground/70 shadow-[0_10px_26px_rgba(31,41,55,0.09)] transition-colors hover:bg-white hover:text-foreground"
            aria-label="Toggle sidebar"
            title="Toggle sidebar"
          >
            <SidebarToggleIcon collapsed={sidebarCollapsed} className="h-4 w-4" />
          </button>
          {sidebarCollapsed && (
            <button
              onClick={handleNewTask}
              className="flex h-9 w-9 items-center justify-center rounded-xl border border-white/70 bg-white/84 text-foreground/70 shadow-[0_10px_26px_rgba(31,41,55,0.09)] transition-colors hover:bg-white hover:text-foreground"
              aria-label="New task"
              title="New task"
            >
              <PencilIcon className="h-4 w-4" />
            </button>
          )}
        </div>
        <div className="absolute right-4 top-4 z-40 flex items-center gap-2 no-drag">
          <div className="hidden items-center gap-2 rounded-full border border-white/70 bg-white/76 px-3 py-1.5 text-[11px] font-medium tracking-[0.16em] text-foreground/58 shadow-[0_10px_30px_rgba(31,41,55,0.08)] md:flex">
            <span className={`status-dot ${status}`} />
            {statusLabel}
          </div>
          <div className="hidden rounded-full border border-white/70 bg-white/76 px-3 py-1.5 text-[11px] font-medium tracking-[0.16em] text-foreground/58 shadow-[0_10px_30px_rgba(31,41,55,0.08)] md:block">
            {activeTabLabel}
          </div>
          <div className="rounded-full border border-white/70 bg-[#192233] px-3 py-1.5 text-[11px] font-semibold tracking-[0.22em] text-white shadow-[0_12px_34px_rgba(25,34,51,0.28)]">
            MAXCLAW
          </div>
        </div>
        <Sidebar />
        <main className="mr-2 flex-1 overflow-hidden rounded-[30px] bg-transparent">
          {activeTab === 'chat' && <ChatView />}
          {activeTab === 'sessions' && <SessionsView />}
          {activeTab === 'scheduled' && <ScheduledTasksView />}
          {activeTab === 'skills' && <SkillsView />}
          {activeTab === 'mcp' && <MCPView />}
          {activeTab === 'settings' && <SettingsView />}
        </main>
      </div>
    </div>
  );
}

export default App;

function SidebarToggleIcon({ className, collapsed }: { className?: string; collapsed: boolean }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <rect x={3} y={4} width={18} height={16} rx={2.5} strokeWidth={1.7} />
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.7} d="M9 4v16" />
      {collapsed ? (
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.7} d="M13 9l3 3-3 3" />
      ) : (
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.7} d="M15 9l-3 3 3 3" />
      )}
    </svg>
  );
}

function PencilIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 20h9" />
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M16.5 3.5a2.1 2.1 0 113 3L7 19l-4 1 1-4 12.5-12.5z" />
    </svg>
  );
}
