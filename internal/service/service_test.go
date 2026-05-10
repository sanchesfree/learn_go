package service

import (
	"strings"
	"testing"
	"time"

	"booking-service/internal/model"
)

func TestValidateBookingRequest(t *testing.T) {
	now := time.Now().Truncate(time.Second)

	tests := []struct {
		name    string
		req     model.CreateBookingRequest
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid booking",
			req: model.CreateBookingRequest{
				RoomID:    1,
				Title:     "Team standup",
				StartTime: now.Add(1 * time.Hour),
				EndTime:   now.Add(2 * time.Hour),
			},
			wantErr: false,
		},
		{
			name: "zero room_id",
			req: model.CreateBookingRequest{
				RoomID:    0,
				Title:     "Test",
				StartTime: now.Add(1 * time.Hour),
				EndTime:   now.Add(2 * time.Hour),
			},
			wantErr: true,
			errMsg:  "room_id must be positive",
		},
		{
			name: "empty title",
			req: model.CreateBookingRequest{
				RoomID:    1,
				Title:     "",
				StartTime: now.Add(1 * time.Hour),
				EndTime:   now.Add(2 * time.Hour),
			},
			wantErr: true,
			errMsg:  "title is required",
		},
		{
			name: "end before start",
			req: model.CreateBookingRequest{
				RoomID:    1,
				Title:     "Test",
				StartTime: now.Add(2 * time.Hour),
				EndTime:   now.Add(1 * time.Hour),
			},
			wantErr: true,
			errMsg:  "end_time must be after start_time",
		},
		{
			name: "equal start and end",
			req: model.CreateBookingRequest{
				RoomID:    1,
				Title:     "Test",
				StartTime: now.Add(1 * time.Hour),
				EndTime:   now.Add(1 * time.Hour),
			},
			wantErr: true,
			errMsg:  "zero duration not allowed",
		},
		{
			name: "too long booking (over 24h)",
			req: model.CreateBookingRequest{
				RoomID:    1,
				Title:     "Marathon",
				StartTime: now.Add(1 * time.Hour),
				EndTime:   now.Add(26 * time.Hour),
			},
			wantErr: true,
			errMsg:  "cannot exceed 24 hours",
		},
		{
			name: "exactly 24 hours — valid",
			req: model.CreateBookingRequest{
				RoomID:    1,
				Title:     "All-dayer",
				StartTime: now.Add(1 * time.Hour),
				EndTime:   now.Add(25 * time.Hour),
			},
			wantErr: false,
		},
		{
			name: "negative room_id",
			req: model.CreateBookingRequest{
				RoomID:    -1,
				Title:     "Test",
				StartTime: now.Add(1 * time.Hour),
				EndTime:   now.Add(2 * time.Hour),
			},
			wantErr: true,
			errMsg:  "room_id must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBookingRequest(tt.req)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
					return
				}
				// Проверяем что сообщение содержит ожидаемый текст
				if tt.errMsg != "" {
					found := false
					for i := 0; i <= len(err.Error())-len(tt.errMsg); i++ {
						if err.Error()[i:i+len(tt.errMsg)] == tt.errMsg {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("error message %q should contain %q", err.Error(), tt.errMsg)
					}
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
			}
		})
	}
}

// TestContainsStr — юнит-тест для хелпера
func TestContainsStr(t *testing.T) {
	tests := []struct {
		s, substr string
		want      bool
	}{
		{"hello world", "world", true},
		{"hello world", "hello", true},
		{"hello world", "foobar", false},
		{"", "", true},
		{"", "a", false},
		{"a", "", true},
		{"abcdef", "cde", true},
		{"abcdef", "abcdefg", false},
	}

	for _, tt := range tests {
		got := strings.Contains(tt.s, tt.substr)
		if got != tt.want {
			t.Errorf("strings.Contains(%q, %q) = %v, want %v", tt.s, tt.substr, got, tt.want)
		}
	}
}

// BenchmarkValidateBookingRequest — бенчмарк для валидации
// Запуск: go test -bench=. -benchmem
func BenchmarkValidateBookingRequest(b *testing.B) {
	req := model.CreateBookingRequest{
		RoomID:    1,
		Title:     "Team standup",
		StartTime: time.Now().Add(1 * time.Hour),
		EndTime:   time.Now().Add(2 * time.Hour),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validateBookingRequest(req)
	}
}
