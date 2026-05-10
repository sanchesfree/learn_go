-- 001_init.sql
-- Первая миграция: создание таблиц для booking service

CREATE EXTENSION IF NOT EXISTS "btree_gist";

CREATE TABLE IF NOT EXISTS rooms (
    id          BIGSERIAL PRIMARY KEY,
    name        VARCHAR(100) NOT NULL,
    capacity    INT NOT NULL CHECK (capacity > 0),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS bookings (
    id          BIGSERIAL PRIMARY KEY,
    room_id     BIGINT NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    user_id     VARCHAR(100) NOT NULL,
    title       VARCHAR(200) NOT NULL,
    start_time  TIMESTAMPTZ NOT NULL,
    end_time    TIMESTAMPTZ NOT NULL,
    status      VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Важный constraint: бронирования не должны пересекаться для одной комнаты
    EXCLUDE USING gist (
        room_id WITH =,
        tstzrange(start_time, end_time, '[)') WITH &&
    ) WHERE (status = 'active')
);

CREATE INDEX idx_bookings_room_id ON bookings(room_id);
CREATE INDEX idx_bookings_user_id ON bookings(user_id);
CREATE INDEX idx_bookings_start_time ON bookings(start_time);

-- Комментари для разработчиков
COMMENT ON TABLE bookings IS 'Бронирования переговорок. EXCLUDE constraint предотвращает двойное бронирование на пересекающееся время.';
COMMENT ON COLUMN bookings.status IS 'active — действующее, cancelled — отменённое. Отменённые не участвуют в EXCLUDE.';
