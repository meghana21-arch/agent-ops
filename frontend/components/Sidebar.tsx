'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';

const nav = [
  { label: 'Projects', href: '/projects', icon: '⬡' },
  { label: 'Runs', href: '/runs', icon: '▶' },
  { label: 'Approvals', href: '/approvals', icon: '✓', disabled: true },
  { label: 'Evals', href: '/evals', icon: '◈', disabled: true },
];

export default function Sidebar() {
  const pathname = usePathname();

  return (
    <aside className="w-56 shrink-0 bg-gray-900 border-r border-gray-800 flex flex-col">
      <div className="px-5 py-5 border-b border-gray-800">
        <span className="text-white font-bold text-base tracking-tight">AgentOps</span>
        <span className="ml-2 text-xs text-gray-500">Runtime</span>
      </div>

      <nav className="flex-1 px-3 py-4 space-y-0.5">
        {nav.map((item) => {
          const active = !item.disabled && pathname.startsWith(item.href);
          return (
            <Link
              key={item.href}
              href={item.disabled ? '#' : item.href}
              className={[
                'flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm transition-colors',
                item.disabled
                  ? 'text-gray-600 cursor-not-allowed'
                  : active
                  ? 'bg-indigo-600/20 text-indigo-400 font-medium'
                  : 'text-gray-400 hover:text-gray-200 hover:bg-gray-800',
              ].join(' ')}
            >
              <span className="text-xs">{item.icon}</span>
              {item.label}
              {item.disabled && (
                <span className="ml-auto text-xs text-gray-700">soon</span>
              )}
            </Link>
          );
        })}
      </nav>

      <div className="px-5 py-4 border-t border-gray-800">
        <span className="text-xs text-gray-600">v0.1.0 · local</span>
      </div>
    </aside>
  );
}
