import { Resolution } from '@/types';

const LANGUAGE_KEY = 'nano-kvm-language';
const VIDEO_MODE_KEY = 'nano-kvm-vide-mode';
const VIDEO_SCALE_KEY = 'nano-kvm-video-scale';
const WEB_RESOLUTION_KEY = 'nano-kvm-web-resolution';
const FPS_KEY = 'nano-kvm-fps';
const QUALITY_KEY = 'nano-kvm-quality';
const GOP_KEY = 'nano-kvm-gop';
const FRAME_DETECT_KEY = 'nano-kvm-frame-detect';
const MOUSE_STYLE_KEY = 'nano-kvm-mouse-style';
const MOUSE_MODE_KEY = 'nano-kvm-mouse-mode';
const MOUSE_SCROLL_DIRECTION_KEY = 'nano-kvm-mouse-scroll-direction';
const MOUSE_SCROLL_INTERVAL_KEY = 'nano-kvm-mouse-scroll-interval';
const SKIP_UPDATE_KEY = 'nano-kvm-check-update';
const KEYBOARD_SYSTEM_KEY = 'nano-kvm-keyboard-system';
const KEYBOARD_LANGUAGE_KEY = 'nano-kvm-keyboard-language';
const SKIP_MODIFY_PASSWORD_KEY = 'nano-kvm-skip-modify-password';
const MENU_DISABLED_ITEMS_KEY = 'nano-kvm-menu-disabled-items';
const MENU_AUTO_HIDE_KEY = 'nano-kvm-menu-auto-hide';
const POWER_CONFIRM_KEY = 'nano-kvm-power-confirm';

type ItemWithExpiry = {
  value: string;
  expiry: number;
};

function parseJSON<T>(value: string | null): T | null {
  if (!value) return null;

  try {
    return JSON.parse(value) as T;
  } catch {
    return null;
  }
}

