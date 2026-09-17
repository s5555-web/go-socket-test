import { ml_kem768 } from '@noble/post-quantum/ml-kem.js';

export function keygen() {
  return ml_kem768.keygen();
}

export function encapsulate(publicKey) {
  return ml_kem768.encapsulate(publicKey);
}

export function decapsulate(cipherText, secretKey) {
  return ml_kem768.decapsulate(cipherText, secretKey);
}

export const parameters = Object.freeze({
  name: 'ML-KEM-768',
  publicKeyBytes: 1184,
  secretKeyBytes: 2400,
  cipherTextBytes: 1088,
  sharedSecretBytes: 32
});
