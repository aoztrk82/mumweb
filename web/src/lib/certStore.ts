// Persistent client certificate (Mumble identity) in IndexedDB.
import type { CertPair } from './protocol'

const DB_NAME = 'mumweb'
const STORE = 'identity'
/** Keys: the main app and the /admin panel use separate identities, so both can be logged in at once. */
export const MAIN_KEY = 'cert'
export const ADMIN_KEY = 'cert-admin'

function openDb(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const req = indexedDB.open(DB_NAME, 1)
    req.onupgradeneeded = () => req.result.createObjectStore(STORE)
    req.onsuccess = () => resolve(req.result)
    req.onerror = () => reject(req.error)
  })
}

async function withStore<T>(mode: IDBTransactionMode, fn: (s: IDBObjectStore) => IDBRequest<T>): Promise<T> {
  const db = await openDb()
  try {
    return await new Promise<T>((resolve, reject) => {
      const tx = db.transaction(STORE, mode)
      const req = fn(tx.objectStore(STORE))
      tx.oncomplete = () => resolve(req.result)
      tx.onerror = () => reject(tx.error)
      tx.onabort = () => reject(tx.error)
    })
  } finally {
    db.close()
  }
}

function isCertPair(v: unknown): v is CertPair {
  return typeof v === 'object' && v !== null && typeof (v as CertPair).pem === 'string' && typeof (v as CertPair).key === 'string'
}

/** Returns the stored certificate, or null if none (or storage is unavailable). */
export async function loadCert(key = MAIN_KEY): Promise<CertPair | null> {
  try {
    const v = await withStore('readonly', (s) => s.get(key))
    return isCertPair(v) ? v : null
  } catch (e) {
    console.warn('certificate storage unavailable', e)
    return null
  }
}

export async function saveCert(cert: CertPair, key = MAIN_KEY): Promise<void> {
  await withStore('readwrite', (s) => s.put({ pem: cert.pem, key: cert.key }, key))
}

export async function clearCert(key = MAIN_KEY): Promise<void> {
  await withStore('readwrite', (s) => s.delete(key))
}
