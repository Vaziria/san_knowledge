import { createBrowserRouter } from 'react-router';
import { Layout, type Page } from './Layout';
import { Home } from './pages/Home';
import { MessageBroker } from './pages/MessageBroker';
import { NotFound } from './pages/NotFound';

// pages are the tower's top-level pages, in the order the nav shows them. A
// new page is one entry here. The server answers every path without a file
// extension with index.html, so each page can be reloaded or linked to.
const pages: Page[] = [
  { path: '/', label: 'Home', element: <Home /> },
  { path: '/message-broker', label: 'Message Broker', element: <MessageBroker /> },
];

export const router = createBrowserRouter([
  {
    element: <Layout pages={pages} />,
    children: [
      ...pages.map(({ path, element }) => ({ path, element })),
      { path: '*', element: <NotFound /> },
    ],
  },
]);
