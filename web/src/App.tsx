import { useState, useEffect, useCallback } from "react";
import { format, addDays, startOfDay } from "date-fns";
import { ru } from "date-fns/locale";
import {
  Plus,
  CalendarDays,
  Users,
  Clock,
  X,
  ChevronLeft,
  ChevronRight,
  Trash2,
  AlertCircle,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Separator } from "@/components/ui/separator";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import * as api from "@/api/client";
import type { Room, Booking } from "@/api/types";

// --- Time utilities ---

const SLOT_START = 7; // 07:00
const SLOT_END = 22; // 22:00
// 30-minute slot granularity
const TIME_SLOTS: string[] = [];
for (let h = SLOT_START; h < SLOT_END; h++) {
  TIME_SLOTS.push(`${String(h).padStart(2, "0")}:00`);
  TIME_SLOTS.push(`${String(h).padStart(2, "0")}:30`);
}

function slotToMinutes(slot: string): number {
  const [h, m] = slot.split(":").map(Number);
  return h * 60 + m;
}

function formatTime(iso: string): string {
  return format(new Date(iso), "HH:mm");
}

function formatDuration(start: string, end: string): string {
  const diff =
    (new Date(end).getTime() - new Date(start).getTime()) / 60000;
  const hours = Math.floor(diff / 60);
  const mins = diff % 60;
  if (hours === 0) return `${mins} мин`;
  if (mins === 0) return `${hours} ч`;
  return `${hours} ч ${mins} мин`;
}

// --- Components ---