function clampNumber(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

function allowlisted(value: string | null, allowed: readonly string[], fallback: string | null = null) {
  return value && allowed.includes(value) ? value : fallback;
}

function setBooleanPreference(key: string, enabled: boolean) {
  localStorage.setItem(key, enabled ? 'true' : 'false');
}

// set the value with expiration time (unit: milliseconds)
function setWithExpiry(key: string, value: string, ttl: number) {
  const now = new Date();

  const item: ItemWithExpiry = {
    value: value,
    expiry: now.getTime() + ttl
  };

  localStorage.setItem(key, JSON.stringify(item));
}

// get the value with expiration time
function getWithExpiry(key: string) {
  const itemStr = localStorage.getItem(key);
  if (!itemStr) return null;

  const item = parseJSON<ItemWithExpiry>(itemStr);
  if (!item || typeof item.expiry !== 'number') {
    localStorage.removeItem(key);
    return null;
  }
  const now = new Date();
  if (now.getTime() > item.expiry) {
    localStorage.removeItem(key);
    return null;
  }

  return item.value;
}

export function getLanguage() {
  return localStorage.getItem(LANGUAGE_KEY);
}

export function setLanguage(language: string) {
  localStorage.setItem(LANGUAGE_KEY, language);
}

export function getVideoMode() {
  return allowlisted(localStorage.getItem(VIDEO_MODE_KEY), ['mjpeg', 'h264', 'direct']);
}

export function setVideoMode(mode: string) {
  const safeMode = allowlisted(mode, ['mjpeg', 'h264', 'direct']);
  if (safeMode) localStorage.setItem(VIDEO_MODE_KEY, safeMode);
}

export function getVideoScale(): number | null {
  const scale = localStorage.getItem(VIDEO_SCALE_KEY);
  if (scale && Number(scale)) {
    return Number(scale);
  }
  return null;
}

export function setVideoScale(scale: number): void {
  localStorage.setItem(VIDEO_SCALE_KEY, String(clampNumber(scale, 25, 300)));
}

export function getResolution(): Resolution | null {
  const resolution = localStorage.getItem(WEB_RESOLUTION_KEY);
  if (resolution) {
    const obj = parseJSON<Resolution>(resolution);
    if (obj && Number.isFinite(obj.width) && Number.isFinite(obj.height)) {
      return obj;
    }
  }

  return null;
}

export function setResolution(resolution: Resolution) {
  if (Number.isFinite(resolution.width) && Number.isFinite(resolution.height)) {
    localStorage.setItem(WEB_RESOLUTION_KEY, JSON.stringify(resolution));
  }
}

export function getFps() {
  const fps = localStorage.getItem(FPS_KEY);
  return fps ? Number(fps) : null;
}

export function setFps(fps: number) {
  localStorage.setItem(FPS_KEY, String(clampNumber(fps, 1, 60)));
}

export function getQuality() {
  const quality = localStorage.getItem(QUALITY_KEY);
  return quality ? Number(quality) : null;
}

export function setQuality(quality: number) {
  localStorage.setItem(QUALITY_KEY, String(clampNumber(quality, 1, 100)));
}

export function getGop() {
  const gop = localStorage.getItem(GOP_KEY);
  return gop ? Number(gop) : null;
}

export function setGop(gop: number) {
  localStorage.setItem(GOP_KEY, String(clampNumber(gop, 1, 600)));
}

export function getFrameDetect(): boolean {
  const enabled = localStorage.getItem(FRAME_DETECT_KEY);
  return enabled === 'true';
}

export function setFrameDetect(enabled: boolean) {
  setBooleanPreference(FRAME_DETECT_KEY, enabled);
}

export function getMouseStyle() {
  return allowlisted(localStorage.getItem(MOUSE_STYLE_KEY), [
    'cursor-default',
    'cursor-grab',
    'cursor-cell',
    'cursor-text',
    'cursor-none'
  ]);
}

export function setMouseStyle(mouse: string) {
  const safeMouse = allowlisted(mouse, [
    'cursor-default',
    'cursor-grab',
    'cursor-cell',
    'cursor-text',
    'cursor-none'
  ]);
  if (safeMouse) localStorage.setItem(MOUSE_STYLE_KEY, safeMouse);
}

export function getMouseMode() {
  return allowlisted(localStorage.getItem(MOUSE_MODE_KEY), ['absolute', 'relative']);
}

export function setMouseMode(mouse: string) {
  const safeMouse = allowlisted(mouse, ['absolute', 'relative']);
  if (safeMouse) localStorage.setItem(MOUSE_MODE_KEY, safeMouse);
}

export function getMouseScrollDirection(): number | null {
  const direction = localStorage.getItem(MOUSE_SCROLL_DIRECTION_KEY);
  if (direction && Number(direction)) {
    return Number(direction);
  }
  return null;
}

export function setMouseScrollDirection(direction: number): void {
  localStorage.setItem(MOUSE_SCROLL_DIRECTION_KEY, String(clampNumber(direction, -1, 1)));
}

export function getMouseScrollInterval() {
  const interval = localStorage.getItem(MOUSE_SCROLL_INTERVAL_KEY);
  return interval ? Number(interval) : null;
}

export function setMouseScrollInterval(interval: number): void {
  localStorage.setItem(MOUSE_SCROLL_INTERVAL_KEY, String(clampNumber(interval, 10, 1000)));
}

export function getSkipUpdate() {
  const skip = getWithExpiry(SKIP_UPDATE_KEY);
  return skip === 'true';
}

export function setSkipUpdate(skip: boolean) {
  const expiry = 3 * 24 * 60 * 60 * 1000; // 3 days
  setWithExpiry(SKIP_UPDATE_KEY, String(skip), expiry);
}

export function setKeyboardSystem(system: string) {
  const safeSystem = allowlisted(system, ['win', 'mac']);
  if (safeSystem) localStorage.setItem(KEYBOARD_SYSTEM_KEY, safeSystem);
}

export function getKeyboardSystem() {
  return allowlisted(localStorage.getItem(KEYBOARD_SYSTEM_KEY), ['win', 'mac']);
}

export function setKeyboardLanguage(language: string) {
  const safeLanguage = allowlisted(language, ['en', 'fr', 'de', 'ru', 'ko', 'ja']);
  if (safeLanguage) localStorage.setItem(KEYBOARD_LANGUAGE_KEY, safeLanguage);
}

export function getKeyboardLanguage() {
  return allowlisted(localStorage.getItem(KEYBOARD_LANGUAGE_KEY), ['en', 'fr', 'de', 'ru', 'ko', 'ja']);
}

export function setSkipModifyPassword(skip: boolean) {
  const expiry = 3 * 24 * 60 * 60 * 1000; // 3 days
  setWithExpiry(SKIP_MODIFY_PASSWORD_KEY, String(skip), expiry);
}

export function getSkipModifyPassword() {
  const skip = getWithExpiry(SKIP_MODIFY_PASSWORD_KEY);
  return skip === 'true';
}

export function setMenuDisabledItems(items: string[]) {
  const allowedItems = items.filter((item) => ['screen', 'mouse', 'keyboard'].includes(item));
  localStorage.setItem(MENU_DISABLED_ITEMS_KEY, JSON.stringify(allowedItems));
}

export function getMenuDisabledItems(): string[] {
  const value = localStorage.getItem(MENU_DISABLED_ITEMS_KEY);
  const parsed = parseJSON<string[]>(value);
  return Array.isArray(parsed) ? parsed.filter((item) => typeof item === 'string') : [];
}

export function getMenuDisplayMode(): string {
  const value = localStorage.getItem(MENU_AUTO_HIDE_KEY);
  return allowlisted(value, ['auto', 'always'], 'auto') || 'auto';
}

export function setMenuDisplayMode(mode: string) {
  const safeMode = allowlisted(mode, ['auto', 'always']);
  if (safeMode) localStorage.setItem(MENU_AUTO_HIDE_KEY, safeMode);
}

export function getPowerConfirm() {
  const enabled = localStorage.getItem(POWER_CONFIRM_KEY);
  return enabled === 'true';
}

export function setPowerConfirm(enabled: boolean) {
  setBooleanPreference(POWER_CONFIRM_KEY, enabled);
}
