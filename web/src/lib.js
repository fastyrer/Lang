export const languages = {
  english: 'Английский',
  chinese: 'Китайский',
}

export const formats = {
  trial: 'Пробный урок',
  group: 'Мини-группа',
  individual: 'Индивидуально',
}

export const levels = {
  beginner: 'С нуля',
  elementary: 'Базовый',
  intermediate: 'Средний и выше',
}

const locale = 'ru-RU'
const timeZone = 'Europe/Moscow'
const longDate = new Intl.DateTimeFormat(locale, { month: 'long', day: 'numeric', timeZone })
const slotDate = new Intl.DateTimeFormat(locale, { weekday: 'short', month: 'long', day: 'numeric', timeZone })
const clock = new Intl.DateTimeFormat(locale, { hour: '2-digit', minute: '2-digit', timeZone })

export const dateLabel = value => longDate.format(new Date(value))
export const slotLabel = value => slotDate.format(new Date(value))
export const timeLabel = value => clock.format(new Date(value))
export const money = value => new Intl.NumberFormat(locale).format(value) + ' ₽'

export function lessonLabel(count) {
  const last = count % 10
  const ending = count % 100 >= 11 && count % 100 <= 14
    ? 'уроков'
    : last === 1 ? 'урок' : last >= 2 && last <= 4 ? 'урока' : 'уроков'
  return count + ' ' + ending
}

export async function api(path, { method = 'GET', body, signal } = {}) {
  let response
  try {
    response = await fetch('/api' + path, {
      method,
      signal,
      headers: body ? { 'Content-Type': 'application/json' } : undefined,
      body: body ? JSON.stringify(body) : undefined,
    })
  } catch (error) {
    if (error.name === 'AbortError') throw error
    throw new Error('Не удалось связаться с сервером. Попробуйте ещё раз.')
  }
  if (response.status === 204) return null
  const data = await response.json().catch(() => null)
  if (!response.ok || !data) {
    const error = new Error(data?.error || 'Ошибка сервера. Попробуйте ещё раз.')
    error.status = response.status
    throw error
  }
  return data
}

export function addToCalendar(booking, title) {
  const stamp = value => new Date(value).toISOString().replace(/[-:]/g, '').split('.')[0] + 'Z'
  const escape = value => value.replace(/\\/g, '\\\\').replace(/\r?\n/g, '\\n').replace(/[,;]/g, '\\$&')
  const starts = Date.parse(booking.slot.startsAt)
  const lines = [
    'BEGIN:VCALENDAR', 'VERSION:2.0', 'PRODID:-//Lingua//Lessons//RU', 'BEGIN:VEVENT',
    'UID:' + booking.id + '@lingua', 'DTSTAMP:' + stamp(Date.now()), 'DTSTART:' + stamp(starts),
    'DTEND:' + stamp(starts + booking.slot.duration * 60_000),
    'SUMMARY:' + escape(title), 'DESCRIPTION:' + escape('Подробности и ссылка на встречу в личном кабинете.'),
    'URL:' + window.location.origin + '/cabinet', 'END:VEVENT', 'END:VCALENDAR',
  ]
  const url = URL.createObjectURL(new Blob([lines.join('\r\n') + '\r\n'], { type: 'text/calendar;charset=utf-8' }))
  const link = document.createElement('a')
  link.href = url
  link.download = 'lingua-lesson.ics'
  link.click()
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}
