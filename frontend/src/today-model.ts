export type ApiLesson = {
  lessonID: number; topic: string; startAt: string; endAt: string; lessonStatus: string;
  remoteLessonLink?: string | null;
  teacher?: { teacherName: string } | null;
  classroom?: { classroomNumber: number } | null;
  building?: { buildingAddress?: string | null; building: string } | null;
  course?: { title: string } | null;
};
export type ApiAppointment = { appoinmentID: string; appointmentTitle: string; appointmentStatus: string; created_at: string };
export type ApiDeadline = { deadlineID: number; courseID?: string | null; deadlineTitle: string; deadlineEndAt?: string | null };
export type ApiGrade = { courseID: string; score: number | null; credit: boolean | null };
export type ApiCourse = { courseID: string; title: string };
export type ApiDashboard = { user: { accountID: string }; nextLessons: ApiLesson[]; appointments: ApiAppointment[]; deadlines: ApiDeadline[] };
export type ApiProfile = { student?: { group?: { groupNumber: number } | null } | null };
export type ApiNotification = { notificationReadAt: string | null };
export type TodaySummary = { accountID: string; groupLabel: string; hasUnread: boolean };
export type LessonView = { id?: number; t: string; n: string; d: string; r: string; format: string; place: string; a?: boolean; remoteLessonLink?: string | null };

