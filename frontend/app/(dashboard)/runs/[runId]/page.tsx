'use client';

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useParams, useRouter } from 'next/navigation';
import Link from 'next/link';
import { runsApi } from '@/lib/api';
import StatusBadge from '@/components/StatusBadge';
import StepTimeline from '@/components/StepTimeline';
import type { RunStatus } from '@/lib/types';

const activeStatuses: RunStatus[] = ['CREATED', 'PLANNING', 'RUNNING', 'RETRYING'];

export default function RunDetailPage() {
  const { runId } = useParams<{ runId: string }>();
  const router = useRouter();
  const qc = useQueryClient();

  const { data: run, isLoading: runLoading } = useQuery({
    queryKey: ['run', runId],
    queryFn: () => runsApi.get(runId),
    refetchInterval: (query) =>
      activeStatuses.includes(query.state.data?.status as RunStatus) ? 2000 : false,
  });

  const { data: steps } = useQuery({
    queryKey: ['steps', runId],
    queryFn: () => runsApi.listSteps(runId),
    refetchInterval: () => {
      const cachedRun = qc.getQueryData<Awaited<ReturnType<typeof runsApi.get>>>(['run', runId]);
      return cachedRun && activeStatuses.includes(cachedRun.status) ? 2000 : false;
    },
    enabled: !!run,
  });

  const { mutate: cancelRun } = useMutation({
    mutationFn: () => runsApi.cancel(runId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['run', runId] }),
  });

  const { mutate: resumeRun } = useMutation({
    mutationFn: () => runsApi.resume(runId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['run', runId] }),
  });

  if (runLoading) {
    return (
      <div className="animate-pulse space-y-4">
        <div className="h-6 bg-gray-800 rounded w-1/3" />
        <div className="h-4 bg-gray-800 rounded w-1/2" />
      </div>
    );
  }

  if (!run) {
    return (
      <div className="text-center py-24">
        <p className="text-gray-400 text-sm">Run not found</p>
        <Link href="/projects" className="mt-4 inline-block text-indigo-400 text-sm hover:text-indigo-300">
          ← Back to Projects
        </Link>
      </div>
    );
  }

  const isActive = activeStatuses.includes(run.status);
  const isCancellable = isActive || run.status === 'WAITING_FOR_APPROVAL';
  const isResumable = run.status === 'FAILED' || run.status === 'WAITING_FOR_APPROVAL';

  return (
    <div>
      <div className="mb-6">
        <Link href="/projects" className="text-xs text-gray-500 hover:text-gray-400 transition-colors">
          ← Projects
        </Link>
      </div>

      <div className="bg-gray-900 border border-gray-800 rounded-xl p-6 mb-6">
        <div className="flex items-start justify-between gap-4">
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-3 mb-2">
              <StatusBadge status={run.status} />
              <span className="text-xs text-gray-500 font-mono">{run.id.slice(0, 8)}…</span>
            </div>
            <p className="text-base font-medium text-white">{run.goal}</p>
          </div>

          <div className="flex gap-2 shrink-0">
            {isResumable && (
              <button
                onClick={() => resumeRun()}
                className="bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-medium px-3 py-1.5 rounded-lg transition-colors"
              >
                Resume
              </button>
            )}
            {isCancellable && (
              <button
                onClick={() => cancelRun()}
                className="bg-gray-800 hover:bg-gray-700 text-gray-300 text-xs font-medium px-3 py-1.5 rounded-lg transition-colors"
              >
                Cancel
              </button>
            )}
          </div>
        </div>

        <div className="grid grid-cols-2 sm:grid-cols-4 gap-4 mt-5 pt-5 border-t border-gray-800">
          <div>
            <p className="text-xs text-gray-500 mb-1">Steps</p>
            <p className="text-sm font-medium text-white">{run.currentStepIndex} / {run.maxSteps}</p>
          </div>
          <div>
            <p className="text-xs text-gray-500 mb-1">Tokens</p>
            <p className="text-sm font-medium text-white">{run.totalTokens.toLocaleString()}</p>
          </div>
          <div>
            <p className="text-xs text-gray-500 mb-1">Cost</p>
            <p className="text-sm font-medium text-white">${run.totalCostUsd.toFixed(4)}</p>
          </div>
          <div>
            <p className="text-xs text-gray-500 mb-1">Started</p>
            <p className="text-sm font-medium text-white">
              {run.startedAt ? new Date(run.startedAt).toLocaleTimeString() : '—'}
            </p>
          </div>
        </div>
      </div>

      <div className="bg-gray-900 border border-gray-800 rounded-xl p-6">
        <h2 className="text-sm font-semibold text-white mb-4">
          Step Timeline
          {isActive && (
            <span className="ml-2 text-xs text-indigo-400 font-normal">· live</span>
          )}
        </h2>
        <StepTimeline steps={steps ?? []} />
      </div>
    </div>
  );
}
