import type {
  Room,
  Booking,
  CreateBookingRequest,
  PaginatedResponse,
  ApiResponse,
} from "./types";

const BASE = "/api";

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    headers: {
      "Content-Type": "application/json",
      "X-User-ID": "frontend-user",
      ...init?.headers,
    },
    ...init,
  });

  const json: ApiResponse<T> = await res.json();

  if (!json.ok) {
    throw new Error(json.error?.message || "Request failed");
  }

  return json.data as T;
}

// --- Rooms ---

export async function getRooms(page = 1, pageSize = 50): Promise<PaginatedResponse<Room>> {
  return request(`/rooms?page=${page}&page_size=${pageSize}`);
}

export async function getRoom(id: number): Promise<Room> {
  return request(`/rooms/${id}`);
}

export async function createRoom(room: { name: string; capacity: number }): Promise<Room> {
  return request("/rooms", {
    method: "POST",
    body: JSON.stringify(room),
  });
}

export async function deleteRoom(id: number): Promise<void> {
  await request(`/rooms/${id}`, { method: "DELETE" });
}

// --- Bookings ---

export async function createBooking(booking: CreateBookingRequest): Promise<Booking> {
  return request("/bookings", {
    method: "POST",
    body: JSON.stringify(booking),
  });
}

export async function cancelBooking(id: number): Promise<{ status: string }> {
  return request(`/bookings/${id}`, { method: "DELETE" });
}

export async function getRoomSchedule(
  roomId: number,
  date: string
): Promise<Booking[]> {
  return request(`/rooms/${roomId}/schedule?date=${date}`);
}

export async function getUserBookings(
  page = 1,
  pageSize = 20
): Promise<PaginatedResponse<Booking>> {
  return request(`/bookings?page=${page}&page_size=${pageSize}`);
}
