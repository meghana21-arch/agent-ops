'use client';

import { useState } from 'react';
import type { Step } from '@/lib/types';
import StatusBadge from './StatusBadge';

const typeIcons: Record<string, string> = {
  PLAN: '💡',
  TOOL_CALL: '🔧',
  OBSERVATION: '👁',
  VERIFICATION: '✓',
  ERROR: '✕',
};

function CodeBlock({ data }: { data: unknown }) {
  return (
    <pre className="mt-2 bg-gray-950 border border-gray-800 rounded-lg p-3 text-xs text-gray-300 overflow-auto max-h-40 whitespace-pre-wrap">
      {JSON.stringify(data, null, 2)}
    </pre>
  );
}

function StepCard({ step, isLast }: { step: Step; isLast: boolean }) {
  const [expanded, setExpanded] = useState(false);
  const hasDetails = step.toolInput || step.toolOutput || step.errorMessage || step.action;

  return (
    <div className="relative flex gap-4">
      <div className="flex flex-col items-center">
        <div className="w-8 h-8 rounded-full bg-gray-800 border border-gray-700 flex items-center justify-center text-sm shrink-0">
          {typeIcons[step.stepType] ?? '·'}
        </div>
        {!isLast && <div className="w-px flex-1 bg-gray-800 mt-1" />}
      </div>

      <div className="flex-1 pb-6">
        <div className="flex items-center gap-2 flex-wrap">
          <span className="text-xs text-gray-500 font-mono">#{step.stepIndex}</span>
          <span className="text-sm text-gray-300 font-medium">{step.stepType.replace('_', ' ')}</span>
          {step.toolName && (
            <code className="text-xs bg-gray-800 text-gray-300 px-2 py-0.5 rounded">{step.toolName}</code>
          )}
          <StatusBadge status={step.status} />
          {step.retryCount > 0 && (
            <span className="text-xs bg-purple-900/40 text-purple-400 px-2 py-0.5 rounded-full">
              retry ×{step.retryCount}
            </span>
          )}
          <span className="ml-auto text-xs text-gray-600">
            {step.startedAt ? new Date(step.startedAt).toLocaleTimeString() : new Date(step.createdAt).toLocaleTimeString()}
          </span>
        </div>

        {step.errorMessage && (
          <p className="mt-1.5 text-xs text-red-400 bg-red-900/20 border border-red-900 rounded px-3 py-2">
            {step.errorMessage}
          </p>
        )}

        {hasDetails && (
          <button
            onClick={() => setExpanded(!expanded)}
            className="mt-2 text-xs text-indigo-400 hover:text-indigo-300 transition-colors"
          >
            {expanded ? '▾ Hide details' : '▸ Show details'}
          </button>
        )}

        {expanded && (
          <div className="mt-1 space-y-2">
            {step.action && (
              <div>
                <p className="text-xs text-gray-500 mb-1">Action</p>
                <CodeBlock data={step.action} />
              </div>
            )}
            {step.toolInput && (
              <div>
                <p className="text-xs text-gray-500 mb-1">Input</p>
                <CodeBlock data={step.toolInput} />
              </div>
            )}
            {step.toolOutput && (
              <div>
                <p className="text-xs text-gray-500 mb-1">Output</p>
                <CodeBlock data={step.toolOutput} />
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

export default function StepTimeline({ steps }: { steps: Step[] }) {
  if (steps.length === 0) {
    return (
      <div className="py-8 text-center text-sm text-gray-500">
        No steps yet — the agent loop will populate this timeline
      </div>
    );
  }

  return (
    <div className="mt-2">
      {steps.map((step, i) => (
        <StepCard key={step.id} step={step} isLast={i === steps.length - 1} />
      ))}
    </div>
  );
}
