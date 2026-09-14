import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { LoginPage } from "./LoginPage";
import { useAuth } from "@/hooks/useAuth";
import { useAuthStore } from "@/store/authStore";

const signInWithEmailAndPasswordMock = vi.fn();
const signInWithPopupMock = vi.fn();
const apiGetMock = vi.fn();
const apiPostMock = vi.fn();
const navigateMock = vi.fn();

vi.mock("@/lib/firebase", () => ({
  auth: {},
  googleProvider: {},
}));

vi.mock("firebase/auth", () => ({
  signInWithEmailAndPassword: (...args: unknown[]) => signInWithEmailAndPasswordMock(...args),
  signInWithPopup: (...args: unknown[]) => signInWithPopupMock(...args),
}));

// The credential accessors stay localStorage-backed so the canonical validation
// authority (which reads the real credential) is exercised end to end.
vi.mock("@/lib/api", () => {
  class ApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.name = "ApiError";
      this.status = status;
    }
  }
  return {
    api: {
      get: (...args: unknown[]) => apiGetMock(...args),
      post: (...args: unknown[]) => apiPostMock(...args),
    },
    ApiError,
    setAuthToken: (token: string) => localStorage.setItem("admin_token", token),
    clearAuthToken: () => localStorage.removeItem("admin_token"),
    getAuthToken: () => localStorage.getItem("admin_token"),
  };
});

vi.mock("react-router-dom", async (importOriginal) => {
  const actual = await importOriginal<typeof import("react-router-dom")>();
  return { ...actual, useNavigate: () => navigateMock };
});

function renderLoginPage() {
  return render(
    <MemoryRouter>
      <LoginPage />
    </MemoryRouter>
  );
}

const USERS_ME = "/api/v1/users/me";
const ADMIN_ME = "/api/v1/admin/me";

function countRequests(url: string): number {
  return apiGetMock.mock.calls.filter(([requested]) => requested === url).length;
}

const ADMIN_ME_RESPONSE = {
  data: {
    id: "admin-1",
    email: "admin@labuda.com",
    username: "admin",
    role: "admin",
    is_admin: true,
    capabilities: ["*"],
  },
};

const USER_ME_RESPONSE = {
  data: {
    user: {
      id: "admin-1",
      email: "admin@labuda.com",
    },
    profile: {
      id: "profile-1",
      user_id: "admin-1",
      username: "admin",
      avatar_url: null,
    },
  },
};

const FIREBASE_EXCHANGE_RESPONSE = {
  success: true,
  data: {
    user_id: "admin-1",
    access_token: "backend-access-token",
    refresh_token: "backend-refresh-token",
    expires_at: "2026-07-19T00:00:00Z",
    refresh_expires_at: "2026-08-19T00:00:00Z",
    requires_profile_completion: false,
    created: false,
  },
};

function Consumer() {
  useAuth();
  return null;
}

