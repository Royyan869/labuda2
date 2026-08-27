import { Button } from "@/components/ui/Button";
import { api, ApiError, setAuthToken } from "@/lib/api";
import { auth, googleProvider } from "@/lib/firebase";
import { useAuthStore, type AdminUser } from "@/store/authStore";
import { FirebaseError } from "firebase/app";
import { signInWithEmailAndPassword, signInWithPopup } from "firebase/auth";
import { useState } from "react";
import { Navigate, useNavigate } from "react-router-dom";

interface AdminMeResponse {
  id: string;
  email: string;
  username: string;
  role: string;
  is_admin: boolean;
  capabilities: string[];
}

interface UserMeResponse {
  user: {
    id: string;
    email?: string | null;
    username: string;
  };
}

interface FirebaseExchangeData {
  user_id: string;
  access_token: string;
  refresh_token: string;
  expires_at: string;
  refresh_expires_at: string;
  requires_profile_completion: boolean;
  created: boolean;
}

interface FirebaseExchangeResponse {
  success: boolean;
  data: FirebaseExchangeData;
}

const ADMIN_DENIED_MESSAGE = "Access denied. This account does not have admin privileges.";
const PROFILE_COMPLETION_DENIED_MESSAGE =
  "This account needs profile completion before admin access can be granted.";

