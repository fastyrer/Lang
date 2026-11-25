import { CalendarDays, ChevronLeft, ChevronRight, Clock3 } from 'lucide-react'
import { useState } from 'react'
import { ErrorMessage } from './Dialogs'
import { formats, languages, levels, slotLabel, timeLabel } from './lib'

const weekdays = ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс']
const dayParts = new Intl.DateTimeFormat('en-CA', {
  year: 'numeric', month: '2-digit', day: '2-digit', timeZone: 'Europe/Moscow',
})
const monthLabel = new Intl.DateTimeFormat('ru-RU', { month: 'long', year: 'numeric', timeZone: 'UTC' })
const dayLabel = new Intl.DateTimeFormat('ru-RU', { weekday: 'long', day: 'numeric', month: 'long', timeZone: 'UTC' })

export function dateKey(value) {
  const parts = Object.fromEntries(dayParts.formatToParts(new Date(value)).map(({ type, value }) => [type, value]))
  return [parts.year, parts.month, parts.day].join('-')
}

function dateOf(key) { return new Date(key + 'T12:00:00Z') }
function monthOf(key) { return key.slice(0, 7) }
function monthDate(month) { return dateOf(month + '-01') }

function moveMonth(month, step) {
  const date = monthDate(month)
  date.setUTCMonth(date.getUTCMonth() + step)
  return date.toISOString().slice(0, 7)
}

function monthGrid(month) {
  const first = monthDate(month)
  const offset = (first.getUTCDay() + 6) % 7
  first.setUTCDate(1 - offset)
  return Array.from({ length: 42 }, (_, index) => {
    const date = new Date(first)
    date.setUTCDate(date.getUTCDate() + index)
    return date.toISOString().slice(0, 10)
  })
}

function placeLabel(number) {
  if (number === 1) return '1 место'
  return number >= 2 && number <= 4 ? number + ' места' : number + ' мест'
}

function Month({ month, day, byDay, today, canNext, onMove, onDay }) {
  const cells = monthGrid(month)
  return (
    <div className="calendar-panel">
      <div className="calendar-heading">
        <button type="button" className="icon-button" aria-label="Предыдущий месяц" disabled={month <= monthOf(today)} onClick={() => onMove(-1)}><ChevronLeft size={20} /></button>
        <h3 aria-live="polite">{monthLabel.format(monthDate(month))}</h3>
        <button type="button" className="icon-button" aria-label="Следующий месяц" disabled={!canNext} onClick={() => onMove(1)}><ChevronRight size={20} /></button>
      </div>
      <table className="month" aria-label="Даты занятий">
        <thead><tr>{weekdays.map(weekday => <th key={weekday} scope="col">{weekday}</th>)}</tr></thead>
        <tbody>
          {Array.from({ length: 6 }, (_, week) => <tr key={cells[week * 7]}>
            {cells.slice(week * 7, week * 7 + 7).map(key => {
              const current = monthOf(key) === month
              const free = byDay[key]?.filter(slot => slot.available > 0).length || 0
              return <td key={key}>
                {current && <button type="button" className="calendar-day" aria-pressed={key === day} aria-current={key === today ? 'date' : undefined}
                  aria-label={dayLabel.format(dateOf(key)) + (free > 0 ? ', есть свободное время' : ', свободного времени нет')}
                  disabled={key < today || !free} onClick={() => onDay(key)}>
                  {Number(key.slice(-2))}
                </button>}
              </td>
            })}
          </tr>)}
        </tbody>
      </table>
    </div>
  )
}

