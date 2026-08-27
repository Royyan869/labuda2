import { renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useAuth } from "./useAuth";

const apiGetMock = vi.fn();
const getAuthTokenMock = vi.fn();
const setUserMock = vi.fn();
const setLoadingMock = vi.fn();

vi.mock("@/lib/api", () => ({
  api: {
    get: (...args: unknown[]) => apiGetMock(...args),
  },
  getAuthToken: (...args: unknown[]) => getAuthTokenMock(...args),
}));

vi.mock("@/store/authStore", () => ({
  useAuthStore: () => ({
    user: null,
    isLoading: true,
    error: null,
    setUser: setUserMock,
    setLoading: setLoadingMock,
  }),
}));

describe("useAuth", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("hydrates identity from /users/me before checking /admin/me", async () => {
    getAuthTokenMock.mockReturnValue("backend-token");
    apiGetMock
      .mockResolvedValueOnce({
        data: {
          user: {
            id: "admin-1",
            email: "admin@labuda.com",
            username: "admin",
          },
        },
      })
      .mockResolvedValueOnce({
        data: {
          id: "admin-1",
          email: "admin@labuda.com",
          username: "admin",
          role: "admin",
          is_admin: true,
          capabilities: ["*"],
        },
      });

    renderHook(() => useAuth());

    await waitFor(() => {
      expect(setUserMock).toHaveBeenCalledWith(
        expect.objectContaining({
          id: "admin-1",
          email: "admin@labuda.com",
          username: "admin",
          isAdmin: true,
        }),
      );
    });

    expect(apiGetMock).toHaveBeenNthCalledWith(1, "/api/v1/users/me");
    expect(apiGetMock).toHaveBeenNthCalledWith(2, "/api/v1/admin/me");
    expect(setLoadingMock).toHaveBeenCalledWith(false);
  });
});