const zone = 'Europe/Moscow';
const timeFormat = new Intl.DateTimeFormat('ru-RU', { timeZone: zone, hour: '2-digit', minute: '2-digit' });
const dateFormat = new Intl.DateTimeFormat('ru-RU', { timeZone: zone, day: 'numeric', month: 'long' });
const monthFormat = new Intl.DateTimeFormat('ru-RU', { timeZone: zone, month: 'short' });
export function moscowDate(now = new Date()) {
  const parts = new Intl.DateTimeFormat('en-CA', { timeZone: zone, year: 'numeric', month: '2-digit', day: '2-digit' }).formatToParts(now);
  const part = (name: string) => parts.find(p => p.type === name)?.value;
  return `${part('year')}-${part('month')}-${part('day')}`;
}
function validDate(value?: string | null) { return Boolean(value && Number.isFinite(Date.parse(value))); }
function clock(value?: string | null) { return validDate(value) ? timeFormat.format(new Date(value!)) : 'Время не указано'; }
export function appointmentStatus(status: string) {
  return ({ creating: 'Создана', proccess: 'В обработке', ready: 'Готова', canceled: 'Отклонена' } as Record<string, string>)[status] || 'Статус не указан';
}
export function safeLessonLink(raw?: string | null): string | null {
  if (!raw) return null;
  try { const url = new URL(raw); return ['http:', 'https:'].includes(url.protocol) ? url.href : null; } catch { return null; }
}
export function buildTodayModel(dashboard: ApiDashboard, schedule: ApiLesson[], courses: ApiCourse[], grades: ApiGrade[], now = new Date(), profile?: ApiProfile, notifications: ApiNotification[] = []) {
  const future = dashboard.nextLessons.filter(l => l.lessonStatus !== 'canceled' && validDate(l.startAt) && Date.parse(l.startAt) >= now.getTime())
    .sort((a, b) => Date.parse(a.startAt) - Date.parse(b.startAt));
  const next = future[0];
  const knownCourses = new Set(courses.map(c => c.courseID));
  const results = grades.filter(g => knownCourses.has(g.courseID) && typeof g.score === 'number' && Number.isFinite(g.score) && g.score >= 0 && g.score <= 100);
  const average = results.length ? results.reduce((sum, g) => sum + g.score!, 0) / results.length : null;
  const completed = results.filter(g => g.credit === true).length;
  const courseCount = courses.length;
  const progress = courseCount ? Math.min(100, completed / courseCount * 100) : 0;
  const minutes = next ? Math.max(0, Math.ceil((Date.parse(next.startAt) - now.getTime()) / 60000)) : 0;
  const when = next ? `${clock(next.startAt)}–${clock(next.endAt)} · ${moscowDate(new Date(next.startAt)) === moscowDate(now) ? (minutes ? `через ${minutes} мин.` : 'начинается сейчас') : dateFormat.format(new Date(next.startAt))}` : 'Предстоящие занятия пока не назначены';
  return {
    summary: { accountID: dashboard.user?.accountID || '', groupLabel: profile?.student?.group ? `Группа ${profile.student.group.groupNumber}` : 'Группа не указана', hasUnread: notifications.some(n => !n.notificationReadAt) } satisfies TodaySummary,
    hero: {
      title: next?.course?.title || next?.topic || 'Занятий нет',
      when, status: next ? (next.lessonStatus === 'rescheduled' ? 'Занятие перенесено' : 'По расписанию') : 'Нет занятий',
      room: next?.remoteLessonLink ? 'Онлайн' : next?.classroom ? `Аудитория ${next.classroom.classroomNumber}` : 'Аудитория не указана',
      teacher: next?.teacher?.teacherName || 'Преподаватель не указан',
      format: next ? (next.remoteLessonLink ? 'Онлайн' : 'Очно') : '—',
    },
    lessons: schedule.slice().sort((a, b) => Date.parse(a.startAt) - Date.parse(b.startAt)).map((l): LessonView => ({
      id: l.lessonID, t: clock(l.startAt), n: l.course?.title || l.topic,
      d: [l.teacher?.teacherName, l.lessonStatus === 'canceled' ? 'Отменено' : l.lessonStatus === 'rescheduled' ? 'Перенесено' : null].filter(Boolean).join(' · '),
      r: l.remoteLessonLink ? 'Онлайн' : String(l.classroom?.classroomNumber ?? '—'), format: l.remoteLessonLink ? 'Онлайн' : 'Очно',
      place: l.building?.buildingAddress || l.building?.building || (l.remoteLessonLink ? 'Онлайн-занятие' : 'Корпус не указан'),
      a: l.lessonStatus !== 'canceled' && Date.parse(l.startAt) <= now.getTime() && Date.parse(l.endAt) > now.getTime(),
      remoteLessonLink: l.lessonStatus === 'canceled' ? null : safeLessonLink(l.remoteLessonLink),
    })),
    requests: dashboard.appointments.map(a => ({
      id: a.appoinmentID, displayId: a.appoinmentID.slice(0, 8), title: a.appointmentTitle,
      date: validDate(a.created_at) ? dateFormat.format(new Date(a.created_at)) : 'Дата не указана',
      status: appointmentStatus(a.appointmentStatus), tone: a.appointmentStatus === 'ready' ? 'mint' : a.appointmentStatus === 'canceled' ? 'gray' : 'blue',
    })),
    average: average === null ? '—' : new Intl.NumberFormat('ru-RU', { maximumFractionDigits: 1 }).format(average),
    completed, courseCount, progress,
    progressStatus: courseCount === 0 ? 'Нет курсов' : results.some(g => g.credit === false) ? 'Незачёт' : completed === courseCount ? 'Зачтено' : 'В процессе',
    deadlines: dashboard.deadlines.slice(0, 2).map(d => ({
      id: d.deadlineID, date: validDate(d.deadlineEndAt) ? new Intl.DateTimeFormat('ru-RU', { timeZone: zone, day: 'numeric' }).format(new Date(d.deadlineEndAt!)) : '—',
      month: validDate(d.deadlineEndAt) ? monthFormat.format(new Date(d.deadlineEndAt!)).replace(/\.$/, '') : '',
      title: d.deadlineTitle,
      sub: `${courses.find(c => c.courseID === d.courseID)?.title || 'Учебная задача'} · ${clock(d.deadlineEndAt)}`,
    })),
  };
}
export type TodayModel = ReturnType<typeof buildTodayModel>;