export function BookingCalendar({ selection, catalog, catalogError, account, onChange, onContinue, onReload, busy, error, reschedule }) {
  const { language, format, slotId, moduleId } = selection
  const school = catalog?.school
  const allModules = school?.modules || []
  const modules = allModules.filter(item => item.language === language)
  const myBookings = account?.bookings || []
  const trialUsed = format === 'trial' && !reschedule && myBookings.some(item => !item.cancelled && item.slot.format === 'trial')
  const [viewMonth, setViewMonth] = useState('')
  const [dayChoice, setDayChoice] = useState('')

  const today = dateKey(Date.now())
  const slots = (catalog?.slots || []).filter(slot => slot.language === language && slot.format === format
    && (!moduleId || !slot.moduleId || slot.moduleId === moduleId)
    && !myBookings.some(item => !item.cancelled && item.slot.id === slot.id))
  const byDay = slots.reduce((grouped, slot) => {
    const key = dateKey(slot.startsAt)
    const daySlots = grouped[key] ||= []
    daySlots.push(slot)
    return grouped
  }, {})
  const chosen = slots.find(slot => slot.id === slotId && slot.available > 0)
  const firstFreeDay = slots.find(slot => slot.available > 0)
  const fallback = chosen ? dateKey(chosen.startsAt) : firstFreeDay ? dateKey(firstFreeDay.startsAt) : today
  const month = viewMonth || monthOf(fallback)
  const day = dayChoice || (month === monthOf(fallback) ? fallback : null)
  const lastMonth = slots.length ? monthOf(dateKey(slots.at(-1).startsAt)) : monthOf(today)
  const sub = account?.subscription
  const plan = catalog?.plans.find(item => item.format === format)
  const needsPlan = Boolean(account && format !== 'trial' && !sub?.active && !reschedule)
  const blocked = Boolean(account && format !== 'trial' && sub?.active && (sub.planId !== plan?.id || !reschedule && sub.remaining === 0))
  const end = chosen && Date.parse(chosen.startsAt) + chosen.duration * 60_000
  const expiresBeforeLesson = end && account && format !== 'trial' && sub?.active && end > Date.parse(sub.endsAt)

  function changeFilter(key, value) {
    setViewMonth('')
    setDayChoice('')
    onChange(key, value)
  }

  function selectDay(key) {
    setDayChoice(key)
    onChange('slotId', '')
  }

  function changeMonth(step) {
    const next = moveMonth(month, step)
    setViewMonth(next)
    const first = Object.keys(byDay).find(key => monthOf(key) === next && byDay[key].some(slot => slot.available > 0))
    selectDay(first || null)
  }

  return (
    <div className="dialog-form booking-calendar">
      <div className="booking-selects">
        <label>Язык<select value={language} onChange={event => changeFilter('language', event.target.value)} disabled={busy || !!reschedule}>
          {Object.entries(languages).map(([id, label]) => <option value={id} key={id}>{label}</option>)}
        </select></label>
        <label>Формат<select value={format} onChange={event => changeFilter('format', event.target.value)} disabled={busy || !!reschedule}>
          {Object.entries(formats).map(([id, label]) => <option value={id} key={id}>{label}</option>)}
        </select></label>
        <label>Модуль<select value={moduleId || ''} onChange={event => changeFilter('moduleId', event.target.value)} disabled={busy || !!reschedule}>
          <option value="">{format === 'group' ? 'Все модули' : 'Выбрать с преподавателем'}</option>
          {modules.map(item => <option value={item.id} key={item.id}>{item.title}</option>)}
        </select></label>
      </div>
      {trialUsed ? <p className="info">Вы уже записывались на пробный урок. Выберите индивидуальное занятие или мини-группу.</p> : <>
        {catalog ? <div className="calendar-layout">
          <Month month={month} day={day} byDay={byDay} today={today} canNext={month < lastMonth} onMove={changeMonth} onDay={selectDay} />
          <fieldset className="times calendar-times" disabled={busy}>
            <legend><Clock3 size={17} /> {day ? slotLabel(dateOf(day)) : 'Выберите дату'} <span>МСК</span></legend>
            <div className="calendar-slots">
              {day && byDay[day]?.map(slot => {
                const topic = allModules.find(item => item.id === (slot.moduleId || moduleId))
                return <label key={slot.id} className="calendar-slot">
                  <input type="radio" name="slot" value={slot.id} checked={slotId === slot.id} disabled={!slot.available} onChange={() => onChange('slotId', slot.id)} />
                  <span className="slot-content">
                    <strong>{timeLabel(slot.startsAt)} · {slot.duration} мин</strong>
                    {topic && <span>{topic.title} · {levels[topic.level]}</span>}
                  </span>
                  <span className="slot-capacity">{slot.available ? placeLabel(slot.available) : 'Занято'}</span>
                </label>
              })}
              {!(day && byDay[day]?.length) && <p className="muted">На эту дату свободного времени нет. Выберите другую дату или формат.</p>}
            </div>
          </fieldset>
        </div> : <p className="muted">{catalogError || 'Загружаем календарь…'}</p>}
        {catalogError && <button className="button button-outline" onClick={onReload}>Обновить календарь</button>}
        {needsPlan && <p className="info">Для этого формата нужен пакет занятий.</p>}
        {blocked && <p className="info">Проверьте формат и остаток уроков в пакете.</p>}
        {expiresBeforeLesson && <p className="info">Выберите занятие до окончания пакета.</p>}
        {catalog?.demoSchedule && <p className="muted demo-note">Демо-расписание. Ссылки на встречи появятся в реальном расписании преподавателя.</p>}
        {school?.cancelHours > 0 && <p className="muted demo-note">Перенос и отмена доступны не позднее чем за {school.cancelHours} ч до начала.</p>}
        <ErrorMessage>{error}</ErrorMessage>
        <div className="calendar-footer">
          <span className="muted"><CalendarDays size={17} /> {chosen ? slotLabel(chosen.startsAt) + ' · ' + timeLabel(chosen.startsAt) + ' МСК' : 'Выберите дату и время'}</span>
          <button className="button button-primary" disabled={!chosen || busy || blocked || Boolean(expiresBeforeLesson)} onClick={() => onContinue(needsPlan ? plan : null)}>
            {busy ? 'Сохраняем…' : !account ? 'Продолжить с профилем' : needsPlan ? 'К пакету занятий' : reschedule ? 'Подтвердить перенос' : 'Подтвердить запись'}
          </button>
        </div>
      </>}
    </div>
  )
}