describe("LoginPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    useAuthStore.setState({
      user: null,
      isLoading: true,
      error: null,
      sessionToken: null,
    });
    apiPostMock.mockResolvedValue(FIREBASE_EXCHANGE_RESPONSE);
  });

  it("renders the Google sign-in button alongside password login", () => {
    renderLoginPage();

    expect(screen.getByText("Masuk dengan Google")).toBeInTheDocument();
    expect(screen.getByLabelText("Email")).toBeInTheDocument();
    expect(screen.getByLabelText("Password")).toBeInTheDocument();
  });

  it("signs in an admin via Google and navigates to the dashboard", async () => {
    const user = userEvent.setup();
    signInWithPopupMock.mockResolvedValue({
      user: { getIdToken: () => Promise.resolve("fake-google-id-token") },
    });
    apiGetMock
      .mockResolvedValueOnce(USER_ME_RESPONSE)
      .mockResolvedValueOnce(ADMIN_ME_RESPONSE);

    renderLoginPage();
    await user.click(screen.getByText("Masuk dengan Google"));

    await waitFor(() =>
      expect(useAuthStore.getState().user?.id).toBe("admin-1")
    );
    expect(apiPostMock).toHaveBeenCalledWith(
      "/api/v1/auth/firebase/exchange",
      { firebase_id_token: "fake-google-id-token" },
    );
    expect(localStorage.getItem("admin_token")).toBe("backend-access-token");
    expect(useAuthStore.getState().sessionToken).toBe("backend-access-token");
    expect(navigateMock).toHaveBeenCalledWith("/", { replace: true });
  });

  it("establishes the session through the canonical authority with exactly one validation pair", async () => {
    const user = userEvent.setup();
    signInWithPopupMock.mockResolvedValue({
      user: { getIdToken: () => Promise.resolve("fake-google-id-token") },
    });
    apiGetMock
      .mockResolvedValueOnce(USER_ME_RESPONSE)
      .mockResolvedValueOnce(ADMIN_ME_RESPONSE);

    renderLoginPage();
    await user.click(screen.getByText("Masuk dengan Google"));

    await waitFor(() =>
      expect(useAuthStore.getState().user?.id).toBe("admin-1")
    );

    // The login flow itself must have used exactly one pair.
    expect(countRequests(USERS_ME)).toBe(1);
    expect(countRequests(ADMIN_ME)).toBe(1);

    // A consumer mounting into the protected shell afterwards must not start a
    // second validation: the store already carries the lifecycle marker.
    render(<Consumer />);

    await waitFor(() =>
      expect(useAuthStore.getState().isLoading).toBe(false)
    );
    expect(countRequests(USERS_ME)).toBe(1);
    expect(countRequests(ADMIN_ME)).toBe(1);
  });

  it("rejects a non-admin Google account with an access-denied message", async () => {
    const user = userEvent.setup();
    signInWithPopupMock.mockResolvedValue({
      user: { getIdToken: () => Promise.resolve("fake-token") },
    });
    apiGetMock
      .mockResolvedValueOnce(USER_ME_RESPONSE)
      .mockResolvedValueOnce({ data: { ...ADMIN_ME_RESPONSE.data, is_admin: false } });

    renderLoginPage();
    await user.click(screen.getByText("Masuk dengan Google"));

    expect(await screen.findByText(/does not have admin privileges/i)).toBeInTheDocument();
    expect(useAuthStore.getState().user).toBeNull();
    expect(useAuthStore.getState().sessionToken).toBeNull();
    expect(navigateMock).not.toHaveBeenCalled();
    expect(localStorage.getItem("admin_token")).toBeFalsy();
  });

  it("shows an error when Google sign-in itself fails", async () => {
    const user = userEvent.setup();
    signInWithPopupMock.mockRejectedValue(new Error("popup blocked"));

    renderLoginPage();
    await user.click(screen.getByText("Masuk dengan Google"));

    expect(await screen.findByText("Login dengan Google gagal. Coba lagi.")).toBeInTheDocument();
    expect(countRequests(USERS_ME)).toBe(0);
  });

  it("still supports password login for an admin", async () => {
    const user = userEvent.setup();
    signInWithEmailAndPasswordMock.mockResolvedValue({
      user: { getIdToken: () => Promise.resolve("fake-password-id-token") },
    });
    apiGetMock
      .mockResolvedValueOnce(USER_ME_RESPONSE)
      .mockResolvedValueOnce(ADMIN_ME_RESPONSE);

    renderLoginPage();
    await user.type(screen.getByLabelText("Email"), "admin@labuda.com");
    await user.type(screen.getByLabelText("Password"), "correct-password");
    await user.click(screen.getByRole("button", { name: "Sign In" }));

    await waitFor(() =>
      expect(useAuthStore.getState().user?.id).toBe("admin-1")
    );
    expect(apiPostMock).toHaveBeenCalledWith(
      "/api/v1/auth/firebase/exchange",
      { firebase_id_token: "fake-password-id-token" },
    );
    expect(localStorage.getItem("admin_token")).toBe("backend-access-token");
    expect(countRequests(USERS_ME)).toBe(1);
    expect(countRequests(ADMIN_ME)).toBe(1);
    expect(navigateMock).toHaveBeenCalledWith("/", { replace: true });
  });

  it("shows an error on invalid password login credentials", async () => {
    const user = userEvent.setup();
    signInWithEmailAndPasswordMock.mockRejectedValue(new Error("auth/wrong-password"));

    renderLoginPage();
    await user.type(screen.getByLabelText("Email"), "admin@labuda.com");
    await user.type(screen.getByLabelText("Password"), "wrong-password");
    await user.click(screen.getByRole("button", { name: "Sign In" }));

    expect(await screen.findByText("Invalid email or password. Please try again.")).toBeInTheDocument();
    expect(countRequests(USERS_ME)).toBe(0);
  });

  it("fails closed when the exchange requires profile completion", async () => {
    const user = userEvent.setup();
    apiPostMock.mockResolvedValueOnce({
      success: true,
      data: {
        ...FIREBASE_EXCHANGE_RESPONSE.data,
        requires_profile_completion: true,
      },
    });
    signInWithPopupMock.mockResolvedValue({
      user: { getIdToken: () => Promise.resolve("profile-incomplete-token") },
    });

    renderLoginPage();
    await user.click(screen.getByText("Masuk dengan Google"));

    expect(
      await screen.findByText(
        "This account needs profile completion before admin access can be granted.",
      ),
    ).toBeInTheDocument();
    expect(useAuthStore.getState().user).toBeNull();
    expect(navigateMock).not.toHaveBeenCalled();
    expect(countRequests(USERS_ME)).toBe(0);
  });
});