export function LoginPage() {
  const { user, setUser } = useAuthStore();
  const navigate = useNavigate();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [isGoogleLoading, setIsGoogleLoading] = useState(false);
  const anyLoading = isLoading || isGoogleLoading;

  if (user?.isAdmin) {
    return <Navigate to="/" replace />;
  }

  // Shared by both password and Google flows: exchange a Firebase ID token
  // for a backend admin session. Both providers hit the same backend
  // endpoint because backend token verification is provider-agnostic.
  const exchangeFirebaseToken = async (idToken: string) => {
    const exchangeResp = await api.post<FirebaseExchangeResponse>(
      "/api/v1/auth/firebase/exchange",
      { firebase_id_token: idToken },
    );
    const exchange = exchangeResp.data;

    if (exchange.requires_profile_completion) {
      throw new Error("PROFILE_COMPLETION_REQUIRED");
    }
    if (!exchange.access_token || !exchange.refresh_token) {
      throw new Error("INVALID_SESSION");
    }

    setAuthToken(exchange.access_token);

    // Fetch canonical user identity first, then verify admin capability.
    const userResp = await api.get<{ data: UserMeResponse }>("/api/v1/users/me");
    const identity = userResp.data.user;

    // Fetch admin authorization/capability metadata.
    const resp = await api.get<{ data: AdminMeResponse }>("/api/v1/admin/me");
    const me = resp.data;

    if (!me.is_admin) {
      throw new Error("NOT_ADMIN");
    }

    const adminUser: AdminUser = {
      id: identity.id,
      email: identity.email ?? "",
      username: identity.username,
      isAdmin: me.is_admin,
      capabilities: me.capabilities,
    };

    setUser(adminUser);
    navigate("/", { replace: true });
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setIsLoading(true);

    try {
      const credential = await signInWithEmailAndPassword(auth, email, password);
      const idToken = await credential.user.getIdToken();
      await exchangeFirebaseToken(idToken);
    } catch (err) {
      setAuthToken(""); // clear on failure
      console.error("Login error:", err);
      if (err instanceof ApiError && err.status === 403) {
        setError(ADMIN_DENIED_MESSAGE);
      } else if (err instanceof Error && err.message === "PROFILE_COMPLETION_REQUIRED") {
        setError(PROFILE_COMPLETION_DENIED_MESSAGE);
      } else if (err instanceof Error && err.message === "NOT_ADMIN") {
        setError(ADMIN_DENIED_MESSAGE);
      } else if (err instanceof Error && err.message === "INVALID_SESSION") {
        setError("Login failed. The backend did not return a valid admin session.");
      } else if (err instanceof ApiError) {
        setError(err.message);
      } else {
        setError("Invalid email or password. Please try again.");
      }
    } finally {
      setIsLoading(false);
    }
  };

  const handleGoogleSignIn = async () => {
    setError("");
    setIsGoogleLoading(true);

    try {
      const credential = await signInWithPopup(auth, googleProvider);
      const idToken = await credential.user.getIdToken();
      await exchangeFirebaseToken(idToken);
    } catch (err) {
      setAuthToken(""); // clear on failure
      if (
        err instanceof FirebaseError &&
        (err.code === "auth/popup-closed-by-user" || err.code === "auth/cancelled-popup-request")
      ) {
        // User closed the popup â€” stay silent, remain on login page
      } else {
        console.error("Google login error:", err);
        if (err instanceof ApiError && err.status === 403) {
          setError(ADMIN_DENIED_MESSAGE);
        } else if (err instanceof Error && err.message === "PROFILE_COMPLETION_REQUIRED") {
          setError(PROFILE_COMPLETION_DENIED_MESSAGE);
        } else if (err instanceof Error && err.message === "NOT_ADMIN") {
          setError(ADMIN_DENIED_MESSAGE);
        } else if (err instanceof Error && err.message === "INVALID_SESSION") {
          setError("Login failed. The backend did not return a valid admin session.");
        } else if (err instanceof ApiError) {
          setError(err.message);
        } else {
          setError("Login dengan Google gagal. Coba lagi.");
        }
      }
    } finally {
      setIsGoogleLoading(false);
    }
  };

  return (
    <div className="flex min-h-screen items-center justify-center bg-gray-50 px-4">
      <div className="w-full max-w-md">
        {/* Logo */}
        <div className="text-center mb-8">
          <h1 className="text-3xl font-bold text-primary">LABUDA</h1>
          <p className="text-gray-600 mt-2">Admin Dashboard</p>
        </div>

        {/* Login Card */}
        <div className="bg-white rounded-lg shadow-lg border border-gray-200 p-8">
          <h2 className="text-2xl font-bold text-gray-900 mb-6">Sign In</h2>

          {error && (
            <div className="mb-4 rounded-lg bg-red-50 border border-red-200 p-3">
              <p className="text-sm text-red-800">{error}</p>
            </div>
          )}

          <form onSubmit={handleSubmit} className="space-y-4">
            {/* Email */}
            <div>
              <label htmlFor="email" className="block text-sm font-medium text-gray-700 mb-1">
                Email
              </label>
              <input
                type="email"
                id="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
                disabled={anyLoading}
                className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent disabled:opacity-50"
                placeholder="admin@labuda.com"
              />
            </div>

            {/* Password */}
            <div>
              <label htmlFor="password" className="block text-sm font-medium text-gray-700 mb-1">
                Password
              </label>
              <input
                type="password"
                id="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
                disabled={anyLoading}
                className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent disabled:opacity-50"
                placeholder="••••••••"
              />
            </div>

            {/* Submit Button */}
            <Button type="submit" className="w-full" isLoading={isLoading} disabled={anyLoading}>
              Sign In
            </Button>
          </form>

          {/* Divider */}
          <div className="flex items-center my-5">
            <div className="flex-1 border-t border-gray-200" />
            <span className="px-3 text-xs text-gray-400">atau</span>
            <div className="flex-1 border-t border-gray-200" />
          </div>

          {/* Google Sign In */}
          <Button
            type="button"
            variant="secondary"
            className="w-full gap-2"
            isLoading={isGoogleLoading}
            disabled={anyLoading}
            onClick={handleGoogleSignIn}
          >
            {!isGoogleLoading && (
              <svg width="18" height="18" viewBox="0 0 18 18" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
                <path
                  d="M17.64 9.2c0-.637-.057-1.251-.164-1.84H9v3.481h4.844c-.209 1.125-.843 2.078-1.796 2.717v2.258h2.908c1.702-1.567 2.684-3.874 2.684-6.615z"
                  fill="#4285F4"
                />
                <path
                  d="M9 18c2.43 0 4.467-.806 5.956-2.18l-2.908-2.259c-.806.54-1.837.86-3.048.86-2.344 0-4.328-1.584-5.036-3.711H.957v2.332A8.997 8.997 0 0 0 9 18z"
                  fill="#34A853"
                />
                <path
                  d="M3.964 10.71A5.41 5.41 0 0 1 3.682 9c0-.593.102-1.17.282-1.71V4.958H.957A8.996 8.996 0 0 0 0 9c0 1.452.348 2.827.957 4.042l3.007-2.332z"
                  fill="#FBBC05"
                />
                <path
                  d="M9 3.58c1.321 0 2.508.454 3.44 1.345l2.582-2.58C13.463.891 11.426 0 9 0A8.997 8.997 0 0 0 .957 4.958L3.964 6.29C4.672 4.163 6.656 3.58 9 3.58z"
                  fill="#EA4335"
                />
              </svg>
            )}
            Masuk dengan Google
          </Button>

          {/* Footer */}
          <div className="mt-6 text-center">
            <p className="text-xs text-gray-500">Admin access only. Contact your administrator if you need access.</p>
          </div>
        </div>

        {/* Version Info */}
        <p className="text-center text-sm text-gray-500 mt-6">LABUDA Admin Dashboard v1.0.0</p>
      </div>
    </div>
  );
}
