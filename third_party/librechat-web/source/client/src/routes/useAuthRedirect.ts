import { useEffect } from 'react';
import { useAuthContext } from '~/hooks';

export default function useAuthRedirect() {
  const { user, isAuthenticated } = useAuthContext();

  useEffect(() => {
    const timeout = setTimeout(() => {
      if (!isAuthenticated) {
        if (!window.location.pathname.startsWith('/app/login')) {
          window.location.assign('/app/login');
        }
      }
    }, 300);

    return () => {
      clearTimeout(timeout);
    };
  }, [isAuthenticated]);

  return {
    user,
    isAuthenticated,
  };
}
