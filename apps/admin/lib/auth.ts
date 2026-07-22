import api from "./api";

export interface AuthUser {
  id: string;
  email: string;
  name: string;
  role: string;
}

export interface LoginResponse {
  data: {
    access_token: string;
    token_type: string;
    expires_at: string;
    user: AuthUser;
  };
}

export async function login(
  email: string,
  password: string
): Promise<{ user: AuthUser; token: string }> {
  const res = await api.post<LoginResponse>("/auth/login", {
    email,
    password,
  });
  const { access_token, user } = res.data.data;
  localStorage.setItem("access_token", access_token);
  localStorage.setItem("user", JSON.stringify(user));
  return { user, token: access_token };
}

export function logout(): void {
  localStorage.removeItem("access_token");
  localStorage.removeItem("user");
  window.location.href = "/login";
}

export function getStoredUser(): AuthUser | null {
  if (typeof window === "undefined") return null;
  const raw = localStorage.getItem("user");
  if (!raw) return null;
  try {
    return JSON.parse(raw) as AuthUser;
  } catch {
    return null;
  }
}

export function getStoredToken(): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem("access_token");
}

export function isAuthenticated(): boolean {
  return !!getStoredToken();
}