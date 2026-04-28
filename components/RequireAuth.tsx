"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { onAuthChange } from "@/lib/firebase";
import type { User } from "firebase/auth";

interface Props {
  fallback?: React.ReactNode;
  children?: React.ReactNode;
}

export function RequireAuth({ children, fallback }: Props) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const router = useRouter();

  useEffect(() => {
    const unsub = onAuthChange((u) => {
      setUser(u);
      setLoading(false);
      if (!u) router.replace("/login");
    });
    return unsub;
  }, [router]);

  if (loading) {
    return (
      fallback ?? (
        <div className="min-h-screen flex items-center justify-center bg-background">
          <div className="w-6 h-6 rounded-full border-2 border-primary border-t-transparent animate-spin" />
        </div>
      )
    );
  }

  if (!user) return null;
  return <>{children}</>;
}

export function useCurrentUser() {
  const [user, setUser] = useState<User | null>(null);
  useEffect(() => onAuthChange(setUser), []);
  return user;
}