function RoomCard({
  room,
  bookings,
  selectedDate,
  index,
  onBook,
}: {
  room: Room;
  bookings: Booking[];
  selectedDate: Date;
  index: number;
  onBook: (room: Room, slot: string) => void;
}) {
  const now = new Date();
  const isToday =
    format(selectedDate, "yyyy-MM-dd") === format(now, "yyyy-MM-dd");
  const currentSlot = isToday
    ? `${String(now.getHours()).padStart(2, "0")}:${String(Math.floor(now.getMinutes() / 30) * 30).padStart(2, "0")}`
    : null;

  const occupiedRanges = bookings.map((b) => ({
    start: slotToMinutes(formatTime(b.start_time)),
    end: slotToMinutes(formatTime(b.end_time)),
    booking: b,
  }));

  return (
    <Card
      className="animate-card-enter group relative overflow-hidden border-border/50 bg-card shadow-sm transition-shadow duration-300 hover:shadow-md"
      style={{ animationDelay: `${index * 80}ms` }}
    >
      <CardContent className="p-0">
        {/* Header */}
        <div className="flex items-center justify-between border-b border-border/50 px-5 py-4">
          <div className="flex items-center gap-3">
            <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-brand/10 text-brand">
              <CalendarDays className="h-4 w-4" />
            </div>
            <div>
              <h3 className="text-sm font-semibold text-foreground">
                {room.name}
              </h3>
              <div className="flex items-center gap-1 text-xs text-muted-foreground">
                <Users className="h-3 w-3" />
                до {room.capacity} чел.
              </div>
            </div>
          </div>
          <Badge variant="secondary" className="text-xs font-normal">
            {bookings.length} брон.
          </Badge>
        </div>

        {/* Timeline */}
        <div className="px-5 py-3">
          <div className="relative flex flex-col gap-px">
            {TIME_SLOTS.map((slot) => {
              const slotMin = slotToMinutes(slot);
              const isPast =
                isToday && currentSlot && slotMin <= slotToMinutes(currentSlot);
              const isOccupied = occupiedRanges.some(
                (r) => slotMin >= r.start && slotMin < r.end
              );
              const isStart = occupiedRanges.some(
                (r) => slotMin === r.start
              );

              return (
                <div key={slot}>
                  {/* Booking label */}
                  {isStart && (
                    <div className="mb-0.5 flex items-center gap-1.5">
                      {occupiedRanges
                        .filter((r) => r.start === slotMin)
                        .map((r) => (
                          <Badge
                            key={r.booking.id}
                            className="h-5 gap-1 rounded-md bg-brand/15 text-[11px] font-medium text-brand hover:bg-brand/20"
                          >
                            <Clock className="h-2.5 w-2.5" />
                            {r.booking.title}
                            <Separator
                              orientation="vertical"
                              className="mx-0.5 h-3 bg-brand/30"
                            />
                            {formatTime(r.booking.start_time)}–
                            {formatTime(r.booking.end_time)}
                          </Badge>
                        ))}
                    </div>
                  )}

                  {/* Slot row */}
                  <div className="flex items-center gap-2">
                    <span className="w-10 shrink-0 text-right text-[11px] tabular-nums text-muted-foreground/60">
                      {slot}
                    </span>
                    <button
                      disabled={isPast || isOccupied}
                      onClick={() => onBook(room, slot)}
                      className={`h-6 flex-1 rounded-md transition-all duration-150 ${
                        isPast
                          ? "cursor-not-allowed bg-muted/30"
                          : isOccupied
                            ? "cursor-not-allowed bg-brand/10"
                            : "cursor-pointer bg-muted/50 hover:bg-brand/15 hover:shadow-[0_0_0_2px_oklch(0.55_0.22_265/0.2)] active:scale-[0.98]"
                      }`}
                      aria-label={`Забронировать ${room.name} в ${slot}`}
                    />
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      </CardContent>
    </Card>
  );
}

function BookingDialog({
  room,
  slot,
  open,
  onOpenChange,
  onSubmit,
  isSubmitting,
}: {
  room: Room;
  slot: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSubmit: (title: string, endTime: string) => void;
  isSubmitting: boolean;
}) {
  const [title, setTitle] = useState("");
  const [endTime, setEndTime] = useState("");

  // Reset form when dialog opens
  useEffect(() => {
    if (open) {
      setTitle("");
      const [h] = slot.split(":").map(Number);
      setEndTime(`${String(h + 1).padStart(2, "0")}:00`);
    }
  }, [open, slot]);

  const handleSubmit = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (!title.trim()) return;
    onSubmit(title.trim(), endTime);
  };

  const slotMin = slotToMinutes(slot);
  const endMin = slotToMinutes(endTime);
  const isInvalid = endMin <= slotMin || endMin - slotMin > 24 * 60;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[420px]">
        <DialogHeader>
          <DialogTitle>Новое бронирование</DialogTitle>
          <DialogDescription>
            {room.name} · начиная с {slot}
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4 pt-2">
          <div className="space-y-2">
            <Label htmlFor="title">Название</Label>
            <Input
              id="title"
              placeholder="Например: Стендап, Ревью, 1:1..."
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              autoFocus
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="endTime">Окончание</Label>
            <select
              id="endTime"
              value={endTime}
              onChange={(e) => setEndTime(e.target.value)}
              className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-xs transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
            >
              {TIME_SLOTS.filter((s) => slotToMinutes(s) > slotMin).map((s) => (
                <option key={s} value={s}>
                  {s} ({((slotToMinutes(s) - slotMin) / 60).toFixed(1)} ч)
                </option>
              ))}
            </select>
          </div>
          <Button
            type="submit"
            disabled={!title.trim() || isInvalid || isSubmitting}
            className="w-full"
          >
            {isSubmitting ? "Бронирую..." : "Забронировать"}
          </Button>
        </form>
      </DialogContent>
    </Dialog>
  );
}

// --- Main App ---

export default function App() {
  const [rooms, setRooms] = useState<Room[]>([]);
  const [selectedDate, setSelectedDate] = useState(startOfDay(new Date()));
  const [schedules, setSchedules] = useState<Record<number, Booking[]>>({});
  const [userBookings, setUserBookings] = useState<Booking[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Booking dialog
  const [bookDialog, setBookDialog] = useState<{
    open: boolean;
    room: Room | null;
    slot: string;
  }>({ open: false, room: null, slot: "" });
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [toast, setToast] = useState<{
    message: string;
    type: "success" | "error";
  } | null>(null);

  const dateStr = format(selectedDate, "yyyy-MM-dd");

  const fetchData = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [roomsRes, bookingsRes] = await Promise.all([
        api.getRooms(),
        api.getUserBookings(1, 100),
      ]);
      setRooms(roomsRes.data);
      setUserBookings(bookingsRes.data);

      // Fetch schedules for all rooms
      const schedulePromises = roomsRes.data.map((room) =>
        api.getRoomSchedule(room.id, dateStr).catch(() => [] as Booking[])
      );
      const scheduleResults = await Promise.all(schedulePromises);
      const newSchedules: Record<number, Booking[]> = {};
      roomsRes.data.forEach((room, i) => {
        newSchedules[room.id] = scheduleResults[i];
      });
      setSchedules(newSchedules);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load data");
    } finally {
      setLoading(false);
    }
  }, [dateStr]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const handleBook = async (title: string, endTime: string) => {
    if (!bookDialog.room) return;
    setIsSubmitting(true);
    try {
      // Convert local time slot to proper ISO string (handles timezone)
      const date = format(selectedDate, "yyyy-MM-dd");
      const startDate = new Date(`${date}T${bookDialog.slot}:00`);
      const endDate = new Date(`${date}T${endTime}:00`);
      await api.createBooking({
        room_id: bookDialog.room.id,
        title,
        start_time: startDate.toISOString(),
        end_time: endDate.toISOString(),
      });
      setBookDialog({ open: false, room: null, slot: "" });
      setToast({ message: "Забронировано!", type: "success" });
      fetchData();
    } catch (err) {
      setToast({
        message: err instanceof Error ? err.message : "Ошибка",
        type: "error",
      });
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleCancel = async (id: number) => {
    try {
      await api.cancelBooking(id);
      setToast({ message: "Бронирование отменено", type: "success" });
      fetchData();
    } catch (err) {
      setToast({
        message: err instanceof Error ? err.message : "Ошибка",
        type: "error",
      });
    }
  };

  const navigateDate = (days: number) => {
    setSelectedDate(addDays(selectedDate, days));
  };

  const goToday = () => setSelectedDate(startOfDay(new Date()));

  return (
    <div className="min-h-screen bg-background">
      {/* Header */}
      <header className="sticky top-0 z-30 border-b border-border/50 bg-background/80 backdrop-blur-md">
        <div className="mx-auto flex max-w-7xl items-center justify-between px-4 py-3 sm:px-6">
          <div className="flex items-center gap-3">
            <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-brand text-brand-foreground">
              <CalendarDays className="h-4 w-4" />
            </div>
            <h1 className="text-lg font-semibold tracking-tight">
              Бронирование
            </h1>
          </div>

          {/* Date navigation */}
          <div className="flex items-center gap-1 rounded-xl bg-muted/60 p-1">
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7"
              onClick={() => navigateDate(-1)}
              aria-label="Предыдущий день"
            >
              <ChevronLeft className="h-4 w-4" />
            </Button>
            <button
              onClick={goToday}
              className="flex h-7 items-center gap-1.5 rounded-lg px-3 text-sm font-medium transition-colors hover:bg-accent"
            >
              {format(selectedDate, "d MMM, EEEE", { locale: ru })}
            </button>
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7"
              onClick={() => navigateDate(1)}
              aria-label="Следующий день"
            >
              <ChevronRight className="h-4 w-4" />
            </Button>
          </div>

          <Button
            onClick={() =>
              setBookDialog({
                open: true,
                room: rooms[0] || null,
                slot: "09:00",
              })
            }
            disabled={rooms.length === 0}
            size="sm"
            className="gap-1.5 rounded-lg"
          >
            <Plus className="h-4 w-4" />
            <span className="hidden sm:inline">Быстрое бронирование</span>
          </Button>
        </div>
      </header>

      <main className="mx-auto max-w-7xl px-4 py-6 sm:px-6">
        {/* Toast */}
        {toast && (
          <div className="animate-card-enter mb-4 flex items-center gap-2 rounded-lg border px-4 py-2.5 text-sm shadow-sm">
            {toast.type === "error" ? (
              <AlertCircle className="h-4 w-4 text-destructive" />
            ) : (
              <CalendarDays className="h-4 w-4 text-success" />
            )}
            <span>{toast.message}</span>
            <button
              onClick={() => setToast(null)}
              className="ml-auto text-muted-foreground hover:text-foreground"
              aria-label="Закрыть"
            >
              <X className="h-3.5 w-3.5" />
            </button>
          </div>
        )}

        {/* Error state */}
        {error && !loading && (
          <div className="flex flex-col items-center justify-center py-20 text-center">
            <AlertCircle className="mb-3 h-10 w-10 text-muted-foreground/40" />
            <p className="text-sm text-muted-foreground">
              Не удалось загрузить данные. Проверьте что API запущен.
            </p>
            <Button variant="outline" size="sm" className="mt-4" onClick={fetchData}>
              Попробовать снова
            </Button>
          </div>
        )}

        {/* Loading */}
        {loading && (
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {[1, 2, 3].map((i) => (
              <Card key={i} className="animate-pulse">
                <CardContent className="p-5">
                  <div className="h-4 w-32 rounded bg-muted" />
                  <div className="mt-3 space-y-2">
                    {[1, 2, 3, 4, 5].map((j) => (
                      <div key={j} className="h-6 rounded bg-muted/60" />
                    ))}
                  </div>
                </CardContent>
              </Card>
            ))}
          </div>
        )}

        {/* Room grid */}
        {!loading && !error && (
          <>
            <div className="mb-4 flex items-center justify-between">
              <p className="text-sm text-muted-foreground">
                {rooms.length} переговорк
                {rooms.length === 1 ? "а" : rooms.length < 5 ? "и" : ""}
              </p>
            </div>
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
              {rooms.map((room, i) => (
                <RoomCard
                  key={room.id}
                  room={room}
                  bookings={schedules[room.id] || []}
                  selectedDate={selectedDate}
                  index={i}
                  onBook={(room, slot) =>
                    setBookDialog({ open: true, room, slot })
                  }
                />
              ))}
            </div>

            {/* My bookings */}
            {userBookings.length > 0 && (
              <section className="mt-8">
                <h2 className="mb-3 text-sm font-semibold text-foreground">
                  Мои бронирования
                </h2>
                <div className="space-y-2">
                  {userBookings
                    .filter((b) => b.status === "active")
                    .slice(0, 5)
                    .map((booking) => (
                      <div
                        key={booking.id}
                        className="group flex items-center justify-between rounded-lg border border-border/50 bg-card px-4 py-3 transition-colors hover:bg-muted/30"
                      >
                        <div className="flex items-center gap-3">
                          <div className="h-2 w-2 rounded-full bg-brand" />
                          <div>
                            <p className="text-sm font-medium">
                              {booking.title}
                            </p>
                            <p className="text-xs text-muted-foreground">
                              {format(new Date(booking.start_time), "d MMM, HH:mm", { locale: ru })}
                              {" – "}
                              {format(new Date(booking.end_time), "HH:mm")}
                              {" · "}
                              {formatDuration(
                                booking.start_time,
                                booking.end_time
                              )}
                            </p>
                          </div>
                        </div>
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-7 w-7 text-muted-foreground opacity-0 transition-opacity hover:text-destructive group-hover:opacity-100"
                          onClick={() => handleCancel(booking.id)}
                          aria-label="Отменить бронирование"
                        >
                          <Trash2 className="h-3.5 w-3.5" />
                        </Button>
                      </div>
                    ))}
                </div>
              </section>
            )}
          </>
        )}
      </main>

      {/* Booking dialog */}
      {bookDialog.room && (
        <BookingDialog
          room={bookDialog.room}
          slot={bookDialog.slot}
          open={bookDialog.open}
          onOpenChange={(open) =>
            setBookDialog({ ...bookDialog, open })
          }
          onSubmit={handleBook}
          isSubmitting={isSubmitting}
        />
      )}
    </div>
  );
}
