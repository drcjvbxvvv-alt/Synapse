import { request } from '../utils/api';

// ─── Types ─────────────────────────────────────────────────────────────────

export interface Project {
  id: number;
  git_provider_id: number;
  name: string;
  repo_url: string;
  default_branch: string;
  description: string;
  created_by: number;
  created_at: string;
  updated_at: string;
}

export interface CreateProjectRequest {
  name: string;
  repo_url: string;
  default_branch?: string;
  description?: string;
}

export interface UpdateProjectRequest {
  name?: string;
  repo_url?: string;
  default_branch?: string;
  description?: string;
}

// ─── Service ───────────────────────────────────────────────────────────────

const projectService = {
  list(providerId: number): Promise<{ items: Project[]; total: number }> {
    return request.get(`/system/git-providers/${providerId}/projects`);
  },

  get(providerId: number, projectId: number): Promise<Project> {
    return request.get(`/system/git-providers/${providerId}/projects/${projectId}`);
  },

  create(providerId: number, data: CreateProjectRequest): Promise<Project> {
    return request.post(`/system/git-providers/${providerId}/projects`, data);
  },

  update(providerId: number, projectId: number, data: UpdateProjectRequest): Promise<Project> {
    return request.put(`/system/git-providers/${providerId}/projects/${projectId}`, data);
  },

  delete(providerId: number, projectId: number): Promise<void> {
    return request.delete(`/system/git-providers/${providerId}/projects/${projectId}`);
  },
};

export default projectService;
