import { NavLink } from 'react-router-dom';
import {
  LayoutDashboard,
  ListTree,
  Cpu,
} from 'lucide-react';
import { cn } from '../../lib/cn';

const links = [
  { to: '/', label: 'Overview', icon: LayoutDashboard },
  { to: '/traces', label: 'Traces', icon: ListTree },
  { to: '/gpus', label: 'GPUs', icon: Cpu },
];

export function Sidebar() {
  return (
    <aside className="flex h-screen w-56 flex-col border-r border-border bg-surface-light">
      <div className="flex h-14 items-center gap-2 border-b border-border px-4">
        <Cpu className="h-6 w-6 text-primary" />
        <span className="text-lg font-semibold tracking-tight">Axonize</span>
      </div>

      <nav className="flex-1 space-y-1 p-3">
        {links.map(({ to, label, icon: Icon }) => (
          <NavLink
            key={to}
            to={to}
            end={to === '/'}
            className={({ isActive }) =>
              cn(
                'flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors',
                isActive
                  ? 'bg-primary/15 text-primary-light'
                  : 'text-text-secondary hover:bg-surface-lighter hover:text-text',
              )
            }
          >
            <Icon className="h-4 w-4" />
            {label}
          </NavLink>
        ))}
      </nav>

      <div className="border-t border-border p-4 text-xs text-text-secondary">
        Axonize v0.1.0
      </div>
    </aside>
  );
}
