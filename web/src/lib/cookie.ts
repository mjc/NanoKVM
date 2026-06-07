import Cookies from 'js-cookie';

const COOKIE_TOKEN_KEY = 'nano-kvm-token';
const tokenCookieOptions = {
  sameSite: 'strict' as const,
  secure: window.location.protocol === 'https:',
  path: '/'
};

export function existToken() {
  const token = Cookies.get(COOKIE_TOKEN_KEY);
  return !!token;
}

export function getToken() {
  const token = Cookies.get(COOKIE_TOKEN_KEY);
  if (!token) return null;

  return token;
}

export function setToken(token: string) {
  Cookies.set(COOKIE_TOKEN_KEY, token, tokenCookieOptions);
}

export function removeToken() {
  Cookies.remove(COOKIE_TOKEN_KEY, tokenCookieOptions);
}
