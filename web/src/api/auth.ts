import { http } from '@/lib/http';
import { encrypt } from '@/lib/encrypt.ts';

export function login(username: string, password: string) {
  const data = {
    username,
    password: encrypt(password)
  };
  return http.post('/api/auth/login', data);
}

export function logout() {
  return http.post('/api/auth/logout');
}

export function getAccount() {
  return http.get('/api/auth/account');
}

export function changePassword(username: string, oldPassword: string, password: string) {
  const data = {
    username,
    oldPassword: encrypt(oldPassword),
    password: encrypt(password)
  };
  return http.post('/api/auth/password', data);
}

export function isPasswordUpdated() {
  return http.get('/api/auth/password');
}
