export const config = {
  matcher: '/(.*)',
};

export default function middleware(request: Request) {
  // Bypass auth if the toggle is set to true
  const bypass = process.env.VITE_BYPASS_AUTH === 'true';
  if (bypass) {
    return;
  }

  // Check for the authorization header
  const authorizationHeader = request.headers.get('authorization');
  if (authorizationHeader) {
    const basicAuth = authorizationHeader.split(' ')[1];
    const [user, password] = atob(basicAuth).split(':');

    // Verify credentials against environment variables
    if (
      user === process.env.ADMIN_USER &&
      password === process.env.ADMIN_PASSWORD
    ) {
      return;
    }
  }

  // If auth fails or is missing, prompt for Basic Auth
  return new Response('Auth required', {
    status: 401,
    headers: {
      'WWW-Authenticate': 'Basic realm="Secure Admin Portal"',
    },
  });
}
