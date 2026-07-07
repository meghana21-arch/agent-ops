'use client';

import { useState, Suspense } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useSearchParams } from 'next/navigation';
import Link from 'next/link';
import { runsApi } from '@/lib/api';
import StatusBadge from '@/components/StatusBadge';

function timeAgo(dateStr: string) {
  const diff = Date.now() - new Date(dateStr).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return 'just now';
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  return `${Math.floor(hrs / 24)}d ago`;
}

function RunsContent() {
  const searchParams = useSearchParams();
  const projectId = searchParams.get('projectId') ?? '';
  const qc = useQueryClient();

  const [showForm, setShowForm] = useState(false);
  const [goal, setGoal] = useState('');
  const [maxSteps, setMaxSteps] = useState(20);

  const { data: runs, isLoading, error } = useQuery({
    queryKey: ['runs', projectId],
    queryFn: () => runsApi.list(projectId),
    enabled: !!projectId,
    refetchInterval: 5000,
  });

  const { mutate: createRun, isPending } = useMutation({
    mutationFn: () => runsApi.create({ projectId, goal, maxSteps }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['runs', projectId] });
      setGoal('');
      setShowForm(false);
    },
  });

  if (!projectId) {
    return (
      <div className="flex flex-col items-center justify-center py-24 text-center">
        <p className="text-gray-500 text-sm">Select a project from the Projects page to view its runs.</p>
        <Link href="/projects" className="mt-4 text-indigo-400 text-sm hover:text-indigo-300">
          ← Go to Projects
        </Link>
      </div>
    );
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-xl font-semibold text-white">Runs</h1>
          <p className="mt-1 text-sm text-gray-500">Agent workflow runs for this project</p>
        </div>
        <button
          onClick={() => setShowForm(!showForm)}
          className="bg-indigo-600 hover:bg-indigo-500 text-white text-sm font-medium px-4 py-2 rounded-lg transition-colors"
        >
          + Start Run
        </button>
      </div>

      {showForm && (
        <div className="bg-gray-900 border border-gray-700 rounded-xl p-5 mb-6">
          <h3 className="text-sm font-medium text-white mb-4">New Agent Run</h3>
          <div className="space-y-3">
            <div>
              <label className="block text-xs text-gray-400 mb-1">Goal</label>
              <input
                autoFocus
                value={goal}
                onChange={(e) => setGoal(e.target.value)}
                placeholder="Find and fix the failing payment service tests"
                className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-indigo-500"
              />
            </div>
            <div>
              <label className="block text-xs text-gray-400 mb-1">Max Steps</label>
              <input
                type="number"
                value={maxSteps}
                onChange={(e) => setMaxSteps(Number(e.target.value))}
                min={1}
                max={50}
                className="w-32 bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-indigo-500"
              />
            </div>
            <div className="flex gap-2 pt-1">
              <button
                onClick={() => setShowForm(false)}
                className="bg-gray-800 hover:bg-gray-700 text-gray-300 text-sm px-4 py-2 rounded-lg transition-colors"
              >
                Cancel
              </button>
              <button
                disabled={isPending || !goal.trim()}
                onClick={() => createRun()}
                className="bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white text-sm font-medium px-4 py-2 rounded-lg transition-colors"
              >
                {isPending ? 'Starting…' : 'Start Run'}
              </button>
            </div>
          </div>
        </div>
      )}

      {isLoading && (
        <div className="space-y-3">
          {[1, 2, 3].map((i) => (
            <div key={i} className="bg-gray-900 border border-gray-800 rounded-xl p-4 animate-pulse">
              <div className="flex items-center gap-3">
                <div className="h-4 bg-gray-800 rounded w-1/2" />
                <div className="h-5 bg-gray-800 rounded w-20 ml-auto" />
              </div>
            </div>
          ))}
        </div>
      )}

      {error && (
        <div className="bg-red-900/20 border border-red-800 rounded-xl p-4 text-red-400 text-sm">
          Failed to load runs
        </div>
      )}

      {!isLoading && !error && runs?.length === 0 && (
        <div className="flex flex-col items-center justify-center py-24 text-center">
          <div className="text-4xl mb-4">▶</div>
          <h3 className="text-base font-medium text-gray-300 mb-2">No runs yet</h3>
          <p className="text-sm text-gray-500 mb-6">Start an agent run by giving it a goal.</p>
          <button
            onClick={() => setShowForm(true)}
            className="bg-indigo-600 hover:bg-indigo-500 text-white text-sm font-medium px-4 py-2 rounded-lg transition-colors"
          >
            Start your first run
          </button>
        </div>
      )}

      {!isLoading && runs && runs.length > 0 && (
        <div className="space-y-3">
          {runs.map((run) => (
            <Link
              key={run.id}
              href={`/runs/${run.id}`}
              className="block bg-gray-900 border border-gray-800 hover:border-gray-700 rounded-xl p-4 transition-colors"
            >
              <div className="flex items-start gap-3">
                <div className="flex-1 min-w-0">
                  <p className="text-sm text-white font-medium truncate">{run.goal}</p>
                  <p className="text-xs text-gray-500 mt-1">
                    Step {run.currentStepIndex}/{run.maxSteps} · {timeAgo(run.createdAt)}
                    {run.totalCostUsd > 0 && ` · $${run.totalCostUsd.toFixed(4)}`}
                  </p>
                </div>
                <StatusBadge status={run.status} />
              </div>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}

export default function RunsPage() {
  return (
    <Suspense fallback={<div className="text-gray-500 text-sm">Loading…</div>}>
      <RunsContent />
    </Suspense>
  );
}
