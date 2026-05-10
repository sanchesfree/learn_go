export interface Room {
  id: number;
  name: string;
  capacity: number;
  created_at: string;
}

export interface Booking {
  id: number;
  room_id: number;
  user_id: string;
  title: string;
  start_time: string;
  end_time: string;
  status: string;
  created_at: string;
}

export interface CreateBookingRequest {
  room_id: number;
  title: string;
  start_time: string;
  end_time: string;
}

export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  page: number;
  page_size: number;
  total_pages: number;
}

export interface ApiError {
  code: string;
  message: string;
}

export interface ApiResponse<T> {
  ok: boolean;
  data?: T;
  error?: ApiError;
}
