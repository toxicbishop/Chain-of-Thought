import { initializeApp, getApps, getApp } from "firebase/app";
import {
  getAuth,
  signInWithEmailAndPassword,
  signOut as firebaseSignOut,
  onAuthStateChanged,
  type User,
} from "firebase/auth";
import { getFirestore } from "firebase/firestore";

const firebaseConfig = {
  apiKey: process.env.NEXT_PUBLIC_FIREBASE_API_KEY || "",
  authDomain: process.env.NEXT_PUBLIC_FIREBASE_AUTH_DOMAIN || "",
  projectId: process.env.NEXT_PUBLIC_FIREBASE_PROJECT_ID || "",
  storageBucket: process.env.NEXT_PUBLIC_FIREBASE_STORAGE_BUCKET || "",
  messagingSenderId: process.env.NEXT_PUBLIC_FIREBASE_MESSAGING_SENDER_ID || "",
  appId: process.env.NEXT_PUBLIC_FIREBASE_APP_ID || "",
};

// Singleton — safe to import anywhere (no duplicate app error on hot reload)
let app;
let isInitialized = false;
try {
  if (firebaseConfig.apiKey) {
    app = getApps().length ? getApp() : initializeApp(firebaseConfig);
    isInitialized = true;
  } else {
    app = {} as any;
  }
} catch (error) {
  console.error("Firebase initialization failed:", error);
  app = {} as any;
}

export const auth = isInitialized ? getAuth(app) : ({} as any);
export const db = isInitialized ? getFirestore(app) : ({} as any);



/**
 * Returns the Firebase ID token for the currently signed-in user,
 * or null if nobody is signed in.
 *
 * Pass `forceRefresh = true` to always get a fresh token (recommended
 * before calls that are sensitive to token expiry).
 */
export async function getIdToken(forceRefresh = false): Promise<string | null> {
  const user = auth.currentUser;
  if (!user) return null;
  return user.getIdToken(forceRefresh);
}

/** Sign in with email + password. Throws on invalid credentials. */
export async function signIn(email: string, password: string): Promise<User> {
  const { user } = await signInWithEmailAndPassword(auth, email, password);
  return user;
}

/** Sign out the current user. */
export async function signOut(): Promise<void> {
  await firebaseSignOut(auth);
}

/** Subscribe to auth state changes. Returns the unsubscribe function. */
export function onAuthChange(callback: (user: User | null) => void): () => void {
  return onAuthStateChanged(auth, callback);
}
