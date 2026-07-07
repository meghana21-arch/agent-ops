import axios from 'axios';
import type { Project, Run, Step } from './types';

const api = axios.create({
  baseURL: '/api',
  headers: { 'Content-Type': 'application/json' },
});

export const projectsApi = {
  list: (): Promise<Project[]> =>
    api.get<{ projects: Project[] }>('/v1/projects').then((r) => r.data.projects),

  create: (data: { name: string; description?: string; environment?: string }): Promise<Project> =>
    api.post<Project>('/v1/projects', data).then((r) => r.data),

  get: (projectId: string): Promise<Project> =>
    api.get<Project>(`/v1/projects/${projectId}`).then((r) => r.data),
};

export const runsApi = {
  list: (projectId: string): Promise<Run[]> =>
    api.get<{ runs: Run[] }>(`/v1/runs?projectId=${projectId}`).then((r) => r.data.runs),

  create: (data: { projectId: string; goal: string; maxSteps?: number }): Promise<Run> =>
    api.post<Run>('/v1/runs', data).then((r) => r.data),

  get: (runId: string): Promise<Run> =>
    api.get<Run>(`/v1/runs/${runId}`).then((r) => r.data),

  listSteps: (runId: string): Promise<Step[]> =>
    api.get<{ steps: Step[] }>(`/v1/runs/${runId}/steps`).then((r) => r.data.steps),

  cancel: (runId: string): Promise<void> =>
    api.post(`/v1/runs/${runId}/cancel`).then(() => undefined),

  resume: (runId: string): Promise<void> =>
    api.post(`/v1/runs/${runId}/resume`).then(() => undefined),
};

export default api;
