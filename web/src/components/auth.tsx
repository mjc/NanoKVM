import { ReactNode, useEffect, useState } from 'react';
import { Navigate } from 'react-router-dom';

import { existToken } from '@/lib/cookie.ts';
import { getAccount } from '@/api/auth.ts';

export const ProtectedRoute = ({ children }: { children: ReactNode }) => {
  const hasToken = existToken();
  const [isValid, setIsValid] = useState<boolean | null>(hasToken ? null : false);

  useEffect(() => {
    if (!hasToken) return;
    getAccount()
      .then((rsp) => setIsValid(rsp.code === 0))
      .catch(() => setIsValid(false));
  }, [hasToken]);

  if (!hasToken || isValid === false) {
    return <Navigate to={'/auth/login'} replace />;
  }
  if (isValid === null) return null;

  return children;
};
