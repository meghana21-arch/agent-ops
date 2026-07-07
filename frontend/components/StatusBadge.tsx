import type { RunStatus, StepStatus } from '@/lib/types';

type Status = RunStatus | StepStatus;

const config: Record<string, { label: string; classes: string; pulse?: boolean }> = {
  CREATED: { label: 'Created', classes: 'bg-gray-800 text-gray-400' },
  PLANNING: { label: 'Planning', classes: 'bg-blue-900/50 text-blue-400', pulse: true },
  RUNNING: { label: 'Running', classes: 'bg-yellow-900/50 text-yellow-400', pulse: true },
  WAITING_FOR_APPROVAL: { label: 'Awaiting Approval', classes: 'bg-orange-900/50 text-orange-400' },
  RETRYING: { label: 'Retrying', classes: 'bg-purple-900/50 text-purple-400', pulse: true },
  COMPLETED: { label: 'Completed', classes: 'bg-green-900/50 text-green-400' },
  FAILED: { label: 'Failed', classes: 'bg-red-900/50 text-red-400' },
  CANCELLED: { label: 'Cancelled', classes: 'bg-gray-800 text-gray-500' },
  PENDING: { label: 'Pending', classes: 'bg-gray-800 text-gray-400' },
  SUCCEEDED: { label: 'Succeeded', classes: 'bg-green-900/50 text-green-400' },
  SKIPPED: { label: 'Skipped', classes: 'bg-gray-800 text-gray-500' },
  REQUIRES_APPROVAL: { label: 'Needs Approval', classes: 'bg-orange-900/50 text-orange-400' },
};

export default function StatusBadge({ status }: { status: Status }) {
  const c = config[status] ?? { label: status, classes: 'bg-gray-800 text-gray-400' };
  return (
    <span
      className={`inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-xs font-medium ${c.classes}`}
    >
      {c.pulse && (
        <span className="relative flex h-1.5 w-1.5">
          <span className="animate-ping absolute inline-flex h-full w-full rounded-full opacity-75 bg-current" />
          <span className="relative inline-flex h-1.5 w-1.5 rounded-full bg-current" />
        </span>
      )}
      {c.label}
    </span>
  );
}
