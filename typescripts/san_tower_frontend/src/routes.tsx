import {
  AppWindowIcon,
  BrainIcon,
  CableIcon,
  HardDriveIcon,
  HouseIcon,
  MessageSquareTextIcon,
  MessagesSquareIcon,
  SettingsIcon,
  WorkflowIcon,
} from 'lucide-react';
import { createBrowserRouter } from 'react-router';
import { Layout, type Page } from './Layout';
import { BrowserRemoteServer } from './pages/BrowserRemoteServer';
import { Chat } from './pages/Chat';
import { Connections } from './pages/Connections';
import { Home } from './pages/Home';
import { Knowledge } from './pages/Knowledge';
import { KnowledgeGraph } from './pages/KnowledgeGraph';
import { MessageBroker } from './pages/MessageBroker';
import { NotFound } from './pages/NotFound';
import { Settings } from './pages/Settings';
import { Storage } from './pages/Storage';
import { WorkflowEditor } from './pages/WorkflowEditor';
import { WorkflowList } from './pages/WorkflowList';

// pages are the tower's top-level pages, in the order the nav shows them. A
// new page is one entry here. The server answers every path without a file
// extension with index.html, so each page can be reloaded or linked to.
const pages: Page[] = [
  { path: '/', label: 'Home', icon: HouseIcon, element: <Home /> },
  // After Home (the user, 2026-10-28): chats with workflows.
  { path: '/chat', label: 'Chat', icon: MessageSquareTextIcon, element: <Chat /> },
  // After Chat, empty for now (the user, 2026-10-28).
  { path: '/knowledge', label: 'Knowledge', icon: BrainIcon, element: <Knowledge /> },
  { path: '/workflow', label: 'Workflow', icon: WorkflowIcon, element: <WorkflowList /> },
  { path: '/message-broker', label: 'Message Broker', icon: MessagesSquareIcon, element: <MessageBroker /> },
  { path: '/connections', label: 'Connections', icon: CableIcon, element: <Connections /> },
  { path: '/storage', label: 'Storage', icon: HardDriveIcon, element: <Storage /> },
  { path: '/browser-remote', label: 'Browser Remote Server', icon: AppWindowIcon, element: <BrowserRemoteServer /> },
  { path: '/settings', label: 'Settings', icon: SettingsIcon, element: <Settings /> },
];

// subpages are pages under a top-level page, which the nav marks while they
// are open.
const subpages = [
  { path: '/workflow/:id', element: <WorkflowEditor /> },
  { path: '/knowledge/:id', element: <KnowledgeGraph /> },
];

export const router = createBrowserRouter([
  {
    element: <Layout pages={pages} />,
    children: [
      ...pages.map(({ path, element }) => ({ path, element })),
      ...subpages,
      { path: '*', element: <NotFound /> },
    ],
  },
]);
