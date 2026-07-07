'use client';

import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import Link from 'next/link';
import { projectsApi } from '@/lib/api';
import CreateProjectModal from '@/components/CreateProjectModal';

const envColors = {
  development: 'bg-blue-900/40 text-blue-400',
  staging: 'bg-yellow-900/40 text-yellow-400',
  production: 'bg-green-900/40 text-green-400',
};

export default function ProjectsPage() {
  const [showModal, setShowModal] = useState(false);

  const { data: projects, isLoading, error } = useQuery({
    queryKey: ['projects'],
    queryFn: projectsApi.list,
  });

  return (
    <div>
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-xl font-semibold text-white">Projects</h1>
          <p className="mt-1 text-sm text-gray-500">Agent workflow projects in your organization</p>
        </div>
        <button
          onClick={() => setShowModal(true)}
          className="bg-indigo-600 hover:bg-indigo-500 text-white text-sm font-medium px-4 py-2 rounded-lg transition-colors"
        >
          + New Project
        </button>
      </div>

      {isLoading && (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {[1, 2, 3].map((i) => (
            <div key={i} className="bg-gray-900 border border-gray-800 rounded-xl p-5 animate-pulse">
              <div className="h-4 bg-gray-800 rounded w-1/2 mb-3" />
              <div className="h-3 bg-gray-800 rounded w-3/4 mb-4" />
              <div className="h-3 bg-gray-800 rounded w-1/4" />
            </div>
          ))}
        </div>
      )}

      {error && (
        <div className="bg-red-900/20 border border-red-800 rounded-xl p-4 text-red-400 text-sm">
          Failed to load projects — is the API running on :8080?
        </div>
      )}

      {!isLoading && !error && projects?.length === 0 && (
        <div className="flex flex-col items-center justify-center py-24 text-center">
          <div className="text-4xl mb-4">⬡</div>
          <h3 className="text-base font-medium text-gray-300 mb-2">No projects yet</h3>
          <p className="text-sm text-gray-500 mb-6 max-w-xs">
            Create a project to start running AI agent workflows with durable state.
          </p>
          <button
            onClick={() => setShowModal(true)}
            className="bg-indigo-600 hover:bg-indigo-500 text-white text-sm font-medium px-4 py-2 rounded-lg transition-colors"
          >
            Create your first project
          </button>
        </div>
      )}

      {!isLoading && !error && projects && projects.length > 0 && (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {projects.map((p) => (
            <div key={p.id} className="bg-gray-900 border border-gray-800 rounded-xl p-5 hover:border-gray-700 transition-colors">
              <div className="flex items-start justify-between mb-3">
                <h3 className="text-sm font-semibold text-white truncate">{p.name}</h3>
                <span className={`ml-2 shrink-0 text-xs px-2 py-0.5 rounded-full font-medium ${envColors[p.environment]}`}>
                  {p.environment}
                </span>
              </div>
              {p.description && (
                <p className="text-xs text-gray-500 mb-4 line-clamp-2">{p.description}</p>
              )}
              <div className="flex items-center justify-between">
                <span className="text-xs text-gray-600">
                  {new Date(p.createdAt).toLocaleDateString()}
                </span>
                <Link
                  href={`/runs?projectId=${p.id}`}
                  className="text-xs text-indigo-400 hover:text-indigo-300 font-medium transition-colors"
                >
                  View Runs →
                </Link>
              </div>
            </div>
          ))}
        </div>
      )}

      {showModal && <CreateProjectModal onClose={() => setShowModal(false)} />}
    </div>
  );
}
