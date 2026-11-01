import { api } from './http.ts';
import type { AdminStats, AdminUserPatch, AuditLog, ListParams, Paginated, User } from './types.ts';

const userPath = (id: string) => `/admin/users/${encodeURIComponent(id)}`;

export const getStats = () => api.request<AdminStats>('/admin/stats');

export const listUsers = (params: ListParams & { q?: string } = {}) =>
	api.request<Paginated<User>>('/admin/users', { query: { ...params } });

export const getUser = (id: string) => api.request<User>(userPath(id));

export const updateUser = (id: string, body: AdminUserPatch) =>
	api.request<User>(userPath(id), { method: 'PATCH', body });

export const deleteUser = (id: string) => api.request<void>(userPath(id), { method: 'DELETE' });

export const revokeSessions = (id: string) =>
	api.request<void>(`${userPath(id)}/revoke-sessions`, { method: 'POST' });

export const listAuditLogs = (params: ListParams & { user_id?: string } = {}) =>
	api.request<Paginated<AuditLog>>('/admin/audit-logs', { query: { ...params } });
