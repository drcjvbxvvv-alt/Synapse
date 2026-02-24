import api from '../utils/api';
import type {
  ApiResponse,
  PaginatedResponse,
  User,
  CreateUserRequest,
  UpdateUserRequest,
} from '../types';

const BASE_URL = '/users';

export const userService = {
  getUsers: async (params?: {
    page?: number;
    pageSize?: number;
    search?: string;
    status?: string;
    auth_type?: string;
  }): Promise<ApiResponse<PaginatedResponse<User>>> => {
    const response = await api.get(BASE_URL, { params });
    return response.data;
  },

  getUser: async (id: number): Promise<ApiResponse<User>> => {
    const response = await api.get(`${BASE_URL}/${id}`);
    return response.data;
  },

  createUser: async (data: CreateUserRequest): Promise<ApiResponse<User>> => {
    const response = await api.post(BASE_URL, data);
    return response.data;
  },

  updateUser: async (id: number, data: UpdateUserRequest): Promise<ApiResponse<User>> => {
    const response = await api.put(`${BASE_URL}/${id}`, data);
    return response.data;
  },

  deleteUser: async (id: number): Promise<ApiResponse<null>> => {
    const response = await api.delete(`${BASE_URL}/${id}`);
    return response.data;
  },

  updateUserStatus: async (id: number, status: string): Promise<ApiResponse<null>> => {
    const response = await api.put(`${BASE_URL}/${id}/status`, { status });
    return response.data;
  },

  resetPassword: async (id: number, newPassword: string): Promise<ApiResponse<null>> => {
    const response = await api.put(`${BASE_URL}/${id}/reset-password`, { new_password: newPassword });
    return response.data;
  },
};

export default userService;
