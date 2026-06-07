import CryptoJS from 'crypto-js';

// This key is only used for legacy payload compatibility.
const PAYLOAD_ENCRYPTION_KEY = 'nanokvm-payload-compat-v2';

export function encrypt(data: string) {
  const dataEncrypt = CryptoJS.AES.encrypt(data, PAYLOAD_ENCRYPTION_KEY).toString();
  return encodeURIComponent(dataEncrypt);
}
